FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/tohub ./cmd/tohub

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/tohub /usr/local/bin/tohub
COPY web ./web
EXPOSE 8080
ENTRYPOINT ["tohub"]
