# ---- Build stage ----
    FROM golang:1.23-alpine AS build
    WORKDIR /app
    
    # go.mod, go.sum 먼저 복사 후 의존성 다운로드
    COPY go.mod go.sum* ./
    RUN go mod download
    
    # 소스 복사
    COPY . .
    
    # 파일 확인 (디버깅)
    RUN ls -la && echo "Checking HTML files:" && ls -la *.html || echo "No HTML files found"
    
    # 빌드 (CGO 비활성화 + 리눅스용)
    ENV CGO_ENABLED=0 GOOS=linux
    RUN go build -o server .
    
# ---- Run stage ----
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=build /app/server /app/server
RUN mkdir -p /app/uploads
    
ENTRYPOINT ["/app/server"]
    