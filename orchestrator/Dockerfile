FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git curl ca-certificates

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o orchestrator .

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache git curl ca-certificates

COPY --from=builder /app/orchestrator .

ENV LISTEN_ADDR=:8085
ENV LOG_LEVEL=debug

EXPOSE 8085

CMD ["./orchestrator"]
