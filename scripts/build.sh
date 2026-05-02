#!/bin/bash
# Script to build the ghreport binary

BINARY_NAME="ghreport"

echo "🚀 Building ${BINARY_NAME}..."
go build -o ${BINARY_NAME} main.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed!"
    exit 1
fi
