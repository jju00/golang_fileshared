# 📁 Golang File Shared

간단하고 안전한 파일 공유 플랫폼입니다. 웹 UI와 API를 통해 파일을 업로드하고, 고유한 링크로 공유할 수 있습니다.

🌐 **Live Service**: [https://golang-fileshared.onrender.com](https://golang-fileshared.onrender.com)

<br>

## ✨ 주요 기능

- 🎨 **직관적인 웹 UI** - 드래그 앤 드롭 업로드 지원
- 📸 **갤러리 뷰** - 업로드된 파일을 카드 형식으로 확인
- 🖼️ **이미지 미리보기** - PNG, JPG, JPEG 파일 썸네일 표시
- 🔐 **보안 다운로드** - HMAC 토큰 기반 인증
- 🚀 **REST API** - curl 및 프로그래밍 언어로 쉽게 연동
- 📦 **Docker 지원** - 컨테이너 기반 배포
- 🔍 **파일 검증** - 확장자, MIME 타입, 경로 검증 등 보안 처리

<br>

## 🛠️ 기술 스택

- **Backend**: Go 1.23
- **Frontend**: Vanilla HTML/CSS/JavaScript
- **Container**: Docker
- **Deployment**: Render.com

## 📋 지원 파일 형식

| 형식 | 확장자 | 최대 크기 |
|------|--------|-----------|
| 텍스트 | `.txt` | 20MB |
| 문서 | `.pdf` | 20MB |
| 이미지 | `.png`, `.jpg`, `.jpeg` | 20MB |
| 압축 | `.zip` | 20MB |

<br>

## 🚀 사용 방법

### 1️⃣ 웹 브라우저로 업로드

1. https://golang-fileshared.onrender.com 접속
2. 파일을 드래그 앤 드롭하거나 **"파일 선택"** 버튼 클릭
3. 업로드 완료 후 공유 가능한 다운로드 링크 자동 생성
4. **"📁 업로드된 파일 보기"** 클릭하여 갤러리에서 모든 파일 확인

### 2️⃣ curl로 업로드 (API)

```bash
# 파일 업로드
curl -F file=@myfile.pdf https://golang-fileshared.onrender.com/api/upload

# 응답 예시
{
  "url": "https://golang-fileshared.onrender.com/d/abc123.../myfile.pdf?token=xyz..."
}
```

### 3️⃣ 파일 다운로드

업로드 후 받은 URL을 통해 파일 다운로드:

```bash
# curl로 다운로드
curl "https://golang-fileshared.onrender.com/d/{id}/{filename}?token={token}" -O

# wget으로 다운로드
wget "https://golang-fileshared.onrender.com/d/{id}/{filename}?token={token}"
```

브라우저에서 URL을 직접 열면:
- 📸 **이미지 파일**: 브라우저에서 바로 미리보기
- 📄 **기타 파일**: 자동 다운로드

### 4️⃣ 파일 목록 조회 (API)

```bash
curl https://golang-fileshared.onrender.com/api/files
```

**응답 예시:**
```json
[
  {
    "id": "abc123...",
    "name": "example.pdf",
    "size": 12345,
    "uploaded_at": "2025-11-20T10:30:00+09:00",
    "url": "https://golang-fileshared.onrender.com/d/abc123.../example.pdf?token=xyz...",
    "is_image": false
  },
  {
    "id": "def456...",
    "name": "photo.jpg",
    "size": 98765,
    "uploaded_at": "2025-11-20T11:15:00+09:00",
    "url": "https://golang-fileshared.onrender.com/d/def456.../photo.jpg?token=abc...",
    "is_image": true
  }
]
```

<br>

## 🔐 보안 기능

### 1. HMAC 토큰 인증
모든 다운로드 링크는 HMAC-SHA256 서명으로 보호됩니다. 서버의 비밀키 없이는 유효한 링크를 생성할 수 없습니다.

### 2. 파일 타입 검증
- 파일 확장자와 실제 Content-Type 일치 여부 검증
- 허용되지 않은 파일 형식 자동 차단
- Magic byte 검사로 파일 위변조 방지

### 3. 파일 크기 제한
- 업로드당 최대 20MB 제한
- DoS 공격 및 서버 자원 고갈 방지

### 4. 안전한 파일명 처리
- 파일명에서 위험한 문자 제거 (`/`, `\`, `..` 등)
- Path traversal 공격 방지
- 서버 파일 시스템 보호

<br>

## 🛠️ 로컬 실행 방법

### 요구사항
- Go 1.23+
- Git

### 설치 및 실행

```bash
# 저장소 클론
git clone https://github.com/yourusername/golang_file_shared.git
cd golang_file_shared

# 의존성 설치
go mod download

# 서버 실행
go run main.go
```

서버가 시작되면 http://localhost:8080 에서 접속할 수 있습니다.

### 환경 변수 설정

```bash
# 로컬 개발용
export BASE_URL=http://localhost:8080
export PORT=8080
export DATA_DIR=./uploads
export TOKEN_SECRET=dev-secret-change-me
export MAX_UPLOAD_MB=20

go run main.go
```

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `PORT` | 서버 포트 | `8080` |
| `BASE_URL` | 서비스 기본 URL | `http://localhost:8080` |
| `DATA_DIR` | 파일 저장 디렉토리 | `./uploads` |
| `TOKEN_SECRET` | HMAC 서명 비밀키 | `dev-secret-change-me` |
| `MAX_UPLOAD_MB` | 최대 업로드 크기 (MB) | `20` |

### Docker로 로컬 실행

```bash
# Docker 이미지 빌드
docker build -t file-shared .

# 컨테이너 실행
docker run -p 8080:8080 \
  -v $(pwd)/uploads:/app/uploads \
  -e BASE_URL=http://localhost:8080 \
  file-shared
```

**Docker Compose:**

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./uploads:/app/uploads
    environment:
      - BASE_URL=http://localhost:8080
      - TOKEN_SECRET=change-this-in-production
      - MAX_UPLOAD_MB=20
```

실행:
```bash
docker-compose up
```

<br>

## 📝 라이센스

MIT License
