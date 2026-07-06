FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/hubproxy ./cmd/hubproxy

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/hubproxy /usr/local/bin/hubproxy
COPY web ./web
EXPOSE 8080
ENTRYPOINT ["hubproxy"]
