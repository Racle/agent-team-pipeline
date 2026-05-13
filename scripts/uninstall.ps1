$ErrorActionPreference = "Stop"

$InstallDir = Join-Path $env:LOCALAPPDATA "agent-team-pipeline"

# Determine config directory
if ($env:HOME -and (Test-Path (Join-Path $env:HOME ".config"))) {
    $ConfigDir = Join-Path $env:HOME ".config/opencode"
} else {
    $ConfigDir = Join-Path $env:APPDATA "opencode"
}

$AgentsDir = Join-Path $ConfigDir "agents"
$CommandsDir = Join-Path $ConfigDir "commands"

# Remove agents and commands directories
if (Test-Path $AgentsDir) {
    Remove-Item $AgentsDir -Recurse -Force
    Write-Host "Removed agents directory"
}

if (Test-Path $CommandsDir) {
    Remove-Item $CommandsDir -Recurse -Force
    Write-Host "Removed commands directory"
}

# Restore backups if they exist
$LatestAgentsBak = Get-ChildItem $ConfigDir -Directory -Filter "agents.bak.*" -ErrorAction SilentlyContinue | Sort-Object Name | Select-Object -Last 1
if ($LatestAgentsBak -and -not (Test-Path $AgentsDir)) {
    Rename-Item $LatestAgentsBak.FullName -NewName (Split-Path $AgentsDir -Leaf)
    Write-Host "Restored agents from backup"
}

$LatestCommandsBak = Get-ChildItem $ConfigDir -Directory -Filter "commands.bak.*" -ErrorAction SilentlyContinue | Sort-Object Name | Select-Object -Last 1
if ($LatestCommandsBak -and -not (Test-Path $CommandsDir)) {
    Rename-Item $LatestCommandsBak.FullName -NewName (Split-Path $CommandsDir -Leaf)
    Write-Host "Restored commands from backup"
}

# Clean up default_agent from opencode.json
$OpenCodeJson = Join-Path $ConfigDir "opencode.json"
if (Test-Path $OpenCodeJson) {
    $config = Get-Content $OpenCodeJson -Raw | ConvertFrom-Json
    if ($config.default_agent -eq "team-captain") {
        $config.PSObject.Properties.Remove("default_agent")
        $config | ConvertTo-Json -Depth 10 | Set-Content $OpenCodeJson -Encoding UTF8
        Write-Host "Removed default_agent from opencode.json"
    }
}

# Optionally remove cloned repo
if (Test-Path $InstallDir) {
    $answer = Read-Host "Remove cloned repo at $InstallDir? [y/N]"
    if ($answer -eq "y" -or $answer -eq "Y") {
        Remove-Item $InstallDir -Recurse -Force
        Write-Host "Removed $InstallDir"
    }
}

Write-Host ""
Write-Host "Uninstall complete."
