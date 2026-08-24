# Installs the kryptic CLI + tray app for the current user and registers the
# tray app (which runs the daemon in-process) to start at login.
# Usage:  powershell -ExecutionPolicy Bypass -File install.ps1 [-BinaryDir .]
param(
    [string]$BinaryDir = "."
)

$ErrorActionPreference = "Stop"

$cli  = Join-Path $BinaryDir "kryptic_windows_amd64.exe"
$tray = Join-Path $BinaryDir "kryptic-tray_windows_amd64.exe"

if (-not (Test-Path $cli)) {
    Write-Error "kryptic_windows_amd64.exe not found in $BinaryDir"
}

$installDir = Join-Path $env:LOCALAPPDATA "Kryptic"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# Stop a previous install so the new files can replace it. Credential Manager
# keeps the session; the user does not have to sign in again.
Get-Process kryptic-tray, kryptic -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

Copy-Item $cli  (Join-Path $installDir "kryptic.exe")  -Force
if (Test-Path $tray) {
    Copy-Item $tray (Join-Path $installDir "kryptic-tray.exe") -Force
}

# Put the CLI on the user PATH (no admin rights needed for the user scope).
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to your user PATH (open a new terminal to pick it up)."
}

# Start the tray app at login - it runs the daemon in-process.
if (Test-Path (Join-Path $installDir "kryptic-tray.exe")) {
    $runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
    Set-ItemProperty -Path $runKey -Name "Kryptic" -Value ('"' + (Join-Path $installDir "kryptic-tray.exe") + '"')
    Start-Process (Join-Path $installDir "kryptic-tray.exe")
    Write-Host "Kryptic tray installed and running; it starts automatically at login."
} else {
    Write-Host "Tray app not found - start the daemon manually with: kryptic start"
}

Write-Host ""
Write-Host "Done. Existing sign-in is kept. If this is a first install, run: kryptic login"
