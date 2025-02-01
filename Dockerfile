FROM golang:1.23-bookworm AS builder

WORKDIR /app

RUN apt-get update && \
    apt-get install -y git ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/main .

RUN useradd -m appuser && chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

CMD ["./main"]
