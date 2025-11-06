# ---- Build stage ----
    FROM golang:1.23-alpine AS build
    WORKDIR /app
    
    # go.mod, go.sum 먼저 복사 후 의존성 다운로드
    COPY go.mod go.sum* ./
    RUN go mod download
    
    # 소스 복사
    COPY . .
    
    # 빌드 (CGO 비활성화 + 리눅스용)
    ENV CGO_ENABLED=0 GOOS=linux
    RUN go build -o server .
    
    # ---- Run stage ----
    FROM scratch
    WORKDIR /
    COPY --from=build /app/server /server
    EXPOSE 8080
    
    # 환경변수 기본값
    ENV BASE_URL=http://localhost:8080
    ENV DATA_DIR=/data
    
    ENTRYPOINT ["/server"]
    