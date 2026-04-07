#!/usr/bin/env bash
#
# Reset Duo CLI environment for testing
#
# This script removes the installed Duo CLI binary and clears all
# configuration values to allow testing the installation flow from scratch.

set -e

GLAB_BIN="${GLAB_BIN:-./bin/glab}"
DUO_BINARY="$HOME/.config/glab-cli/bin/duo"
CONFIG_FILE="$HOME/.config/glab-cli/config.yml"

echo "🔄 Resetting Duo CLI environment..."
echo

# Remove binary if it exists
if [ -f "$DUO_BINARY" ]; then
    echo "🗑️  Removing binary: $DUO_BINARY"
    rm -f "$DUO_BINARY"
else
    echo "ℹ️  Binary not found: $DUO_BINARY"
fi

# Clear all duo cli config values by directly editing the config file
if [ -f "$CONFIG_FILE" ]; then
    echo "🧹 Clearing configuration values from $CONFIG_FILE..."

    # Create a backup
    cp "$CONFIG_FILE" "${CONFIG_FILE}.backup"

    # Use sed to remove the !!null prefix and set values to empty
    # This handles the weird YAML format that glab creates
    sed -i.tmp \
        -e 's/^duo_cli_auto_run:.*/duo_cli_auto_run:/' \
        -e 's/^duo_cli_auto_download:.*/duo_cli_auto_download:/' \
        -e 's/^duo_cli_binary_path:.*/duo_cli_binary_path:/' \
        -e 's/^duo_cli_binary_version:.*/duo_cli_binary_version:/' \
        -e 's/^duo_cli_binary_checksum:.*/duo_cli_binary_checksum:/' \
        -e 's/^duo_cli_last_update_check:.*/duo_cli_last_update_check:/' \
        "$CONFIG_FILE"

    rm -f "${CONFIG_FILE}.tmp"
    echo "   ✓ Config backup saved to ${CONFIG_FILE}.backup"
else
    echo "⚠️  Config file not found: $CONFIG_FILE"
fi

echo
echo "✅ Environment reset complete!"
echo
echo "📋 Current state:"
echo "   Binary exists: $([ -f "$DUO_BINARY" ] && echo "yes" || echo "no")"
echo "   Config values:"
echo "     - duo_cli_auto_run: '$("$GLAB_BIN" config get duo_cli_auto_run)'"
echo "     - duo_cli_auto_download: '$("$GLAB_BIN" config get duo_cli_auto_download)'"
echo "     - duo_cli_binary_version: '$("$GLAB_BIN" config get duo_cli_binary_version)'"
echo
echo "🚀 Ready to test! Run: $GLAB_BIN duo cli"
