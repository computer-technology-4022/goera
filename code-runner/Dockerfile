FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o code-runner code-runner.go

EXPOSE 8081

CMD ["./code-runner", "serve", "--listen", "8081", "--judge", "http://judge:8080"]