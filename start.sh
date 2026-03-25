#!/bin/bash
set -e

echo "=== AirPlay Server ==="
echo ""

# Build Go backend
echo "Building Go backend..."
cd server
go build -o ../bin/airplay-server ./cmd/
cd ..
echo "✓ Backend built"

# Install frontend deps if needed
if [ ! -d "frontend/node_modules" ]; then
    echo "Installing frontend dependencies..."
    cd frontend && npm install && cd ..
fi

echo ""
echo "Starting services..."
echo ""

# Start Go backend
./bin/airplay-server "$@" &
BACKEND_PID=$!

# Start Next.js frontend
cd frontend
npx next dev -p 3000 &
FRONTEND_PID=$!
cd ..

echo ""
echo "=== Services Running ==="
echo "Backend:  http://localhost:7000 (AirPlay)"
echo "Frontend: http://localhost:3000 (Display)"
echo ""
echo "Press Ctrl+C to stop"

cleanup() {
    echo ""
    echo "Stopping services..."
    kill $BACKEND_PID 2>/dev/null || true
    kill $FRONTEND_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGINT SIGTERM

wait
