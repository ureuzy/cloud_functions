FROM golang:1.23-alpine AS builder

# Install necessary packages
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o daily-poster .

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/daily-poster .

CMD ["./daily-poster"]
