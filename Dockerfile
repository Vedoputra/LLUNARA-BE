# Stage 1: build
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# Stage 2: run
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/api /usr/local/bin/api

# Hanya dipakai untuk development & testing lokal — deploy production memakai
# Vercel Go Framework Preset (lihat vercel.json), bukan image ini. Jangan hardcode port di kode.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
