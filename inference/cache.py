import math
import threading
import time
from collections import OrderedDict

from prometheus_client import Counter, Histogram

BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5]

CACHE_HITS = Counter(
    "switchboard_ai_cache_hits_total", "Cache hits", ["kind"]
)
CACHE_MISSES = Counter(
    "switchboard_ai_cache_misses_total", "Cache misses", ["kind"]
)
RETRIEVE_SECONDS = Histogram(
    "switchboard_ai_retrieve_seconds", "Retrieval latency", buckets=BUCKETS
)
ANSWER_SECONDS = Histogram(
    "switchboard_ai_answer_seconds", "Grounded answer latency", buckets=BUCKETS
)
LLM_CALLS = Counter(
    "switchboard_ai_llm_calls_total", "LLM calls", ["model"]
)


class TTLCache:
    def __init__(self, ttl, maxsize=1024):
        self.ttl = ttl
        self.maxsize = maxsize
        self._mu = threading.Lock()
        self._data = OrderedDict()

    def get(self, key):
        with self._mu:
            item = self._data.get(key)
            if item is None:
                return None
            ts, value = item
            if time.time() - ts > self.ttl:
                del self._data[key]
                return None
            self._data.move_to_end(key)
            return value

    def set(self, key, value):
        with self._mu:
            self._data[key] = (time.time(), value)
            self._data.move_to_end(key)
            while len(self._data) > self.maxsize:
                self._data.popitem(last=False)


class EmbeddingCache:
    def __init__(self, maxsize=4096):
        self.maxsize = maxsize
        self._mu = threading.Lock()
        self._data = OrderedDict()

    def get(self, key):
        with self._mu:
            value = self._data.get(key)
            if value is None:
                return None
            self._data.move_to_end(key)
            return value

    def set(self, key, value):
        with self._mu:
            self._data[key] = value
            self._data.move_to_end(key)
            while len(self._data) > self.maxsize:
                self._data.popitem(last=False)


def _cosine(a, b):
    if not a or not b:
        return 0.0
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


class SemanticCache:
    def __init__(self, ttl=3600, threshold=0.93, max_entries=256):
        self.ttl = ttl
        self.threshold = threshold
        self.max_entries = max_entries
        self._mu = threading.Lock()
        self._data = {}

    def get(self, tenant, version, vec):
        now = time.time()
        best = None
        best_sim = 0.0
        with self._mu:
            entries = self._data.get(tenant, [])
            for entry in entries:
                if entry["version"] != version:
                    continue
                if now - entry["ts"] > self.ttl:
                    continue
                sim = _cosine(vec, entry["vec"])
                if sim > best_sim:
                    best_sim = sim
                    best = entry
            if best is not None and best_sim >= self.threshold:
                CACHE_HITS.labels("answer").inc()
                return {
                    "answer": best["answer"],
                    "citations": best["citations"],
                    "similarity": round(best_sim, 3),
                }
        CACHE_MISSES.labels("answer").inc()
        return None

    def set(self, tenant, version, vec, answer, citations):
        with self._mu:
            entries = self._data.setdefault(tenant, [])
            entries.append(
                {
                    "version": version,
                    "vec": vec,
                    "answer": answer,
                    "citations": citations,
                    "ts": time.time(),
                }
            )
            if len(entries) > self.max_entries:
                del entries[0 : len(entries) - self.max_entries]
