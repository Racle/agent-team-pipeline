#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
INSTALL_DIR="${HOME}/.local/share/agent-team-pipeline"

# Remove symlinks only if they point to our repo
if [ -L "$CONFIG_DIR/agents" ]; then
	target=$(readlink "$CONFIG_DIR/agents")
	if [[ "$target" == *"agent-team-pipeline"* ]]; then
		rm "$CONFIG_DIR/agents"
		echo "Removed agents symlink"
	fi
fi

if [ -L "$CONFIG_DIR/commands" ]; then
	target=$(readlink "$CONFIG_DIR/commands")
	if [[ "$target" == *"agent-team-pipeline"* ]]; then
		rm "$CONFIG_DIR/commands"
		echo "Removed commands symlink"
	fi
fi

# Restore backups if they exist
latest_agents_bak=$(ls -d "$CONFIG_DIR"/agents.bak.* 2>/dev/null | sort | tail -1 || true)
if [ -n "$latest_agents_bak" ] && [ ! -e "$CONFIG_DIR/agents" ]; then
	mv "$latest_agents_bak" "$CONFIG_DIR/agents"
	echo "Restored agents from backup"
fi

latest_commands_bak=$(ls -d "$CONFIG_DIR"/commands.bak.* 2>/dev/null | sort | tail -1 || true)
if [ -n "$latest_commands_bak" ] && [ ! -e "$CONFIG_DIR/commands" ]; then
	mv "$latest_commands_bak" "$CONFIG_DIR/commands"
	echo "Restored commands from backup"
fi

# Remove update script
if [ -f "$HOME/.local/bin/agent-team-update" ]; then
	rm "$HOME/.local/bin/agent-team-update"
	echo "Removed ~/.local/bin/agent-team-update"
fi

# Clean up default_agent from opencode.json
OPENCODE_JSON="${CONFIG_DIR}/opencode.json"
if [ -f "$OPENCODE_JSON" ] && command -v jq &>/dev/null; then
    if jq -e '.default_agent == "team-captain"' "$OPENCODE_JSON" &>/dev/null; then
        tmp=$(mktemp)
        jq 'del(.default_agent)' "$OPENCODE_JSON" > "$tmp" && mv "$tmp" "$OPENCODE_JSON"
        echo "Removed default_agent from opencode.json"
    fi
elif [ -f "$OPENCODE_JSON" ]; then
    echo "WARNING: jq not found. Please remove \"default_agent\": \"team-captain\" from $OPENCODE_JSON manually."
fi

# Optionally remove cloned repo
if [ -d "$INSTALL_DIR" ]; then
	echo ""
	read -rp "Remove cloned repo at $INSTALL_DIR? [y/N] " answer
	if [[ "$answer" =~ ^[Yy]$ ]]; then
		rm -rf "$INSTALL_DIR"
		echo "Removed $INSTALL_DIR"
	fi
fi

echo ""
echo "Uninstall complete."
