FROM docker:28-dind


RUN apk add --no-cache go



# Set up Go environment (optional but good practice)

ENV GOPATH=/go

ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH



# Create and set the working directory

RUN mkdir -p "$GOPATH/src/app"

WORKDIR "$GOPATH/src/app"



# Copy Go module files and download dependencies

COPY go.mod go.sum ./

RUN go mod download



# Copy the rest of your application source code

COPY . .



# Build code-runner component

WORKDIR "$GOPATH/src/app/code-runner"


RUN go build . # This is the line that fails

WORKDIR "$GOPATH/src/app"



# Build judge component (server part)

RUN go build . # Output to a location in PATH



# --- Custom Start Script ---

# The base docker:dind image has an entrypoint that starts dockerd.

# We need to start that *and* our Go applications.

# Create a new entrypoint script.

COPY start-dind.sh /usr/local/bin/start-dind.sh

RUN chmod +x /usr/local/bin/start-dind.sh



# Expose the judge server port

EXPOSE 8080



# Use our custom script as the entrypoint

ENTRYPOINT ["/usr/local/bin/start-dind.sh"]



# Default command (can be overridden in docker-compose)

# Not strictly needed if entrypoint handles everything, but can be good practice

CMD [""]

