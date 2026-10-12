#!/usr/bin/env bash
# Entire installer runs in a disposable tree with shell-function service mocks.
set -euo pipefail
src="$(cd "$(dirname "$0")/.." && pwd)/install-macos.sh"
tmp="$(mktemp -d)"
tmp="$(cd "$tmp" && pwd -P)"
[[ -n "$tmp" && "$tmp" != / && ( "$tmp" == /tmp/* || "$tmp" == "${TMPDIR:-/tmp}/"* ) ]] || exit 1
trap 'rm -rf "$tmp"' EXIT
for scenario in upgrade fresh migration bootstrap kickstart no_pid stop_busy rollback_busy; do
  root="$tmp/$scenario"
  mkdir -p "$root/package" "$root/install/db"
  sed -e "s|^PLIST_PATH=.*|PLIST_PATH=\"$root/service.plist\"|" \
      -e '/^if \[ "${EUID:/,/^fi$/d' "$src" > "$root/package/install.sh"
  if [[ "$scenario" != fresh ]]; then
    echo original > "$root/install/db/main.db"
    echo wal > "$root/install/db/main.db-wal"
    echo old-env > "$root/install/s-ui.env"
    echo old-binary > "$root/install/sui"
    echo old-plist > "$root/service.plist"
  fi
  cat > "$root/package/sui" <<'EOF'
#!/usr/bin/env bash
echo "$1" >> "$ROOT/calls"
case "$1" in
  migrate)
    echo migrated > db/main.db
    [[ "$SCENARIO" != migration && "$SCENARIO" != rollback_busy ]] ;;
  setting|admin) [[ "$SCENARIO" == fresh ]] ;;
esac
EOF
  chmod +x "$root/package/sui"
  (
    export ROOT="$root" SCENARIO="$scenario"
    uname() { if [[ "$1" == -s ]]; then echo Darwin; else echo arm64; fi; }
    sleep() { :; }
    kill() { return 0; }
    pgrep() {
      if [[ "$SCENARIO" == stop_busy || ( "$SCENARIO" == rollback_busy && -f "$ROOT/calls" ) ]]; then echo 123; return 0; fi
      return 1
    }
    launchctl() {
      echo "$1" >> "$ROOT/launch-calls"
      case "$1" in
        bootout) rm -f "$ROOT/running" ;;
        bootstrap) [[ "$SCENARIO" != bootstrap ]] || return 1; touch "$ROOT/running" ;;
        kickstart) [[ "$SCENARIO" != kickstart ]] ;;
        print)
          if [[ -f "$ROOT/running" ]]; then
            if [[ "$SCENARIO" != no_pid ]]; then echo 'pid = 123'; fi
          else echo 'Could not find service' >&2; return 113; fi ;;
        *) return 1 ;;
      esac
    }
    export -f uname sleep kill pgrep launchctl
    printf '%s\n2095\n/app/\n2096\n/sub/\nadmin\nsecret\n' "$ROOT/install" | bash "$ROOT/package/install.sh" --lang=en
  ) > "$root/output" 2>&1 && result=0 || result=$?
  case "$scenario" in
    upgrade|fresh) [[ "$result" == 0 ]] ;;
    *) [[ "$result" != 0 ]] ;;
  esac
  case "$scenario" in
    upgrade)
      [[ "$(cat "$root/calls")" == migrate ]]
      [[ "$(cat "$root/install/s-ui.env")" == old-env ]]
      [[ "$(cat "$root"/.s-ui-backup.*/db/main.db-wal)" == wal ]] ;;
    fresh) [[ "$(cat "$root/calls")" == $'migrate\nsetting\nadmin' ]] ;;
    rollback_busy) [[ "$(cat "$root/install/db/main.db")" == migrated ]]; grep -q 'Rollback refused' "$root/output" ;;
    stop_busy) [[ ! -e "$root/calls" ]]; [[ "$(cat "$root/install/db/main.db")" == original ]] ;;
    *) [[ "$(cat "$root/install/db/main.db")" == original ]]; [[ "$(cat "$root/install/sui")" == old-binary ]] ;;
  esac
  echo "$scenario: OK"
done
