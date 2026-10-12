#!/usr/bin/env bash
set -eu
src="$(cd "$(dirname "$0")/.." && pwd)/s-ui.sh"
tmp="$(mktemp -d)"
[[ "$tmp" == /tmp/* && "$tmp" != / ]] || exit 1
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
touch sample.example.com
CF_Domain=example.com
force_flag=''
capture() { printf '%s\n' "$@"; }
line="$(grep -- '--issue --dns dns_cf' "$src")"
line="${line/\~\/.acme.sh\/acme.sh/capture}"
args="$(eval "$line")"
[[ "$args" == *$'\n*.example.com\n'* ]] || { echo 'Wildcard was expanded by shell'; exit 1; }
echo 'Wildcard domain stays literal: OK'