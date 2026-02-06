# ビルドステージ
FROM golang:1.25-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -v -o ai-web-summarizer .

# 実行ステージ
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/ai-web-summarizer /ai-web-summarizer

ENTRYPOINT ["/ai-web-summarizer"]
