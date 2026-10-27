import json
import os
import re
import urllib.request

from rag import OLLAMA_URL, retrieve

CHAT_MODEL = os.getenv("CHAT_MODEL") or os.getenv("OLLAMA_MODEL") or "qwen3:1.7b"

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


def _chat(messages):
    body = json.dumps(
        {"model": CHAT_MODEL, "stream": False, "messages": messages}
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
    res = retrieve(tenant, query, k)
    if not res["grounded"] or not res["results"]:
        return {"grounded": False, "answer": "", "citations": [], "top_score": res["top_score"]}

    context = "\n---\n".join(r["text"] for r in res["results"])
    reply = _chat(
        [
            {"role": "system", "content": INSTRUCTION},
            {"role": "user", "content": "Context:\n" + context + "\n\nCaller: " + query},
        ]
    )

    text = (reply or "").strip()
    low = text.strip(" .!\"'").lower()
    if not text or low == "none" or low.startswith("none ") or _looks_imperative(text):
        text = _best_sentence(query, context)
    if not text or _looks_imperative(text):
        return {"grounded": False, "answer": "", "citations": [], "top_score": res["top_score"]}

    if ("$" in text or "dollar" in text.lower()) and "$" not in context and "dollar" not in context.lower():
        return {"grounded": False, "answer": "", "citations": [], "top_score": res["top_score"]}

    citations = [f"{r['title']} ({r['doc_id']})" for r in res["results"]]
    return {
        "grounded": True,
        "answer": text,
        "citations": citations,
        "top_score": res["top_score"],
    }
