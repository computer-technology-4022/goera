#!/bin/sh
# Use /bin/sh for broader compatibility (Alpine uses ash)

# Start the Docker daemon in the background.
# Use the original entrypoint provided by the base image to ensure
# it sets up cgroups, storage drivers, etc., correctly.
# Run it in the background (&)
/usr/local/bin/dockerd-entrypoint.sh &

# Wait for the Docker daemon to be ready by polling the docker socket.
echo "Waiting for nested Docker daemon to start..."
while ! docker info > /dev/null 2>&1; do
    echo -n "."
    sleep 1
done
echo "\nNested Docker daemon started."

# Now that the *internal* Docker daemon is running, start your applications.

# Start the judge server in the background

# Start the code runner in the foreground (or judge code-runner)
# This will keep the container running.

(
echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner 
# Or: judge code-runner
) &&

echo "Starting Judge Server..."
./judge serve --listen 8080 &&

# Optional: Add a wait command if both judge and code-runner were backgrounded
wait
