#!/bin/bash

# GitHub Repo Info
REPO="sakatimuna7/github-report"
BINARY_NAME="ghreport"
INSTALL_PATH="/usr/local/bin"

echo "🚀 Memulai instalasi ${BINARY_NAME}..."

# 1. Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "❌ Arsitektur ${ARCH} tidak didukung."; exit 1 ;;
esac

# Mapping OS/Arch to release binary names
if [ "${OS}" == "darwin" ]; then
    BINARY_FILE="${BINARY_NAME}-darwin-${ARCH}"
elif [ "${OS}" == "linux" ]; then
    BINARY_FILE="${BINARY_NAME}-linux-${ARCH}"
else
    echo "❌ OS ${OS} tidak didukung oleh script ini. Silakan download manual di GitHub Releases."
    exit 1
fi

echo "🔍 Mendeteksi sistem: ${OS} (${ARCH})"

# 2. Get latest release version from GitHub API
echo "🌐 Mencari versi terbaru..."
LATEST_RELEASE=$(curl -s https://api.github.com/repos/${REPO}/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "${LATEST_RELEASE}" ]; then
    echo "❌ Gagal mendapatkan versi terbaru. Pastikan Anda memiliki koneksi internet."
    exit 1
fi

echo "📦 Versi terbaru: ${LATEST_RELEASE}"

# 3. Download Binary
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${BINARY_FILE}"
echo "📥 Mendownload dari: ${DOWNLOAD_URL}"

curl -L -o "${BINARY_NAME}" "${DOWNLOAD_URL}"

if [ $? -ne 0 ]; then
    echo "❌ Download gagal! Pastikan binary untuk platform Anda sudah tersedia di rilis tersebut."
    exit 1
fi

# 4. Install Binary
chmod +x "${BINARY_NAME}"
echo "🛡️ Memindahkan binary ke ${INSTALL_PATH} (mungkin memerlukan password sudo)..."

sudo mv "${BINARY_NAME}" "${INSTALL_PATH}/${BINARY_NAME}"

if [ $? -eq 0 ]; then
    echo "✅ Berhasil! Anda sekarang dapat menjalankan '${BINARY_NAME}' di terminal Anda."
else
    echo "❌ Gagal memindahkan binary. Pastikan Anda memiliki akses sudo."
    exit 1
fi
