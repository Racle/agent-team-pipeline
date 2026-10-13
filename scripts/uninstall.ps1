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


# Only remove if it is our junction/link (mirrors uninstall.sh)
foreach ($d in @($AgentsDir, $CommandsDir)) {
    $item = Get-Item $d -Force -ErrorAction SilentlyContinue
    if ($item -and $item.LinkType -and ("$($item.Target)" -like "*agent-team-pipeline*")) {
        $item.Delete()
        Write-Host "Removed $(Split-Path $d -Leaf) link"
    }
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
    $changed = $false
    if ($config.default_agent -in @("team-captain", "team-solo")) {
        $config.PSObject.Properties.Remove("default_agent")
        Write-Host "Removed default_agent from opencode.json"
        $changed = $true
    }
    # Remove the explore model only if it is still the value the installer set
    if ($config.agent.explore.model -eq "github-copilot/claude-haiku-5.5") {
        $config.agent.explore.PSObject.Properties.Remove("model")
        if (-not @($config.agent.explore.PSObject.Properties).Count) { $config.agent.PSObject.Properties.Remove("explore") }
        if (-not @($config.agent.PSObject.Properties).Count) { $config.PSObject.Properties.Remove("agent") }
        Write-Host "Removed agent.explore.model from opencode.json"
        $changed = $true
    }
    if ($changed) { $config | ConvertTo-Json -Depth 10 | Set-Content $OpenCodeJson -Encoding UTF8 }
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
