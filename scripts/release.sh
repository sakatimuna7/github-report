#!/bin/bash
# Script to create production-grade releases

echo "📦 Starting release process..."

# Ensure we are in the root directory
cd "$(dirname "$0")/.."

# Clean previous builds
echo "🧹 Cleaning up old builds..."
make clean

# Create releases
echo "🏗️  Building for multiple platforms..."
make release

if [ $? -eq 0 ]; then
    echo "✨ All release binaries are ready in the dist/ folder!"
    ls -lh dist/
else
    echo "❌ Release build failed!"
    exit 1
fi
