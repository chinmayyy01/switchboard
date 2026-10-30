import json
import os
import re
import urllib.request

from cache import ANSWER_SECONDS, LLM_CALLS, SemanticCache
from rag import OLLAMA_URL, doc_version, embed_text, retrieve

CHAT_MODEL = os.getenv("CHAT_MODEL") or os.getenv("OLLAMA_MODEL") or "qwen3:1.7b"
CHAT_MODEL_LARGE = os.getenv("CHAT_MODEL_LARGE", "")
SEMANTIC_TTL = float(os.getenv("SEMANTIC_CACHE_TTL_SECONDS", "3600"))
SEMANTIC_THRESHOLD = float(os.getenv("SEMANTIC_CACHE_THRESHOLD", "0.93"))

_cache = SemanticCache(ttl=SEMANTIC_TTL, threshold=SEMANTIC_THRESHOLD)

INSTRUCTION = (
    "Quote the single sentence from the context that answers the caller's "
    "question. Reply with that sentence only. If the context does not answer it, "
    "reply exactly: NONE."
)

IMPERATIVE_PREFIXES = [
    "never ", "do not ", "don't ", "always ", "tell ", "offer ",
    "waive ", "transfer ", "escalate", "if a caller", "for an emergency",
]

STOPWORDS = {
    "do", "you", "your", "the", "a", "an", "is", "are", "what", "how", "much",
    "we", "i", "to", "of", "for", "and", "on", "in", "at", "it", "this", "that",
    "can", "does", "with", "my", "there", "any",
}


def _tokens(text):
    return {
        t
        for t in re.findall(r"[a-z0-9]+", text.lower())
        if len(t) > 1 and t not in STOPWORDS
    }


def _split_sentences(text):
    return [s.strip() for s in re.split(r"(?<=[.?!])\s+", text) if s.strip()]


def _best_sentence(question, context):
    q = _tokens(question)
    if not q:
        return ""
    best = ""
    best_score = 0.0
    for sentence in _split_sentences(context):
        score = len(q & _tokens(sentence)) / len(q)
        if score > best_score:
            best_score = score
            best = sentence
    return best if best_score >= 0.3 else ""


def _looks_imperative(text):
    low = text.strip().lower()
    return any(low.startswith(p) for p in IMPERATIVE_PREFIXES)


def _choose_model(query, top_score):
    if CHAT_MODEL_LARGE and (len(query.split()) >= 8 or top_score < 0.6):
        return CHAT_MODEL_LARGE
    return CHAT_MODEL


def _chat(messages, model):
    LLM_CALLS.labels(model).inc()
    body = json.dumps(
        {"model": model, "stream": False, "messages": messages}
    ).encode()
    req = urllib.request.Request(
        OLLAMA_URL + "/api/chat",
        data=body,
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = json.loads(resp.read())
    except Exception:
        return ""
    return data.get("message", {}).get("content", "")


def answer(tenant, query, k=3):
    with ANSWER_SECONDS.time():
        res = retrieve(tenant, query, k)
        if not res["grounded"] or not res["results"]:
            return {
                "grounded": False,
                "answer": "",
                "citations": [],
                "top_score": res["top_score"],
                "cached": False,
            }

        version = doc_version(tenant)
        qvec = embed_text(query)
        if qvec is not None:
            hit = _cache.get(tenant, version, qvec)
            if hit is not None:
                return {
                    "grounded": True,
                    "answer": hit["answer"],
                    "citations": hit["citations"],
                    "top_score": res["top_score"],
                    "cached": True,
                }

        context = "\n---\n".join(r["text"] for r in res["results"])
        messages = [
            {"role": "system", "content": INSTRUCTION},
            {"role": "user", "content": "Context:\n" + context + "\n\nCaller: " + query},
        ]
        model = _choose_model(query, res["top_score"])
        reply = _chat(messages, model)
        if not reply and model != CHAT_MODEL:
            model = CHAT_MODEL
            reply = _chat(messages, model)

        text = (reply or "").strip()
        low = text.strip(" .!\"'").lower()
        if not text or low == "none" or low.startswith("none ") or _looks_imperative(text):
            text = _best_sentence(query, context)
        if not text or _looks_imperative(text):
            return {
                "grounded": False,
                "answer": "",
                "citations": [],
                "top_score": res["top_score"],
                "cached": False,
            }

        if ("$" in text or "dollar" in text.lower()) and "$" not in context and "dollar" not in context.lower():
            return {
                "grounded": False,
                "answer": "",
                "citations": [],
                "top_score": res["top_score"],
                "cached": False,
            }

        citations = [f"{r['title']} ({r['doc_id']})" for r in res["results"]]
        if qvec is not None:
            _cache.set(tenant, version, qvec, text, citations)

        return {
            "grounded": True,
            "answer": text,
            "citations": citations,
            "top_score": res["top_score"],
            "cached": False,
            "model": model,
        }
