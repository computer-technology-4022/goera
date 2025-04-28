FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o built main.go

EXPOSE 5000

CMD ["./built", "serve", "--listen", "5000"]