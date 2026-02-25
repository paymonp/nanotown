$ErrorActionPreference = "Stop"

$repo = "paymonp/nanotown"
$installDir = "$env:LOCALAPPDATA\Programs\nanotown"
$binaryName = "nt.exe"

# Detect architecture
$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    "AMD64" { $arch = "amd64" }
    default { Write-Error "Unsupported architecture: $arch"; exit 1 }
}

# Fetch latest release tag
Write-Host "Fetching latest release..."
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
$tag = $release.tag_name

if (-not $tag) {
    Write-Error "Could not determine latest release tag"
    exit 1
}

$asset = "nanotown-windows-${arch}.exe"
$url = "https://github.com/$repo/releases/download/$tag/$asset"

Write-Host "Downloading $asset ($tag)..."
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

Invoke-WebRequest -Uri $url -OutFile "$installDir\$binaryName" -UseBasicParsing

# Add to user PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host ""
    Write-Host "Added $installDir to your user PATH."
    Write-Host "Restart your terminal for the change to take effect."
}

Write-Host ""
Write-Host "Installed $binaryName to $installDir\$binaryName"
