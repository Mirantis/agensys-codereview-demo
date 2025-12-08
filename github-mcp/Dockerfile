FROM golang:1.22-alpine

RUN apk add --no-cache git curl ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -o github-mcp-server .

EXPOSE 80

CMD ["./github-mcp-server"]
