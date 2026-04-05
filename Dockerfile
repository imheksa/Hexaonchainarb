FROM golang:1.21-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/hexa-arb ./cmd

# ─── Runtime image ────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/hexa-arb /app/hexa-arb

EXPOSE 8080 9090 9091

ENTRYPOINT ["/app/hexa-arb"]
