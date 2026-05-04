# PowerShell Install Script for ghreport
$Repo = "sakatimuna7/github-report"
$BinaryName = "ghreport.exe"
$InstallDir = "$HOME\.ghreport"

Write-Host "🚀 Memulai instalasi ghreport untuk Windows..." -ForegroundColor Cyan

# 1. Create Install Directory
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
    Write-Host "📂 Membuat folder instalasi di $InstallDir" -ForegroundColor Gray
}

# 2. Get Latest Release
Write-Host "🌐 Mencari versi terbaru..." -ForegroundColor Gray
$ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $ReleaseInfo.tag_name
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/ghreport-windows-amd64.exe"

Write-Host "📦 Versi terbaru: $Version" -ForegroundColor Green

# 3. Download Binary
Write-Host "📥 Mendownload binary..." -ForegroundColor Gray
Invoke-WebRequest -Uri $DownloadUrl -OutFile "$InstallDir\$BinaryName"

# 4. Add to PATH if not already there
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "🛠 Menambahkan ke PATH..." -ForegroundColor Cyan
    [System.Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "✅ Berhasil ditambahkan ke User PATH." -ForegroundColor Green
} else {
    Write-Host "✅ Folder sudah ada di PATH." -ForegroundColor Green
}

Write-Host "🎉 Instalasi selesai! Silakan buka terminal baru dan ketik 'ghreport'." -ForegroundColor Cyan
