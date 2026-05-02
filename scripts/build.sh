#!/bin/bash
# Script to build the ghreport binary

echo "🚀 Building via Makefile..."
make build

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed!"
    exit 1
fi
