package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/tenant"
)

const (
	maxDocBytes    = 2 << 20
	maxDocsTotal   = 200
	maxIngestBytes = 16 << 20
	ingestTimeout  = 2 * time.Minute
)

type DocsAPI struct {
	Tenants *tenant.Registry
	Dir     string
	AIURL   string
}

type docFile struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Bytes     int64  `json:"bytes"`
	UpdatedAt int64  `json:"updated_at"`
}

type ingestDoc struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type ingestResult struct {
	Chunks   int `json:"chunks"`
	Embedded int `json:"embedded"`
}

func (a *DocsAPI) root(r *http.Request, account auth.Account) (string, string, bool) {
	id := requestTenantID(r, account)
	if _, ok := a.Tenants.Get(id); !ok {
		return "", "", false
	}
	dir := a.Dir
	if dir == "" {
		dir = "tenants"
	}
	return id, filepath.Join(dir, id, "docs"), true
}

func canEditTenant(account auth.Account) bool {
	return account.IsOwner || account.Role == auth.RoleOwner
}

func (a *DocsAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, root, ok := a.root(r, account)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		files, err := listDocs(root)
		if err != nil {
			http.Error(w, "could not read documents", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"documents": files})
	}
}

func (a *DocsAPI) Upload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !canEditTenant(account) {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		id, root, ok := a.root(r, account)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		if err := r.ParseMultipartForm(6 << 20); err != nil {
			http.Error(w, "file is too large", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "choose a file", http.StatusBadRequest)
			return
		}
		defer file.Close()
		name, ok := safeDocName(header.Filename)
		if !ok {
			http.Error(w, "only .md and .txt files are supported", http.StatusBadRequest)
			return
		}
		docType := strings.ToLower(strings.TrimSpace(r.FormValue("type")))
		if docType != "policy" {
			docType = "faq"
		}
		data, err := io.ReadAll(io.LimitReader(file, maxDocBytes+1))
		if err != nil {
			http.Error(w, "could not read the file", http.StatusBadRequest)
			return
		}
		if len(data) > maxDocBytes {
			http.Error(w, "each file must be 2 MB or smaller", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(string(data)) == "" {
			http.Error(w, "the file is empty", http.StatusBadRequest)
			return
		}
		target := filepath.Join(root, docType, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			http.Error(w, "could not save the file", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			http.Error(w, "could not save the file", http.StatusInternalServerError)
			return
		}
		a.respondWithIndex(w, r.Context(), id, root)
	}
}

func (a *DocsAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !canEditTenant(account) {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		id, root, ok := a.root(r, account)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		target, ok := docPath(root, name)
		if !ok {
			http.Error(w, "unknown document", http.StatusBadRequest)
			return
		}
		if err := os.Remove(target); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "unknown document", http.StatusNotFound)
				return
			}
			http.Error(w, "could not delete the document", http.StatusInternalServerError)
			return
		}
		a.respondWithIndex(w, r.Context(), id, root)
	}
}

func (a *DocsAPI) Reindex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		account, ok := accountFrom(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !canEditTenant(account) {
			http.Error(w, "owner role required", http.StatusForbidden)
			return
		}
		id, root, ok := a.root(r, account)
		if !ok {
			http.Error(w, "unknown tenant", http.StatusNotFound)
			return
		}
		files, err := listDocs(root)
		if err != nil {
			http.Error(w, "could not read documents", http.StatusInternalServerError)
			return
		}
		result, err := a.index(r.Context(), id, root)
		response := map[string]any{"documents": files}
		if err != nil {
			response["index_error"] = err.Error()
		} else {
			response["index"] = result
		}
		writeJSON(w, response)
	}
}

// respondWithIndex returns the saved document list even when the AI service is
// unavailable, so an upload is never silently lost.
func (a *DocsAPI) respondWithIndex(w http.ResponseWriter, ctx context.Context, id, root string) {
	files, err := listDocs(root)
	if err != nil {
		http.Error(w, "could not read documents", http.StatusInternalServerError)
		return
	}
	response := map[string]any{"documents": files}
	result, err := a.index(ctx, id, root)
	if err != nil {
		response["index_error"] = err.Error()
	} else {
		response["index"] = result
	}
	writeJSON(w, response)
}

func (a *DocsAPI) index(ctx context.Context, id, root string) (ingestResult, error) {
	if strings.TrimSpace(a.AIURL) == "" {
		return ingestResult{}, errors.New("indexing service is not configured")
	}
	docs, err := readDocs(root)
	if err != nil {
		return ingestResult{}, err
	}
	body, err := json.Marshal(map[string]any{"tenant": id, "docs": docs})
	if err != nil {
		return ingestResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, ingestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.AIURL, "/")+"/ingest", bytes.NewReader(body))
	if err != nil {
		return ingestResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ingestResult{}, errors.New("indexing service is unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ingestResult{}, fmt.Errorf("indexing failed: %s", strings.TrimSpace(string(message)))
	}
	var result ingestResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ingestResult{}, errors.New("indexing returned an invalid response")
	}
	return result, nil
}

func listDocs(root string) ([]docFile, error) {
	files := []docFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !isDocFile(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		name := filepath.ToSlash(rel)
		files = append(files, docFile{
			Name:      name,
			Type:      docType(name),
			Bytes:     info.Size(),
			UpdatedAt: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func readDocs(root string) ([]ingestDoc, error) {
	var docs []ingestDoc
	total := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() || !isDocFile(path) || len(docs) >= maxDocsTotal {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > maxDocBytes || total+int(info.Size()) > maxIngestBytes {
			return nil
		}
		total += int(info.Size())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		docs = append(docs, ingestDoc{Name: filepath.ToSlash(rel), Text: string(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

func docType(name string) string {
	if i := strings.Index(name, "/"); i > 0 {
		return name[:i]
	}
	return "faq"
}

func isDocFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return false
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".md", ".txt":
		return true
	default:
		return false
	}
}

func safeDocName(raw string) (string, bool) {
	base := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	base = strings.TrimSpace(base)
	ext := filepath.Ext(base)
	switch strings.ToLower(ext) {
	case ".md", ".txt":
	default:
		return "", false
	}
	stem := strings.TrimSuffix(base, ext)
	var b strings.Builder
	for _, r := range stem {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ', r == '.':
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-_")
	if name == "" {
		return "", false
	}
	if len(name) > 80 {
		name = name[:80]
	}
	return name + ext, true
}

// docPath resolves a client-supplied relative name inside the docs root.
func docPath(root, name string) (string, bool) {
	clean := filepath.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || strings.HasPrefix(clean, ".") {
		return "", false
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, target)
	if err != nil || strings.HasPrefix(rel, "..") || !isDocFile(target) {
		return "", false
	}
	return target, true
}
