package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// 기본 설정들. render에서 설정한 값을 사용하고, 없으면 아래 값들을 사용. 로컬 디버깅용
	baseURL     = env("BASE_URL", "http://localhost:8080")
	dataDir     = env("DATA_DIR", "./uploads")
	secret      = env("TOKEN_SECRET", "dev-secret-change-me")
	maxUploadMB = int64Env("MAX_UPLOAD_MB", 20)
	allowExt    = map[string]bool{".txt": true, ".pdf": true, ".png": true, ".jpg": true, ".jpeg": true, ".zip": true}
)

// 서버 시작 & 라우팅
func main() {
	_ = os.MkdirAll(dataDir, 0o755)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("GET /list", listPage)
	mux.HandleFunc("POST /upload", uploadWeb)
	mux.HandleFunc("POST /api/upload", uploadAPI)
	mux.HandleFunc("GET /api/files", listFiles)
	mux.HandleFunc("GET /d/{id}/{name}", download)

	addr := ":" + env("PORT", "8080")
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logMW(mux)))
}

// 로컬 디버깅용
func home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "upload.html")
}

func listPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "list.html")
}

// 브라우저에서 폼으로 업로드 시 처리
func uploadWeb(w http.ResponseWriter, r *http.Request) {
	link, err := saveFromMultipart(w, r)
	if err != nil { http.Error(w, err.Error(), 400); return }
	http.Redirect(w, r, link, http.StatusSeeOther)
}

// API로 업로드 시 처리 -> {"url": "<다운로드 링크>"} 형식으로 응답.
func uploadAPI(w http.ResponseWriter, r *http.Request) {
	link, err := saveFromMultipart(w, r)
	if err != nil { http.Error(w, err.Error(), 400); return }
	_ = json.NewEncoder(w).Encode(map[string]string{"url": link})
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		http.Error(w, "failed to read directory", 500)
		return
	}

	type FileInfo struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Size       int64  `json:"size"`
		UploadedAt string `json:"uploaded_at"`
		URL        string `json:"url"`
		IsImage    bool   `json:"is_image"`
	}

	var files []FileInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		id := entry.Name()
		metaPath := filepath.Join(dataDir, id, "meta.json")
		b, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta struct {
			ID         string `json:"id"`
			OrigName   string `json:"orig_name"`
			Size       int64  `json:"size"`
			UploadedAt string `json:"uploaded_at"`
		}
		if err := json.Unmarshal(b, &meta); err != nil {
			continue
		}

		token := sign(secret, id)
		url := fmt.Sprintf("%s/d/%s/%s?token=%s", baseURL, id, safeName(meta.OrigName), token)
		
		ext := strings.ToLower(filepath.Ext(meta.OrigName))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"

		files = append(files, FileInfo{
			ID:         id,
			Name:       meta.OrigName,
			Size:       meta.Size,
			UploadedAt: meta.UploadedAt,
			URL:        url,
			IsImage:    isImage,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

// 실제 저장 로직
func saveFromMultipart(w http.ResponseWriter, r *http.Request) (string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadMB*1024*1024)
	if err := r.ParseMultipartForm(maxUploadMB * 1024 * 1024); err != nil {
		return "", fmt.Errorf("form too large")
	}
	f, hdr, err := r.FormFile("file")
	if err != nil { return "", fmt.Errorf("file required") }
	defer f.Close()

	name := hdr.Filename
	ext := strings.ToLower(filepath.Ext(name))
	if !allowExt[ext] { return "", fmt.Errorf("extension not allowed") }

	head, body := peek512(f)
	if err := validateContent(head, ext); err != nil { return "", err }

	id := newID()
	dir := filepath.Join(dataDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil { return "", err }

	blob := filepath.Join(dir, "blob")
	out, err := os.Create(blob)
	if err != nil { return "", err }
	n, err := io.Copy(out, body)
	_ = out.Close()
	if err != nil { return "", err }

	meta := map[string]any{
		"id": id, "orig_name": name, "stored_path": "blob", "size": n, "uploaded_at": time.Now().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644); err != nil { return "", err }

	token := sign(secret, id)
	return fmt.Sprintf("%s/d/%s/%s?token=%s", baseURL, id, safeName(name), token), nil
}

func download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !hmac.Equal([]byte(sign(secret, id)), []byte(r.URL.Query().Get("token"))) {   // 토큰 hmac 검증. sercret을 알아야 함
		http.Error(w, "forbidden", http.StatusForbidden); return
	}
	metaPath := filepath.Join(dataDir, id, "meta.json")    // mets.json읽어서 파일명, 경로 가져오기
	b, err := os.ReadFile(metaPath)
	if err != nil {
		log.Printf("Failed to read meta.json: %v", err)
		http.NotFound(w, r)
		return
	}

	var meta struct{
		OrigName   string `json:"orig_name"`
		StoredPath string `json:"stored_path"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		log.Printf("Failed to unmarshal meta.json: %v, content: %s", err, string(b))
		http.Error(w, "internal error", 500)
		return
	}

	log.Printf("Download: id=%s, name=%s, path=%s", id, meta.OrigName, meta.StoredPath)

	f, err := os.Open(filepath.Join(dataDir, id, meta.StoredPath))
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(meta.OrigName)))
	if ct == "" { ct = "application/octet-stream" }
	w.Header().Set("Content-Type", ct)
	
	// 이미지는 브라우저에서 바로 보이도록 inline, 나머지는 다운로드
	ext := strings.ToLower(filepath.Ext(meta.OrigName))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, safeName(meta.OrigName)))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName(meta.OrigName)))
	}
	
	http.ServeContent(w, r, meta.OrigName, time.Now(), f)
}

func validateContent(head []byte, ext string) error {
	ct := http.DetectContentType(head)
	ok := false
	switch {
	case strings.HasPrefix(ct, "image/jpeg") && (ext == ".jpg" || ext == ".jpeg"):
		ok = true
	case strings.HasPrefix(ct, "image/png") && ext == ".png":
		ok = true
	case strings.HasPrefix(ct, "application/pdf") && ext == ".pdf":
		ok = true
	case strings.HasPrefix(ct, "application/zip") && ext == ".zip":
		ok = true
	case strings.HasPrefix(ct, "text/plain") && ext == ".txt":
		ok = true
	}
	if !ok { return fmt.Errorf("content-type mismatch: %s", ct) }
	return nil
}

func newID() string {
	b := make([]byte, 16); _, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
func sign(secret, id string) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func safeName(n string) string {
	n = filepath.Base(n)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	return re.ReplaceAllString(n, "_")
}
func peek512(r io.Reader) ([]byte, io.Reader) {
	buf := make([]byte, 512)
	n, _ := io.ReadFull(r, buf)
	head := buf[:n]
	return head, io.MultiReader(bytes.NewReader(head), r)
}
func logMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now(); next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(t))
	})
}
func env(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }
func int64Env(k string, d int64) int64 {
	if v := os.Getenv(k); v != "" {
		var n int64; fmt.Sscan(v, &n); if n > 0 { return n }
	}
	return d
}
