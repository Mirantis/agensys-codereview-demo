FROM golang:1.22-alpine

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY main.go .

RUN go build -o pr-agent main.go

EXPOSE 80
CMD ["/app/pr-agent"]
