#!/bin/bash
set -e
make build
echo ""
echo "Starting AirPlay Server..."
./bin/airplay-server "$@"
