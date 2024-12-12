#!/bin/bash

# === Configurable Variables ===
MAIN_MODULE="./main-module"  # Path to the main module
VENDOR_PLUGINS="./vendor-plugins"  # Path to vendor plugins
THIRD_PARTY_PLUGINS="./third-party-plugins"  # Path to third-party plugins
GO_VERSION="1.21"  # Default Go version (override with an argument)

# Override GO_VERSION if passed as the first argument
if [ -n "$1" ]; then
    GO_VERSION="$1"
fi

# Function to update Go version in a go.mod file
update_go_version() {
    local go_mod_file="$1"
    local version="$2"

    if [ -f "$go_mod_file" ]; then
        echo "Updating $go_mod_file to use Go $version"
        sed -i.bak -E "s/^\s*go\s+[0-9]+\.[0-9]+/go $version/" "$go_mod_file" && rm -f "$go_mod_file.bak"
    else
        echo "go.mod not found: $go_mod_file"
    fi
}

# Collect all go.mod files
collect_go_mod_files() {
    find "$1" -name "go.mod"
}

# Main logic
sync_go_mod_versions() {
    local base_dirs=("$MAIN_MODULE" "$VENDOR_PLUGINS" "$THIRD_PARTY_PLUGINS")

    for dir in "${base_dirs[@]}"; do
        if [ -d "$dir" ]; then
            echo "Scanning directory: $dir"
            go_mod_files=$(collect_go_mod_files "$dir")

            for go_mod in $go_mod_files; do
                update_go_version "$go_mod" "$GO_VERSION"
            done
        else
            echo "Directory not found: $dir"
        fi
    done
}

# Display current versions for verification
check_go_mod_versions() {
    local base_dirs=("$MAIN_MODULE" "$VENDOR_PLUGINS" "$THIRD_PARTY_PLUGINS")

    echo "Current Go versions in go.mod files:"
    for dir in "${base_dirs[@]}"; do
        if [ -d "$dir" ]; then
            go_mod_files=$(collect_go_mod_files "$dir")

            for go_mod in $go_mod_files; do
                current_version=$(grep -E "^\s*go\s+[0-9]+\.[0-9]+" "$go_mod" | awk '{print $2}')
                echo "$go_mod: $current_version"
            done
        fi
    done
}

# Script entry point
case "$2" in
    sync)
        sync_go_mod_versions
        ;;
    check)
        check_go_mod_versions
        ;;
    *)
        echo "Usage: $0 [GO_VERSION] {sync|check}"
        echo "  GO_VERSION: Optional Go version (default: $GO_VERSION)"
        echo "  sync: Update all go.mod files to the specified Go version"
        echo "  check: Display current Go versions in all go.mod files"
        exit 1
        ;;
esac