# Use a minimal Go image to potentially compile inside later if needed,
# or just a minimal base like alpine if always compiling on host.
# Using golang allows building the runner image easily.
FROM golang:1.24-alpine as builder

# For the final image, just use alpine for minimal size
FROM alpine:latest

# Install ca-certificates for potential HTTPS calls if networking WAS enabled
# Although we disable network, it's good practice in base images.
RUN apk --no-cache add ca-certificates

# Create a non-root user and group first
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create app directory with correct permissions
RUN mkdir /app && chown appuser:appgroup /app

# Set working directory
WORKDIR /app

# Switch to non-root user
USER appuser

# Copy the executable from the host (this will be done in docker run command via volume)
# We just define the entrypoint here. The actual binary name might vary.
# The actual command to run will be specified in `docker run`.
# Example placeholder:
# COPY ./program_to_run /app/program_to_run
# ENTRYPOINT ["/app/program_to_run"]