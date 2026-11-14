"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Badge, Card } from "@/components/ui";

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

type DocFile = {
  name: string;
  type: string;
  bytes: number;
  updated_at: number;
};

type DocsResponse = {
  documents?: DocFile[];
  index?: { chunks: number; embedded: number };
  index_error?: string;
  error?: string;
};

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function docTitle(name: string) {
  const file = name.split("/").pop() ?? name;
  return file.replace(/\.(md|txt)$/i, "");
}

export function DocumentsCard({ tenant }: { tenant: string }) {
  const [docs, setDocs] = useState<DocFile[]>([]);
  const [docType, setDocType] = useState("policy");
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  const apply = useCallback((data: DocsResponse) => {
    if (Array.isArray(data.documents)) setDocs(data.documents);
    if (data.index) {
      setStatus(
        data.index.chunks === 0
          ? "Index cleared. The receptionist will say it does not know until documents are added."
          : `Indexed ${data.index.chunks} passage${data.index.chunks === 1 ? "" : "s"}. New calls use the update.`,
      );
    } else if (data.index_error) {
      setStatus("");
      setError(`Saved, but indexing failed: ${data.index_error}. Try Rebuild index.`);
    } else {
      setStatus("");
    }
  }, []);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    setStatus("");
    fetch(`${API}/api/tenant/docs?tenant=${encodeURIComponent(tenant)}`, {
      credentials: "include",
      cache: "no-store",
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("Could not load documents.");
        const data = (await res.json()) as DocsResponse;
        if (active) setDocs(data.documents ?? []);
      })
      .catch((err: Error) => {
        if (active) setError(err.message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [tenant]);

  const upload = async () => {
    const file = input.current?.files?.[0];
    if (!file) {
      setError("Choose a file first.");
      return;
    }
    setBusy(true);
    setError("");
    setStatus("");
    try {
      const form = new FormData();
      form.append("file", file);
      form.append("type", docType);
      const res = await fetch(`${API}/api/tenant/docs?tenant=${encodeURIComponent(tenant)}`, {
        method: "POST",
        credentials: "include",
        body: form,
      });
      if (!res.ok) throw new Error((await res.text()) || "Could not upload the file.");
      apply((await res.json()) as DocsResponse);
      if (input.current) input.current.value = "";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not upload the file.");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (name: string) => {
    setBusy(true);
    setError("");
    setStatus("");
    try {
      const res = await fetch(
        `${API}/api/tenant/docs?tenant=${encodeURIComponent(tenant)}&name=${encodeURIComponent(name)}`,
        { method: "DELETE", credentials: "include" },
      );
      if (!res.ok) throw new Error((await res.text()) || "Could not delete the document.");
      apply((await res.json()) as DocsResponse);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not delete the document.");
    } finally {
      setBusy(false);
    }
  };

  const reindex = async () => {
    setBusy(true);
    setError("");
    setStatus("");
    try {
      const res = await fetch(
        `${API}/api/tenant/docs/reindex?tenant=${encodeURIComponent(tenant)}`,
        { method: "POST", credentials: "include" },
      );
      if (!res.ok) throw new Error((await res.text()) || "Could not rebuild the index.");
      apply((await res.json()) as DocsResponse);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not rebuild the index.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card className="p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-[#EAE5DC]">Business documents</h2>
          <p className="mt-1 max-w-xl text-xs leading-relaxed text-[#8A8379]">
            Upload policy and FAQ files as Markdown or plain text. The receptionist
            answers from these and says when it does not know.
          </p>
        </div>
        <button
          type="button"
          onClick={reindex}
          disabled={busy}
          className="rounded-lg border border-[#3A3A3A] px-3 py-1.5 text-xs font-medium text-[#D8D2C7] hover:bg-[#2A2A2A] disabled:opacity-60"
        >
          Rebuild index
        </button>
      </div>

      {error && (
        <Alert tone="error" className="mt-4">
          {error}
        </Alert>
      )}
      {status && (
        <Alert tone="success" className="mt-4">
          {status}
        </Alert>
      )}

      <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:items-center">
        <label className="flex items-center gap-2 text-xs text-[#A8A092]">
          <span className="sr-only">Document type</span>
          <select
            value={docType}
            onChange={(event) => setDocType(event.target.value)}
            className="min-h-10 rounded-lg border border-[#3A3A3A] bg-[#242424] px-3 py-2 text-sm text-[#D8D2C7]"
          >
            <option value="policy">Policy</option>
            <option value="faq">FAQ</option>
          </select>
        </label>
        <input
          ref={input}
          type="file"
          aria-label="Document file"
          accept=".md,.txt,text/markdown,text/plain"
          className="min-w-0 flex-1 text-sm text-[#A8A092] file:mr-3 file:rounded-lg file:border-0 file:bg-[#2E2E2E] file:px-3 file:py-2 file:text-sm file:font-medium file:text-[#EAE5DC] hover:file:bg-[#3A3A3A]"
        />
        <button
          type="button"
          onClick={upload}
          disabled={busy}
          className="rounded-lg bg-[#C87941] px-4 py-2 text-sm font-semibold text-[#111111] hover:bg-[#D98E5A] disabled:opacity-60"
        >
          {busy ? "Working…" : "Upload"}
        </button>
      </div>

      <div className="mt-4">
        {loading ? (
          <p className="text-sm text-[#A8A092]">Loading documents…</p>
        ) : docs.length === 0 ? (
          <p className="text-sm text-[#8A8379]">No documents yet.</p>
        ) : (
          <ul className="divide-y divide-[#2E2E2E] rounded-xl border border-[#3A3A3A]">
            {docs.map((doc) => (
              <li key={doc.name} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2.5">
                <div className="min-w-0">
                  <div className="truncate text-sm text-[#EAE5DC]">{docTitle(doc.name)}</div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-[#8A8379]">
                    <Badge tone="slate">{doc.type}</Badge>
                    <span>{formatBytes(doc.bytes)}</span>
                    <span className="truncate">{doc.name}</span>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => remove(doc.name)}
                  disabled={busy}
                  className="rounded-lg px-2 py-1 text-xs font-medium text-[#E9C2BC] hover:bg-[#2A2A2A] disabled:opacity-60"
                >
                  Remove
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}
