<#
.SYNOPSIS
    Compilación cruzada universal para CloudflareNetConnect.
.DESCRIPTION
    Genera binarios optimizados (-s -w) sin dependencias C (CGO_ENABLED=0)
    para Windows (x64), Linux (x64) y macOS (Apple Silicon ARM64).
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$ProjectDir = Split-Path -Parent $PSScriptRoot
$BinDir = Join-Path $ProjectDir "bin"

Push-Location $ProjectDir
try {
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }

    $GoExe = if (Get-Command go -ErrorAction SilentlyContinue) { "go" } else { "$env:USERPROFILE\scoop\shims\go.exe" }
    if (-not (Test-Path $GoExe) -and $GoExe -ne "go") {
        Write-Error "No se encontró el ejecutable de Go."
    }

    Write-Host "Compilando CloudflareNetConnect..." -ForegroundColor Cyan

    # Cerrar instancia en ejecución si existe para liberar el archivo .exe
    Get-Process -Name "CloudflareNetConnect" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

    # 1. Windows (AMD64) con subsistema GUI silencioso
    Write-Host "  -> Generando binario Windows (GUI Silencioso)..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    & $GoExe build -ldflags "-s -w -H=windowsgui" -o (Join-Path $BinDir "CloudflareNetConnect.exe") ./cmd/daemon
    if ($LASTEXITCODE -ne 0) { throw "Error compilando binario Windows" }

    # 2. Linux (AMD64)
    Write-Host "  -> Generando binario Linux..."
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    & $GoExe build -ldflags "-s -w" -o (Join-Path $BinDir "CloudflareNetConnect-linux") ./cmd/daemon
    if ($LASTEXITCODE -ne 0) { throw "Error compilando binario Linux" }

    # 3. macOS (Apple Silicon ARM64)
    Write-Host "  -> Generando binario macOS (ARM64)..."
    $env:GOOS = "darwin"
    $env:GOARCH = "arm64"
    & $GoExe build -ldflags "-s -w" -o (Join-Path $BinDir "CloudflareNetConnect-mac-arm64") ./cmd/daemon
    if ($LASTEXITCODE -ne 0) { throw "Error compilando binario macOS" }

    Write-Host "[OK] Matriz de binarios generada con éxito en $BinDir" -ForegroundColor Green
    Get-ChildItem -Path $BinDir | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize
}
finally {
    Pop-Location
}
