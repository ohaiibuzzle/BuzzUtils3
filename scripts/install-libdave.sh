#!/bin/sh
# Installs libdave (voice encryption) into ~/.local, at the version the godave in
# go.mod is built for, using godave's own install script. Needs `go mod download`
# first. Afterwards, builds need PKG_CONFIG_PATH="$HOME/.local/lib/pkgconfig".
set -eu

GODAVE=$(go list -m -f '{{.Dir}}' github.com/disgoorg/godave)
LIBDAVE=$(go list -m -f '{{.Dir}}' github.com/disgoorg/godave/libdave)
if [ -z "$GODAVE" ] || [ -z "$LIBDAVE" ]; then
    echo "godave isn't downloaded; run 'go mod download' first" >&2
    exit 1
fi

# The script reads $SHELL under set -u, which isn't set in Docker or with dash
SHELL=${SHELL:-/bin/sh} NON_INTERACTIVE=1 sh "$GODAVE/scripts/libdave_install.sh" "$(cat "$LIBDAVE/release.txt")"
