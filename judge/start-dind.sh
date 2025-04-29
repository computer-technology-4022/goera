#!/bin/sh
# Use /bin/sh for broader compatibility (Alpine uses ash)

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&

echo "Starting Code Runner..."
./judge coderunner &&
echo "Starting Judge Server..."
./judge serve --listen 8080 &&

# Optional: Add a wait command if both judge and code-runner were backgrounded
wait
