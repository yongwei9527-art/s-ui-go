#!/usr/bin/env bash
set -euo pipefail
source /dev/stdin <<< "$(sed -n '/^validate_install_path()/,/^# END TRANSACTION HELPERS/p' "$(dirname "$0")/../install-macos.sh")"
tmp="$(mktemp -d)"
tmp="$(cd "$tmp" && pwd -P)"
[[ -n "$tmp" && "$tmp" != / && ( "$tmp" == /tmp/* || "$tmp" == "${TMPDIR:-/tmp}/"* ) ]] || exit 1
trap 'rm -rf "$tmp"' EXIT
validate_install_path "$tmp/install"
for path in relative / /tmp/ /tmp//bad /tmp/../bad /tmp/./bad '/tmp/a b' '/tmp/a&b'; do
  if validate_install_path "$path"; then echo "Accepted unsafe path: $path"; exit 1; fi
done
mkdir "$tmp/real"
ln -s "$tmp/real" "$tmp/link"
if [[ -L "$tmp/link" ]]; then
  if validate_install_path "$tmp/link/child"; then exit 1; fi
else
  echo 'SKIP: this shell emulates directory symlinks; native macOS symlink verification still required'
fi
INSTALL_DIR="$tmp/real"
PLIST_PATH="$tmp/service.plist"
mkdir "$INSTALL_DIR/db"
mkdir "$tmp/link-target"
ln -s "$tmp/link-target" "$INSTALL_DIR/db/link"
if [[ -L "$INSTALL_DIR/db/link" ]]; then
  if assert_safe_targets; then exit 1; fi
fi
echo 'path validation: OK'
