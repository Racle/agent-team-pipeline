#!/usr/bin/env bash
set -euo pipefail

REPO_URL="https://github.com/Racle/agent-team-pipeline.git"
INSTALL_DIR="${HOME}/.local/share/agent-team-pipeline"

# Detect if running from pipe or from local clone
if [ -t 0 ] && [ -d "$(dirname "$0")/../.git" ]; then
	# Running locally from a cloned repo
	REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
	echo "Running from local clone: $REPO_DIR"
else
	# Running from pipe (curl | bash) or not in a git repo
	echo "Cloning agent-team-pipeline..."
	if [ -d "$INSTALL_DIR" ]; then
		echo "Updating existing clone at $INSTALL_DIR"
		git -C "$INSTALL_DIR" pull --quiet
	else
		git clone --quiet "$REPO_URL" "$INSTALL_DIR"
	fi
	REPO_DIR="$INSTALL_DIR"
fi

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
mkdir -p "$CONFIG_DIR"

TIMESTAMP=$(date +%Y%m%d%H%M%S)

# Back up existing agents/ if it exists and is not a symlink
if [ -d "$CONFIG_DIR/agents" ] && [ ! -L "$CONFIG_DIR/agents" ]; then
	echo "Backing up existing agents/ to agents.bak.$TIMESTAMP"
	mv "$CONFIG_DIR/agents" "$CONFIG_DIR/agents.bak.$TIMESTAMP"
fi

# Back up existing commands/ if it exists and is not a symlink
if [ -d "$CONFIG_DIR/commands" ] && [ ! -L "$CONFIG_DIR/commands" ]; then
	echo "Backing up existing commands/ to commands.bak.$TIMESTAMP"
	mv "$CONFIG_DIR/commands" "$CONFIG_DIR/commands.bak.$TIMESTAMP"
fi

# Remove existing symlinks for re-install
[ -L "$CONFIG_DIR/agents" ] && rm "$CONFIG_DIR/agents"
[ -L "$CONFIG_DIR/commands" ] && rm "$CONFIG_DIR/commands"

# Create symlinks
ln -s "$REPO_DIR/agents" "$CONFIG_DIR/agents"
ln -s "$REPO_DIR/commands" "$CONFIG_DIR/commands"
echo "Symlinked agents and commands to $CONFIG_DIR"

# Patch opencode.json
OPENCODE_JSON="$CONFIG_DIR/opencode.json"
if [ -f "$OPENCODE_JSON" ]; then
	if command -v jq &>/dev/null; then
		tmp=$(mktemp)
		jq '.default_agent = "team-captain"' "$OPENCODE_JSON" >"$tmp" && mv "$tmp" "$OPENCODE_JSON"
		echo "Updated opencode.json: default_agent set to team-captain"
	else
		echo "WARNING: jq not found. Please add \"default_agent\": \"team-captain\" to $OPENCODE_JSON manually."
	fi
else
	cat >"$OPENCODE_JSON" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "default_agent": "team-captain"
}
EOF
	echo "Created $OPENCODE_JSON with default_agent: team-captain"
fi

# Create update script
mkdir -p "$HOME/.local/bin"
cat >"$HOME/.local/bin/agent-team-update" <<EOF
#!/usr/bin/env bash
set -euo pipefail
cd "$INSTALL_DIR" && git pull
echo "agent-team-pipeline updated."
EOF
chmod +x "$HOME/.local/bin/agent-team-update"
echo "Created ~/.local/bin/agent-team-update"

# Check if ~/.local/bin is on PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "$HOME/.local/bin"; then
	echo "WARNING: ~/.local/bin is not on your PATH. Add it to use 'agent-team-update'."
fi

echo ""
echo "Installation complete!"
echo ""
echo "Usage: just open any project with OpenCode — the Captain agent is now your default."
echo "Update: run 'agent-team-update' to pull the latest agent definitions."
echo "Uninstall: run '$REPO_DIR/scripts/uninstall.sh'"
