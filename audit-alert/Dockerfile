# ビルドステージ
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# 依存関係のキャッシュを利用するために先にコピー
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# 静的バイナリとしてビルド
RUN CGO_ENABLED=0 GOOS=linux go build -v -o audit-alert .

# 実行ステージ
FROM gcr.io/distroless/static-debian12

# バイナリをコピー
COPY --from=builder /app/audit-alert /audit-alert

# 実行
ENTRYPOINT ["/audit-alert"]
