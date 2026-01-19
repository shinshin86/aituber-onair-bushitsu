Param(
  [Parameter(Position=0)]
  [string]$Version,
  [switch]$Help
)

$ErrorActionPreference = 'Stop'

function Show-Usage {
  Write-Host "Usage:" 
  Write-Host "  .\\install.ps1 [vX.Y.Z]"
  Write-Host "  .\\install.ps1 -Help"
  Write-Host ""
  Write-Host "Examples:"
  Write-Host "  .\\install.ps1"
  Write-Host "  .\\install.ps1 v1.2.3"
  Write-Host ""
  Write-Host "Environment:"
  Write-Host "  BIN_DIR   Install directory (default: %LOCALAPPDATA%\\Programs\\bushitsu)"
}

function Fail([string]$Message) {
  throw $Message
}

try {
  if ($Help) {
    Show-Usage
    exit 0
  }

  $Owner = 'shinshin86'
  $Repo = 'aituber-onair-bushitsu'
  $BinName = 'bushitsu.exe'

  if ($env:OS -ne 'Windows_NT') {
    Fail "Unsupported OS: $($env:OS). Windows only."
  }

  $archRaw = $env:PROCESSOR_ARCHITECTURE
  switch ($archRaw) {
    'AMD64' { $Arch = 'amd64' }
    'ARM64' { $Arch = 'arm64' }
    default { Fail "Unsupported CPU architecture: $archRaw (only amd64/arm64 are supported)." }
  }

  if ([string]::IsNullOrEmpty($Version)) {
    $Tag = 'latest'
  } elseif ($Version -match '^v\d+\.\d+\.\d+$') {
    $Tag = $Version
  } else {
    Show-Usage
    Fail "Invalid version: $Version (expected vX.Y.Z)."
  }

  [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

  if ($Tag -eq 'latest') {
    Write-Host "==> Resolving latest release tag via GitHub API..."
    $apiUrl = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    $latest = Invoke-RestMethod -Uri $apiUrl -Method Get
    if (-not $latest.tag_name) {
      Fail "Failed to parse tag_name from GitHub API."
    }
    $Tag = $latest.tag_name
  }

  $asset = "bushitsu_${Tag}_windows_${Arch}.zip"
  $checksums = 'checksums.txt'
  $baseUrl = "https://github.com/$Owner/$Repo/releases/download/$Tag"

  Write-Host "==> Target release: $Tag"
  Write-Host "==> Asset: $asset"

  $tmpDir = New-Item -ItemType Directory -Path ([System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.IO.Path]::GetRandomFileName()))
  try {
    $checksumsPath = Join-Path $tmpDir.FullName $checksums
    Write-Host "==> Downloading checksums..."
    Invoke-WebRequest -Uri "$baseUrl/$checksums" -OutFile $checksumsPath -UseBasicParsing

    $expectedSha = $null
    Get-Content $checksumsPath | ForEach-Object {
      if ($_ -match '^(?<sha>[0-9a-fA-F]{64})\s+(?<file>\S+)$') {
        if ($Matches['file'] -eq $asset) {
          $expectedSha = $Matches['sha'].ToLower()
        }
      }
    }
    if (-not $expectedSha) {
      Fail "Checksum for $asset not found in $checksums."
    }

    $zipPath = Join-Path $tmpDir.FullName $asset
    Write-Host "==> Downloading asset..."
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $zipPath -UseBasicParsing

    $actualSha = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToLower()
    if ($expectedSha -ne $actualSha) {
      Fail "Checksum mismatch for $asset (expected $expectedSha, got $actualSha)."
    }

    Write-Host "==> Extracting..."
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir.FullName -Force

    $binPath = Join-Path $tmpDir.FullName $BinName
    if (-not (Test-Path $binPath)) {
      Fail "Binary not found in archive: $BinName"
    }

    # Use LocalAppData\Programs for user-scope installs without requiring admin rights.
    $binDir = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\bushitsu' }
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null

    $dest = Join-Path $binDir $BinName
    if (Test-Path $dest) {
      $ts = Get-Date -Format 'yyyyMMddHHmmss'
      $backup = "$dest.$ts.bak"
      Write-Host "==> Existing binary found, backing up to $backup"
      Move-Item -Path $dest -Destination $backup -Force
    }

    Write-Host "==> Installing to $dest"
    Copy-Item -Path $binPath -Destination $dest -Force

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ([string]::IsNullOrEmpty($userPath)) {
      $userPath = ''
    }

    $pathParts = $userPath -split ';' | Where-Object { $_ -ne '' }
    $alreadyInPath = $false
    foreach ($p in $pathParts) {
      if ($p.Trim().ToLower() -eq $binDir.Trim().ToLower()) { $alreadyInPath = $true }
    }

    if (-not $alreadyInPath) {
      $newPath = if ($userPath -eq '') { $binDir } else { "$userPath;$binDir" }
      [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
      Write-Host "==> Added $binDir to your user PATH (effective in new terminals)."
    }

    Write-Host ""
    Write-Host "Installed $BinName $Tag to $dest"
    Write-Host "Try: $BinName --help"
    if (-not $alreadyInPath) {
      Write-Host "Note: PATH changes apply to new PowerShell/Command Prompt windows."
    }
  }
  finally {
    if ($tmpDir -and (Test-Path $tmpDir.FullName)) {
      Remove-Item -Path $tmpDir.FullName -Recurse -Force
    }
  }
}
catch {
  Write-Error "Install failed: $($_.Exception.Message)"
  exit 1
}
