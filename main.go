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
	baseURL     = env("BASE_URL", "http://localhost:8080")
	dataDir     = env("DATA_DIR", "./uploads")
	maxUploadMB = int64Env("MAX_UPLOAD_MB", 20)
	secret      = env("TOKEN_SECRET", "dev-secret-change-me")
	allowExt    = map[string]bool{".txt": true, ".pdf": true, ".png": true, ".jpg": true, ".jpeg": true, ".zip": true}
)

func main() {
	_ = os.MkdirAll(dataDir, 0o755)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("POST /upload", uploadWeb)          // 웹 폼
	mux.HandleFunc("POST /api/upload", uploadAPI)      // curl
	mux.HandleFunc("GET /d/{id}/{name}", download)     // 공유 링크

	addr := ":" + env("PORT", "8080")
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logMW(mux)))
}

/* ------------ 메인 페이지 스타일 -----------------*/
func home(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, `<!doctype html><meta charset="utf-8">
<title>file shared platform</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
  :root {
    --accent: #b01212;
    --accent-weak: #e74c3c22;
    --text: #222;
    --muted: #666;
    --bg: #fafafa;
    --card: #fff;
    --radius: 14px;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 40px 16px; background: var(--bg);
    color: var(--text); font-family: ui-sans-serif, -apple-system, system-ui, "Segoe UI", Roboto, "Noto Sans KR", Apple SD Gothic Neo, "Malgun Gothic", sans-serif;
  }
  .wrap { max-width: 960px; margin: 0 auto; }
  h1 {
    margin: 0 0 8px; text-align: center; font-size: 36px; letter-spacing: .3px;
  }
  .desc {
    text-align: center; color: var(--muted); line-height: 1.6; margin-bottom: 24px;
  }
  .card {
    background: var(--card); border-radius: var(--radius);
    box-shadow: 0 8px 24px rgba(0,0,0,.05);
    padding: 24px; border: 1px solid #eee;
  }
  .dropzone {
    border: 3px dashed var(--accent);
    border-radius: 18px;
    height: 320px;
    display: grid; place-items: center;
    margin: 12px 0 8px;
    background: #fff;
    transition: background .15s ease, transform .1s ease;
  }
  .dropzone.dragover { background: var(--accent-weak); transform: scale(1.002); }
  .btn {
    appearance: none; border: 0; cursor: pointer;
    background: var(--accent); color: #fff; font-weight: 700;
    padding: 12px 22px; border-radius: 10px; font-size: 16px;
  }
  .hint { text-align: center; color: var(--muted); margin-top: 10px; }
  .hr { height: 1px; background: #eee; margin: 22px 0; }
  code, pre { background: #f6f6f6; padding: 8px 10px; border-radius: 8px; }
  pre { overflow: auto; }
</style>

<div class="wrap">
  <h1>Free File Upload</h1>
  <p class="desc">
    자유롭게 파일을 공유하는 플랫폼입니다.<br>
    악의적인 파일 업로드는 금지하고 있습니다.
  </p>

  <div class="card">
    <form id="uploadForm" action="/upload" method="post" enctype="multipart/form-data">
      <input id="fileInput" type="file" name="file" style="display:none"
             accept=".txt,.pdf,.png,.jpg,.jpeg,.zip">
      <div id="dropzone" class="dropzone">
        <button type="button" class="btn" id="pickBtn">파일 선택</button>
      </div>
      <p class="hint">여기에 파일을 드래그 앤 드롭하거나 클릭하여 업로드하세요</p>
    </form>

    <div class="hr"></div>

    <p><strong>curl로 업로드</strong></p>
    <pre>curl -F file=@README.md ` + baseURL + `/api/upload</pre>
  </div>
</div>

<script>
  const form = document.getElementById('uploadForm');
  const fileInput = document.getElementById('fileInput');
  const pickBtn = document.getElementById('pickBtn');
  const dz = document.getElementById('dropzone');

  // 버튼 클릭 → 파일 선택
  pickBtn.addEventListener('click', () => fileInput.click());
  fileInput.addEventListener('change', () => {
    if (fileInput.files.length) form.submit();
  });

  // 드래그 앤 드롭 업로드
  ;['dragenter','dragover'].forEach(e =>
    dz.addEventListener(e, evt => { evt.preventDefault(); evt.stopPropagation(); dz.classList.add('dragover'); })
  );
  ;['dragleave','drop'].forEach(e =>
    dz.addEventListener(e, evt => { evt.preventDefault(); evt.stopPropagation(); dz.classList.remove('dragover'); })
  );
  dz.addEventListener('drop', async (evt) => {
    const files = evt.dataTransfer.files;
    if (!files || !files.length) return;

    // 폼 전송 (기본: /upload 로 리다이렉트)
    const data = new FormData();
    data.append('file', files[0]);
    // 서버에서 JSON 응답을 원하면 /api/upload 로 바꿔도 됨
    form.action = '/upload';
    // 폴백: 실제 폼 제출로 동작시키려면 아래 두 줄 대신 form.submit() 사용 가능
    const res = await fetch(form.action, { method: 'POST', body: data, redirect: 'follow' });
    // /upload는 303으로 공유 링크로 리다이렉트되므로, 브라우저가 자동 이동함.
    // (fetch로 보내면 리다이렉트 주소를 새 창으로 열어주자)
    if (res.redirected) {
      window.location.href = res.url;
    } else if (res.headers.get('content-type')?.includes('application/json')) {
      const j = await res.json(); if (j.url) window.location.href = j.url;
    }
  });
</script>
`)
}

func uploadWeb(w http.ResponseWriter, r *http.Request) {
	link, err := saveFromMultipart(w, r)
	if err != nil { http.Error(w, err.Error(), 400); return }
	http.Redirect(w, r, link, http.StatusSeeOther)
}

func uploadAPI(w http.ResponseWriter, r *http.Request) {
	link, err := saveFromMultipart(w, r)
	if err != nil { http.Error(w, err.Error(), 400); return }
	_ = json.NewEncoder(w).Encode(map[string]string{"url": link})
}

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
	if !hmac.Equal([]byte(sign(secret, id)), []byte(r.URL.Query().Get("token"))) {
		http.Error(w, "forbidden", http.StatusForbidden); return
	}
	b, err := os.ReadFile(filepath.Join(dataDir, id, "meta.json"))
	if err != nil { http.NotFound(w, r); return }

	var meta struct{ OrigName, StoredPath string }
	_ = json.Unmarshal(b, &meta)

	f, err := os.Open(filepath.Join(dataDir, id, meta.StoredPath))
	if err != nil { http.NotFound(w, r); return }
	defer f.Close()

	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(meta.OrigName)))
	if ct == "" { ct = "application/octet-stream" }
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName(meta.OrigName)))
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
