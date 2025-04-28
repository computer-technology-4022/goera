FROM golang:latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build code-runner component
WORKDIR /app/code-runner
RUN go build -o code-runner .  # Builds all .go files in the directory
WORKDIR /app

# Build judge component
RUN go build -o judge judge.go

# Create a start script
RUN echo '#!/bin/sh\n./judge serve --listen 8080 &\n./judge code-runner\nwait' > start.sh && \
    chmod +x start.sh

EXPOSE 8080

CMD ["./start.sh"]