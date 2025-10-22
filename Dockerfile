FROM golang:1.24-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o kyverno-watcher .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl

RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')/kubectl" -o /usr/local/bin/kubectl \
  && chmod +x /usr/local/bin/kubectl

WORKDIR /app

COPY --from=builder /build/kyverno-watcher /usr/local/bin/kyverno-watcher

ENTRYPOINT ["/usr/local/bin/kyverno-watcher"]
