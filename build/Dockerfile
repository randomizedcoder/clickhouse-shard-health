FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /clickhouse-shard-health ./cmd/

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /clickhouse-shard-health /clickhouse-shard-health

EXPOSE 9363

ENTRYPOINT ["/clickhouse-shard-health"]
