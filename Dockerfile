FROM golang:1.15.2 AS builder

WORKDIR /app
COPY . .
RUN mkdir -p /app/bin && CGO_ENABLED=0 go build -o bin/server ./cmd/storage-gatekeeper/

FROM scratch
WORKDIR /app
COPY --from=builder /app/bin/server .
ENTRYPOINT ["/app/server"]
