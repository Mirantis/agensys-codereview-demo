FROM golang:1.22-alpine

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod ./
RUN go mod download || true

COPY main.go .

RUN go build -o summarizer-agent .

EXPOSE 80
CMD ["./summarizer-agent"]
