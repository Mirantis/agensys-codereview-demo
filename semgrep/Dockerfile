FROM golang:1.22-alpine

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o semgrep-service .

ENV PORT=80
ENV LOG_LEVEL=info

EXPOSE 80

CMD ["./semgrep-service"]
