$ErrorActionPreference = "Stop"

$RepoUrl = "https://github.com/Racle/agent-team-pipeline.git"
$InstallDir = Join-Path $env:LOCALAPPDATA "agent-team-pipeline"

# Clone or update repo
if (Test-Path $InstallDir) {
    Write-Host "Updating existing clone at $InstallDir"
    git -C $InstallDir pull --quiet
} else {
    Write-Host "Cloning agent-team-pipeline..."
    git clone --quiet $RepoUrl $InstallDir
}

# Determine config directory
if ($env:HOME -and (Test-Path (Join-Path $env:HOME ".config"))) {
    $ConfigDir = Join-Path $env:HOME ".config/opencode"
} else {
    $ConfigDir = Join-Path $env:APPDATA "opencode"
}

if (-not (Test-Path $ConfigDir)) {
    New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
}

$AgentsDir = Join-Path $ConfigDir "agents"
$CommandsDir = Join-Path $ConfigDir "commands"

# Back up existing dirs
$Timestamp = Get-Date -Format "yyyyMMddHHmmss"

if ((Test-Path $AgentsDir) -and -not (Get-Item $AgentsDir).Attributes.HasFlag([System.IO.FileAttributes]::ReparsePoint)) {
    $BackupName = "agents.bak.$Timestamp"
    Rename-Item $AgentsDir (Join-Path $ConfigDir $BackupName)
    Write-Host "Backed up existing agents/ to $BackupName"
}

if ((Test-Path $CommandsDir) -and -not (Get-Item $CommandsDir).Attributes.HasFlag([System.IO.FileAttributes]::ReparsePoint)) {
    $BackupName = "commands.bak.$Timestamp"
    Rename-Item $CommandsDir (Join-Path $ConfigDir $BackupName)
    Write-Host "Backed up existing commands/ to $BackupName"
}

# Remove existing symlinks/junctions
if (Test-Path $AgentsDir) { Remove-Item $AgentsDir -Force }
if (Test-Path $CommandsDir) { Remove-Item $CommandsDir -Force }

# Copy files (Windows doesn't do symlinks easily without admin)
Copy-Item -Recurse (Join-Path $InstallDir "agents") $AgentsDir
Copy-Item -Recurse (Join-Path $InstallDir "commands") $CommandsDir
Write-Host "Copied agents and commands to $ConfigDir"

# Patch opencode.json
$OpenCodeJson = Join-Path $ConfigDir "opencode.json"
if (Test-Path $OpenCodeJson) {
    $config = Get-Content $OpenCodeJson -Raw | ConvertFrom-Json
    $config | Add-Member -NotePropertyName "default_agent" -NotePropertyValue "team-captain" -Force
    $config | ConvertTo-Json -Depth 10 | Set-Content $OpenCodeJson -Encoding UTF8
    Write-Host "Updated opencode.json: default_agent set to team-captain"
} else {
    @{
        '$schema' = "https://opencode.ai/config.json"
        'default_agent' = "team-captain"
    } | ConvertTo-Json | Set-Content $OpenCodeJson -Encoding UTF8
    Write-Host "Created $OpenCodeJson with default_agent: team-captain"
}

Write-Host ""
Write-Host "Installation complete!"
Write-Host ""
Write-Host "Usage: just open any project with OpenCode - the Captain agent is now your default."
Write-Host "Update: cd $InstallDir; git pull; .\scripts\install.ps1"
Write-Host "Uninstall: & `"$InstallDir\scripts\uninstall.ps1`""
