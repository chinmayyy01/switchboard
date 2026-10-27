import argparse

from rag import retrieve

CASES = [
    {"q": "what are your hours", "doc": "faq/hours", "answerable": True},
    {"q": "are you open on weekends", "doc": "faq/hours", "answerable": True},
    {"q": "do you cover the airport district", "doc": "policy/service-area", "answerable": True},
    {"q": "which areas do you serve", "doc": "policy/service-area", "answerable": True},
    {"q": "how much is a callout", "doc": "policy/pricing", "answerable": True},
    {"q": "what does drain cleaning cost", "doc": "policy/pricing", "answerable": True},
    {"q": "what is the warranty on repairs", "doc": "policy/warranty", "answerable": True},
    {"q": "what is not covered by the warranty", "doc": "policy/warranty", "answerable": True},
    {"q": "what is the cancellation fee", "doc": "policy/cancellation", "answerable": True},
    {"q": "can i cancel for free", "doc": "policy/cancellation", "answerable": True},
    {"q": "is a gas smell an emergency", "doc": "faq/emergencies", "answerable": True},
    {"q": "what counts as an emergency", "doc": "faq/emergencies", "answerable": True},
    {"q": "do you offer a senior discount", "doc": None, "answerable": False},
    {"q": "do you fix dishwashers", "doc": None, "answerable": False},
    {"q": "are you hiring", "doc": None, "answerable": False},
    {"q": "what is your license number", "doc": None, "answerable": False},
    {"q": "do you service commercial buildings", "doc": None, "answerable": False},
    {"q": "can i pay with bitcoin", "doc": None, "answerable": False},
]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tenant", default="rivera")
    ap.add_argument("--k", type=int, default=3)
    ap.add_argument("--gate", action="store_true")
    ap.add_argument("--min-recall", type=float, default=0.85)
    ap.add_argument("--min-abstention", type=float, default=0.80)
    ap.add_argument("--verbose", action="store_true")
    args = ap.parse_args()

    hits_k = hits_1 = 0
    rr = 0.0
    grounded_ok = 0
    pos = neg = neg_ok = 0

    for c in CASES:
        res = retrieve(args.tenant, c["q"], args.k)
        docs = [r["doc_id"] for r in res["results"]]
        if c["answerable"]:
            pos += 1
            if c["doc"] in docs:
                hits_k += 1
                rank = docs.index(c["doc"]) + 1
                rr += 1.0 / rank
                if rank == 1:
                    hits_1 += 1
            good = res["grounded"] and bool(docs) and docs[0] == c["doc"]
            if good:
                grounded_ok += 1
        else:
            neg += 1
            good = not res["grounded"]
            if good:
                neg_ok += 1
                grounded_ok += 1
        if args.verbose:
            print(
                f"  {'OK ' if good else 'BAD'} q={c['q']!r} "
                f"grounded={res['grounded']} top={docs[0] if docs else '-'} "
                f"want={c['doc']}"
            )

    total = len(CASES)
    recall_k = hits_k / pos if pos else 0.0
    recall_1 = hits_1 / pos if pos else 0.0
    mrr = rr / pos if pos else 0.0
    abstention = neg_ok / neg if neg else 0.0
    grounded_acc = grounded_ok / total if total else 0.0

    print(f"RAG eval (tenant={args.tenant}, k={args.k}, n={total}, answerable={pos}, negatives={neg})")
    print(f"  recall@1={recall_1:.3f}  recall@{args.k}={recall_k:.3f}  mrr={mrr:.3f}")
    print(f"  grounded_accuracy={grounded_acc:.3f}  abstention_accuracy={abstention:.3f}")

    if args.gate and (recall_k < args.min_recall or abstention < args.min_abstention):
        print("GATE FAILED")
        raise SystemExit(1)


if __name__ == "__main__":
    main()
