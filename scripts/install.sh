#!/bin/bash
# Script to build and install the ghreport binary to /usr/local/bin

BINARY_NAME="ghreport"
INSTALL_PATH="/usr/local/bin"

# Build first
bash scripts/build.sh

if [ $? -ne 0 ]; then
    exit 1
fi

echo "📦 Installing ${BINARY_NAME} to ${INSTALL_PATH}..."
sudo mv ${BINARY_NAME} ${INSTALL_PATH}/${BINARY_NAME}

if [ $? -eq 0 ]; then
    echo "🎉 Successfully installed ${BINARY_NAME}!"
    echo "You can now run it by typing '${BINARY_NAME}' in your terminal."
else
    echo "❌ Installation failed! You might need to provide sudo password."
    exit 1
fi
