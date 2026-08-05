# install.ps1 - Install walkline on native Windows PowerShell
# This script is for Windows ONLY. For Mac/Linux/WSL/Git-Bash, use install.sh instead.

param(
    [string]$Owner = $env:WALKLINE_OWNER,
    [string]$Repo = $env:WALKLINE_REPO,
    [string]$Version = $env:WALKLINE_VERSION
)

$ErrorActionPreference = "Stop"

if (-not $Owner) { $Owner = "Pantho-Haque" }
if (-not $Repo) { $Repo = "walkline" }

# Detect architecture
$arch = $env:PROCESSOR_ARCHITECTURE
if ($arch -eq "AMD64") { $arch = "amd64" }
elseif ($arch -eq "ARM64") { $arch = "arm64" }
else {
    Write-Error "Unsupported architecture: $arch"
    exit 1
}

$OS = "windows"

# Validate platform combination
$validCombo = @("windows_amd64")
if ($validCombo -notcontains "${OS}_${arch}") {
    Write-Error "No release available for ${OS}/${arch}"
    exit 1
}

# Resolve version
if (-not $Version) {
    Write-Host "Resolving latest release..."
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/${Owner}/${Repo}/releases/latest" -UseBasicParsing
    $Version = $response.tag_name
}

Write-Host "Installing walkline ${Version} for ${OS}/${arch}"

$archiveName = "walkline_${Version.TrimStart('v')}_${OS}_${arch}.zip"
$binaryName = "walkline.exe"

# Create temp directory
$tmpdir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ([System.IO.Path]::GetRandomFileName()))
try {
    # Download release
    Write-Host "Downloading ${archiveName}..."
    $archivePath = Join-Path $tmpdir $archiveName
    Invoke-WebRequest -Uri "https://github.com/${Owner}/${Repo}/releases/download/${Version}/${archiveName}" -OutFile $archivePath -UseBasicParsing

    $checksumsPath = Join-Path $tmpdir "checksums.txt"
    Invoke-WebRequest -Uri "https://github.com/${Owner}/${Repo}/releases/download/${Version}/checksums.txt" -OutFile $checksumsPath -UseBasicParsing

    # Verify checksum
    Write-Host "Verifying checksum..."
    $expectedHash = Get-Content $checksumsPath | ForEach-Object {
        if ($_ -match "^([a-f0-9]+)\s+${archiveName}$") { $matches[1] }
    }
    if (-not $expectedHash) {
        Write-Error "Could not find checksum for ${archiveName}"
    }
    $actualHash = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
    if ($expectedHash.ToLower() -ne $actualHash) {
        Write-Error "Checksum verification failed!"
    }

    # Extract
    Write-Host "Extracting..."
    Expand-Archive -Path $archivePath -DestinationPath $tmpdir -Force
    Remove-Item $archivePath, $checksumsPath

    # Determine install location - find directory on PATH or create new one
    $installDir = $null
    foreach ($dir in $env:PATH -split ';') {
        $cleanDir = $dir.Trim()
        if ($cleanDir -and (Test-Path $cleanDir -PathType Container)) {
            $testPath = Join-Path $cleanDir $binaryName
            if (-not (Test-Path $testPath)) {
                $installDir = $cleanDir
                break
            }
        }
    }

    if (-not $installDir) {
        $installDir = Join-Path $env:LOCALAPPDATA "walkline"
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    # Install binary
    Copy-Item (Join-Path $tmpdir $binaryName) -Destination $installDir -Force
    $installPath = Join-Path $installDir $binaryName

    Write-Host ""
    Write-Host "Installed: $installPath"

    # Check if on PATH
    if ($env:PATH -split ';' -contains $installDir) {
        Write-Host "walkline is on your PATH."
    } else {
        Write-Host ""
        Write-Host "WARNING: $installDir is not on your PATH."
        Write-Host "Add it to PATH with: [Environment]::SetEnvironmentVariable('PATH', \"$installDir;$env:PATH\", 'User')"
    }

    # Set up git hooks template
    Write-Host ""
    Write-Host "Setting up git hooks template..."
    & $installPath install
    if ($LASTEXITCODE -eq 0) {
        Write-Host ""
        Write-Host "walkline is ready to use!"
        Write-Host ""
        Write-Host "Next steps:"
        Write-Host "  1. To track existing repos: walkline scan <directory>"
        Write-Host "  2. New repos are automatically instrumented going forward"
    } else {
        Write-Host ""
        Write-Host "walkline binary installed, but 'walkline install' failed."
        Write-Host "Run 'walkline install' manually to set up the git hooks template."
    }
}
finally {
    # Clean up temp directory
    Remove-Item $tmpdir -Recurse -Force -ErrorAction SilentlyContinue
}
