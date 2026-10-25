import argparse
import json
from pathlib import Path

from rag import embed_text


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tenant", required=True)
    ap.add_argument("--docs", required=True)
    ap.add_argument("--out", default=None)
    ap.add_argument("--no-embed", action="store_true")
    args = ap.parse_args()

    from rag import chunk_doc

    base = Path(args.docs)
    chunks = []
    for path in sorted(base.rglob("*.md")):
        rel = path.relative_to(base)
        doc_type = rel.parts[0] if len(rel.parts) > 1 else "faq"
        text = path.read_text()
        for part in chunk_doc(text):
            vec = None if args.no_embed else embed_text(part)
            chunks.append(
                {
                    "doc_id": str(rel.with_suffix("")),
                    "tenant": args.tenant,
                    "type": doc_type,
                    "title": path.stem,
                    "text": part,
                    "vec": vec,
                }
            )

    out = Path(args.out or f"data/knowledge/{args.tenant}.json")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps({"tenant": args.tenant, "chunks": chunks}, indent=2))
    embedded = sum(1 for c in chunks if c["vec"])
    print(f"indexed {len(chunks)} chunks ({embedded} embedded) -> {out}")


if __name__ == "__main__":
    main()
