import json
import math
import os
import re
import urllib.request
from functools import lru_cache
from pathlib import Path

import numpy as np

from cache import CACHE_HITS, CACHE_MISSES, RETRIEVE_SECONDS, EmbeddingCache, TTLCache

KNOWLEDGE_DIR = os.getenv("KNOWLEDGE_DIR", "data/knowledge")
OLLAMA_URL = os.getenv("OLLAMA_URL", "http://localhost:11434")
EMBED_MODEL = os.getenv("EMBED_MODEL", "nomic-embed-text")
VEC_MIN = float(os.getenv("RAG_MIN_VECTOR", "0.55"))
SCORE_MIN = float(os.getenv("RAG_MIN_SCORE", "0.45"))

RETRIEVAL_TTL = float(os.getenv("RETRIEVAL_TTL_SECONDS", "300"))

_TOKEN = re.compile(r"[a-z0-9]+")

_embed_cache = EmbeddingCache(maxsize=4096)
_retrieval_cache = TTLCache(ttl=RETRIEVAL_TTL, maxsize=1024)


def tokenize(text):
    return [t for t in _TOKEN.findall(text.lower()) if len(t) > 1]


def chunk_doc(text, max_chars=600):
    paras = [p.strip() for p in text.split("\n\n") if p.strip()]
    chunks, buf = [], ""
    for p in paras:
        if buf and len(buf) + len(p) + 2 > max_chars:
            chunks.append(buf)
            buf = ""
        if len(p) > max_chars:
            for s in re.split(r"(?<=[.?!])\s+", p):
                if buf and len(buf) + len(s) + 1 > max_chars:
                    chunks.append(buf)
                    buf = ""
                buf = (buf + " " + s).strip()
            continue
        buf = (buf + "\n\n" + p).strip() if buf else p
    if buf:
        chunks.append(buf)
    return chunks


def embed_text(text):
    key = (EMBED_MODEL, text)
    cached = _embed_cache.get(key)
    if cached is not None:
        CACHE_HITS.labels("embedding").inc()
        return cached

    CACHE_MISSES.labels("embedding").inc()
    body = json.dumps({"model": EMBED_MODEL, "input": text}).encode()
    req = urllib.request.Request(
        OLLAMA_URL + "/api/embed",
        data=body,
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read())
    except Exception:
        return None
    embs = data.get("embeddings")
    vec = embs[0] if embs else None
    if vec is not None:
        _embed_cache.set(key, vec)
    return vec


def _cosine(a, b):
    if not a or not b:
        return 0.0
    va = np.asarray(a, dtype=np.float32)
    vb = np.asarray(b, dtype=np.float32)
    na = float(np.linalg.norm(va))
    nb = float(np.linalg.norm(vb))
    if na == 0 or nb == 0:
        return 0.0
    return float(np.dot(va, vb)) / (na * nb)


class BM25:
    def __init__(self, docs, k1=1.5, b=0.75):
        self.docs = docs
        self.k1 = k1
        self.b = b
        self.n = len(docs)
        self.avgdl = sum(len(d) for d in docs) / self.n if self.n else 0.0
        df = {}
        for d in docs:
            for t in set(d):
                df[t] = df.get(t, 0) + 1
        self.df = df
        self.idf = {
            t: math.log(1 + (self.n - c + 0.5) / (c + 0.5)) for t, c in df.items()
        }

    def idf_for(self, t):
        df = self.df.get(t, 0)
        return math.log(1 + (self.n - df + 0.5) / (df + 0.5))

    def score(self, query, i):
        d = self.docs[i]
        dl = len(d)
        if dl == 0 or not self.avgdl:
            return 0.0
        tf = {}
        for t in d:
            tf[t] = tf.get(t, 0) + 1
        s = 0.0
        for t in query:
            if t not in self.idf:
                continue
            f = tf.get(t, 0)
            s += (
                self.idf[t]
                * (f * (self.k1 + 1))
                / (f + self.k1 * (1 - self.b + self.b * dl / self.avgdl))
            )
        return s


class Index:
    def __init__(self, chunks):
        self.chunks = chunks
        self.bm25 = BM25([tokenize(c["text"]) for c in chunks])


@lru_cache(maxsize=64)
def load_index(tenant, version):
    path = Path(KNOWLEDGE_DIR) / f"{tenant}.json"
    if not path.exists():
        return None
    data = json.loads(path.read_text())
    return Index(data.get("chunks", []))


def doc_version(tenant):
    path = Path(KNOWLEDGE_DIR) / f"{tenant}.json"
    try:
        st = path.stat()
    except FileNotFoundError:
        return "missing"
    return f"{st.st_mtime_ns}-{st.st_size}"


def retrieve(tenant, query, k=3, types=None):
    version = doc_version(tenant)
    key = (tenant, query.strip().lower(), k, tuple(types or ()), version)
    cached = _retrieval_cache.get(key)
    if cached is not None:
        CACHE_HITS.labels("retrieval").inc()
        return cached

    CACHE_MISSES.labels("retrieval").inc()
    with RETRIEVE_SECONDS.time():
        result = _retrieve(tenant, query, k, types, version)
    _retrieval_cache.set(key, result)
    return result


def _retrieve(tenant, query, k, types, version):
    idx = load_index(tenant, version)
    if idx is None or not idx.chunks:
        return {"grounded": False, "top_score": 0.0, "results": []}

    q = tokenize(query)
    qvec = embed_text(query)
    raw = [idx.bm25.score(q, i) for i in range(len(idx.chunks))]
    denom = sum(idx.bm25.idf_for(t) for t in q) if q else 0.0

    scored = []
    for i, c in enumerate(idx.chunks):
        if types and c.get("type") not in types:
            continue
        keyword = min(1.0, raw[i] / denom) if denom > 0 else 0.0
        if qvec and c.get("vec"):
            vector = _cosine(qvec, c["vec"])
            score = 0.6 * vector + 0.4 * keyword
        else:
            vector = 0.0
            score = keyword
        scored.append(
            {
                "doc_id": c["doc_id"],
                "title": c["title"],
                "type": c["type"],
                "text": c["text"],
                "score": round(score, 4),
                "vector": round(vector, 4),
                "keyword": round(keyword, 4),
            }
        )

    scored.sort(key=lambda x: x["score"], reverse=True)
    top = scored[:k]
    top_score = top[0]["score"] if top else 0.0
    if top and qvec and top[0]["vector"] > 0:
        grounded = top[0]["vector"] >= VEC_MIN and top_score >= SCORE_MIN
    else:
        grounded = top_score >= 0.5
    return {
        "grounded": grounded,
        "top_score": top_score,
        "results": top,
    }
