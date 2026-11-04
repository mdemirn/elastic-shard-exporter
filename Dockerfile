FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o elasticsearch-shard-exporter .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/elasticsearch-shard-exporter /usr/local/bin/

RUN adduser -D -g '' exporter
USER exporter

EXPOSE 9061

ENTRYPOINT ["elasticsearch-shard-exporter"]
CMD ["--listen-address=:9061"]
