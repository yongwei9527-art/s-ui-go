#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")/.."
block="$(sed -n '/^# Never classify unknown/,/^if \[\[ "\$STATE_VALUE" == "installing"/p' install-linux.sh | sed '$d')"
[[ -n "$block" ]]
tmp="$(mktemp -d "$PWD/.verify-build/db-guard.XXXXXX")"
[[ "$tmp" == "$PWD"/.verify-build/db-guard.* ]] || exit 1
trap 'rm -f -- "$tmp/db/legacy.db"; rmdir -- "$tmp/db" "$tmp"' EXIT
mkdir "$tmp/db"
INSTALL_DIR="$tmp"
DB_EXISTS=0
die() { echo "$*" >&2; exit 1; }
(eval "$block")
printf '%s\n' 'keep-me' > "$tmp/db/legacy.db"
if (eval "$block"); then echo 'Unknown legacy data accepted'; exit 1; fi
[[ "$(cat "$tmp/db/legacy.db")" == keep-me ]]
echo 'Unknown nonempty database rejected without modifying data: OK'
