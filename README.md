# 🚀 GitHub Report AI (ghreport)

[![Go Version](https://img.shields.io/github/go-mod/go-version/sakatimuna7/github-report)](https://golang.org/)
[![License](https://img.shields.io/github/license/sakatimuna7/github-report)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/sakatimuna7/github-report)](https://github.com/sakatimuna7/github-report/releases)

**GitHub Report AI** (`ghreport`) adalah tool CLI cerdas yang mengotomatisasi pembuatan laporan aktivitas GitHub harian atau mingguan Anda menggunakan kecerdasan buatan (AI). Tool ini merangkum commit Anda menjadi poin-poin yang profesional dan siap dikirim atau disimpan.

---

## ✨ Fitur Utama

- 🤖 **AI Summarization**: Mendukung model **Groq (Llama 3)** dan **Google Gemini** untuk ringkasan yang akurat.
- 📊 **Sheets Integration**: Langsung ekspor laporan ke **Google Sheets** secara otomatis.
- 🖥️ **Interactive TUI**: Antarmuka terminal yang cantik menggunakan framework [Charm Bracelet](https://charm.sh/).
- 📅 **Flexible Date Selection**: Pilih rentang tanggal (hari ini, kemarin, atau custom) dengan mudah.
- 🔄 **Auto Update**: Fitur self-update langsung dari terminal untuk memastikan Anda selalu menggunakan versi terbaru.
- 🔐 **Encrypted Config**: Penyimpanan API Key yang aman dengan enkripsi AES.

---

## 📋 Daftar Isi

1. [Instalasi](#-instalasi)
   - [macOS](#macos)
   - [Linux](#linux)
   - [Windows](#windows)
   - [Build from Source](#build-from-source)
2. [Konfigurasi](#-konfigurasi)
3. [Penggunaan](#-penggunaan)
4. [Pengembangan](#-pengembangan)

---

## 🛠 Instalasi

Pilih metode instalasi yang paling mudah untuk Anda:

### One-Liner (macOS & Linux)
Cara paling simpel tanpa perlu menginstal Go:

```bash
curl -sSL https://raw.githubusercontent.com/sakatimuna7/github-report/main/scripts/install.sh | bash
```

### Build from Source (Opsional)
Jika Anda ingin melakukan build sendiri dan sudah memiliki [Go](https://go.dev/doc/install) (versi 1.25+):

```bash
go install github.com/sakatimuna7/github-report@latest
```

### Windows (PowerShell)
Cara paling simpel untuk menginstal secara global:

```powershell
iwr https://raw.githubusercontent.com/sakatimuna7/github-report/main/scripts/install.ps1 | iex
```
*Setelah menjalankan perintah di atas, silakan restart terminal Anda.*

---

## ⚙️ Konfigurasi

Saat pertama kali dijalankan, `ghreport` akan meminta Anda mengisi konfigurasi melalui menu **Settings**. Namun, Anda juga bisa menyiapkan file `.env` di direktori aplikasi:

```env
# API Keys
GITHUB_TOKEN=ghp_your_token_here
GROQ_API_KEY=gsk_your_groq_key_here
GEMINI_API_KEY=your_gemini_key_here

# User Info
DEVELOPER_NAME="Nama Anda"
DIVISI="Mobile/Web/Backend"

# Google Sheets (Optional)
SHEETS_ID=your_google_sheets_id_here
GOOGLE_CREDENTIALS_PATH=/path/to/credentials.json

# Work Hours (Optional)
WORK_START=9
WORK_END=18
```

> [!TIP]
> Anda bisa mendapatkan GitHub Token di [Settings > Developer Settings > Personal Access Tokens](https://github.com/settings/tokens).

---

## 🚀 Penggunaan

Cukup jalankan perintah berikut di terminal Anda:

```bash
ghreport
```

### Navigasi
- **Arrow Keys**: Berpindah menu.
- **Enter**: Memilih/Konfirmasi.
- **Esc/Ctrl+C**: Keluar.

### Menu Utama
1. **Generate Report**: Mulai proses penarikan commit dan pembuatan ringkasan AI.
2. **Settings**: Atur API Keys dan profil pengguna.
3. **Update**: Cek dan instal versi terbaru.
4. **Exit**: Keluar dari aplikasi.

---

## 👨‍💻 Pengembangan

Jika Anda ingin berkontribusi atau melakukan modifikasi:

```bash
# Clone repository
git clone https://github.com/sakatimuna7/github-report.git

# Build binary lokal
make build

# Menjalankan aplikasi
./ghreport

# Cross-compile untuk semua platform
make release
```

---

## 📄 Lisensi

Proyek ini dilisensikan di bawah [MIT License](LICENSE).

---

<p align="center">
  Dibuat dengan ❤️ oleh <a href="https://github.com/sakatimuna7">sakatimuna7</a>
</p>
