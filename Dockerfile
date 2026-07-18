# Navo NT QQ BOT 论坛 - Dockerfile
# 多阶段构建，ARM64 友好

FROM golang:1.22-alpine AS builder

WORKDIR /app

# 依赖缓存
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /navo-forum ./cmd/server

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /navo-forum .
COPY configs/ ./configs/

RUN mkdir -p data && chown -R app:app /app

USER app

EXPOSE 8080

CMD ["./navo-forum", "-config", "configs/config.yaml"]
