#!/usr/bin/env bash
# Full entrypoint tests. Only disposable copies and mocked macOS commands run.
set -euo pipefail
src="$(cd "$(dirname "$0")/.." && pwd -P)/uninstall-macos.sh"
temp_parent="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
tmp="$(mktemp -d "$temp_parent/s-ui-uninstall-test.XXXXXXXX")"
tmp="$(cd "$tmp" && pwd -P)"
# Every deletion below must stay below this verified absolute, dedicated tree.
[[ "$temp_parent" == /* && "$tmp" == "$temp_parent"/s-ui-uninstall-test.* && "$tmp" != "$temp_parent" && ! -L "$tmp" ]] || exit 1
TEST_TMP="$tmp"
export TEST_TMP
safe_test_target() {
  local path="$1"
  [[ "$path" == "$TEST_TMP" || "$path" == "$TEST_TMP"/* ]] || return 1
  [[ "$path" != *'/../'* && "$path" != */.. && "$path" != *'/./'* && "$path" != *'//'* ]]
}
cleanup() {
  # realpath resolves symlinks and '..' before any recursive cleanup.
  local resolved
  resolved="$(cd "$TEST_TMP" && pwd -P)" || return 1
  [[ "$resolved" == "$TEST_TMP" && "$resolved" == "$temp_parent"/s-ui-uninstall-test.* ]] || return 1
  safe_test_target "$resolved" || return 1
  command rm -rf -- "$resolved"
}
trap cleanup EXIT

# Override every OS/service/process command, including rm with a containment guard.
uname() { [[ "$1" == -s ]] || return 2; if [[ "$SCENARIO" == non_macos ]]; then echo Linux; else echo Darwin; fi; }
sleep() { :; }
rm() {
  local arg
  for arg in "$@"; do
    case "$arg" in -f|-rf|--) ;; *) safe_test_target "$arg" || { echo "UNSAFE mock rm: $arg" >&2; return 99; } ;; esac
  done
  printf '%s\n' "$*" >> "$ROOT/rm-calls"
  command rm "$@"
}
launchctl() {
  printf '%s\n' "$*" >> "$ROOT/launch-calls"
  case "$1" in
    print)
      [[ "$2" == system/com.s-ui ]] || return 99
      if [[ "$SCENARIO" == query_error ]]; then echo 'permission denied' >&2; return 5; fi
      if [[ "$SCENARIO" == query_error_after && ! -f "$ROOT/running" ]]; then echo 'I/O error' >&2; return 5; fi
      if [[ ! -f "$ROOT/running" ]]; then echo 'Could not find service "com.s-ui" in domain for system' >&2; return 113; fi
      local dir="$ROOT/custom-install" plist="$ROOT/service.plist"
      [[ "$SCENARIO" != loaded_wrong_dir ]] || dir="$ROOT/other-install"
      [[ "$SCENARIO" != loaded_wrong_plist ]] || plist="$ROOT/other.plist"
      printf 'system/com.s-ui = {\n path = %s\n program = %s/sui\n working directory = %s\n' "$plist" "$dir" "$dir"
      case "$SCENARIO" in
        bad_pid) echo ' pid = invalid' ;;
        no_pid) : ;;
        *) echo ' pid = 12345' ;;
      esac
      [[ "$SCENARIO" != loaded_duplicate ]] || printf ' program = %s/sui\n' "$dir"
      echo '}' ;;
    bootout)
      [[ "$2" == system/com.s-ui ]] || return 99
      case "$SCENARIO" in
        stop_failure) echo 'bootout failed' >&2; return 5 ;;
        stop_failure_absent) safe_test_target "$ROOT/running" || return 99; command rm -f -- "$ROOT/running"; return 5 ;;
        still_loaded) return 0 ;;
        swap_plist) printf 'DIR=%s\nLABEL=com.s-ui\n' "$ROOT/other-install" > "$ROOT/service.plist" ;;
        late_symlink) ln -s "$ROOT/outside/sentinel" "$ROOT/custom-install/cert/link" ;;
      esac
      safe_test_target "$ROOT/running" || return 99
      command rm -f -- "$ROOT/running" ;;
    *) echo 'UNEXPECTED launchctl action' >&2; return 99 ;;
  esac
}
pgrep() {
  if [[ "$1" == -x ]]; then
    [[ "$2" == sui ]] || return 99
    if [[ "$SCENARIO" == relative_process ]]; then echo 54321; return 0; fi
    return 1
  fi
  local dir="$ROOT/custom-install" expected
  expected="(^|[[:space:]])${dir//./[.]}/sui([[:space:]]|$)"
  [[ "$1" == -f && "$2" == "$expected" ]] || return 99
  echo "$*" >> "$ROOT/pgrep-calls"
  case "$SCENARIO" in
    busy_process) echo 12345; return 0 ;;
    pgrep_error) echo 'pgrep failed' >&2; return 2 ;;
    pgrep_dirty_absent) echo 'unexpected output'; return 1 ;;
  esac
  return 1
}
ps() {
  [[ "$*" == '-p 12345 -o pid=' ]] || return 99
  echo "$*" >> "$ROOT/ps-calls"
  case "$SCENARIO" in
    pid_alive) echo 12345; return 0 ;;
    ps_error) echo 'ps failed' >&2; return 2 ;;
  esac
  return 1
}
export -f uname sleep rm safe_test_target launchctl pgrep ps

skipped=0
native_symlinks=1
mkdir -p "$tmp/probe-dir"
printf 'probe\n' > "$tmp/probe-file"
ln -s "$tmp/probe-dir" "$tmp/probe-dir-link"
ln -s "$tmp/probe-file" "$tmp/probe-file-link"
[[ -L "$tmp/probe-dir-link" && -L "$tmp/probe-file-link" ]] || native_symlinks=0
passed=0
for scenario in keep_enter keep_eof keep_yes purge_prompt purge_flag adjacent adjacent_relative no_config already_absent no_pid \
    no_discovery unknown_arg empty_arg duplicate_arg missing_arg invalid_consent truncated_consent non_macos \
    relative_path root_path critical_path dot_path dotdot_path double_slash trailing_slash space_path \
    env_command env_unknown env_duplicate env_wrong_dir env_wrong_service missing_binary unknown_file \
    wrong_plist_dir wrong_plist_label wrong_plist_work extra_argument override_program malformed_plist \
    stop_failure stop_failure_absent still_loaded busy_process relative_process pid_alive pgrep_error pgrep_dirty_absent ps_error \
    query_error query_error_after loaded_wrong_dir loaded_wrong_plist loaded_duplicate bad_pid swap_plist late_symlink \
    symlink_dir symlink_ancestor symlink_binary symlink_config symlink_plist nested_symlink; do
  case "$scenario" in
    symlink_*|nested_symlink|late_symlink)
      if [[ "$native_symlinks" != 1 ]]; then
        printf 'SKIP: %s (shell emulates symlinks; requires native macOS)\n' "$scenario"
        skipped=$((skipped+1))
        continue
      fi ;;
  esac
  root="$tmp/$scenario"
  mkdir -p "$root/package" "$root/custom-install/db" "$root/custom-install/logs" "$root/custom-install/cert" "$root/outside"
  printf 'original-db\n' > "$root/custom-install/db/main.db"
  printf 'wal\n' > "$root/custom-install/db/main.db-wal"
  printf 'log\n' > "$root/custom-install/logs/stdout.log"
  printf 'key\n' > "$root/custom-install/cert/key.pem"
  printf 'outside-untouched\n' > "$root/outside/sentinel"
  printf '#!/usr/bin/env bash\nexit 99\n' > "$root/custom-install/sui"
  chmod +x "$root/custom-install/sui"
  printf 'INSTALL_DIR=%s\nSERVICE_NAME=s-ui\nPANEL_PORT=2095\nPANEL_PATH=/app/\nSUB_PORT=2096\nSUB_PATH=/sub/\n' "$root/custom-install" > "$root/custom-install/s-ui.env"
  printf 'readme\n' > "$root/custom-install/README.md"
  printf 'guide\n' > "$root/custom-install/PROTOCOL_GUIDE.md"
  printf 'DIR=%s\nLABEL=com.s-ui\n' "$root/custom-install" > "$root/service.plist"
  touch "$root/running"
  # PlistBuddy is absolute in production. Substitute ONLY in the test copy.
  cat > "$root/plistbuddy" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == -c && "$3" == "$ROOT/service.plist" ]] || exit 99
dir=$(sed -n 's/^DIR=//p' "$3")
label=$(sed -n 's/^LABEL=//p' "$3")
[[ "$SCENARIO" != malformed_plist ]] || exit 1
case "$2" in
  'Print :Label') echo "$label" ;;
  'Print :ProgramArguments:0') echo "$dir/sui" ;;
  'Print :WorkingDirectory')
    if [[ "$SCENARIO" == wrong_plist_work ]]; then echo "$ROOT/outside"; else echo "$dir"; fi ;;
  'Print :ProgramArguments:1') [[ "$SCENARIO" == extra_argument ]] || exit 1; echo --other ;;
  'Print :Program') [[ "$SCENARIO" == override_program ]] || exit 1; echo /other/program ;;
  *) exit 99 ;;
esac
EOF
  chmod +x "$root/plistbuddy"
  # Literal EUID text is a sed pattern, not a variable expansion.
  # shellcheck disable=SC2016
  sed -e "s|^PLIST_PATH=.*|PLIST_PATH=\"$root/service.plist\"|" \
      -e "s|^PLIST_BUDDY=.*|PLIST_BUDDY=\"$root/plistbuddy\"|" \
      -e '/^if \[\[ "${EUID:/d' "$src" > "$root/package/uninstall.sh"
  args=(--install-dir "$root/custom-install")
  input=$'\n'; expected=fail; script="$root/package/uninstall.sh"
  case "$scenario" in
    keep_enter|keep_eof|keep_yes|purge_prompt|purge_flag|adjacent|adjacent_relative|no_config|already_absent|no_pid) expected=success ;;
  esac
  case "$scenario" in
    keep_eof) input='' ;;
    keep_yes) input=$'y\n' ;;
    purge_prompt) input=$'n\n' ;;
    purge_flag) args+=(--remove-data); input='' ;;
    adjacent|adjacent_relative)
      cp "$script" "$root/custom-install/uninstall-macos.sh"
      script="$root/custom-install/uninstall-macos.sh"; args=() ;;
    no_config) safe_test_target "$root/custom-install/s-ui.env"; command rm -f -- "$root/custom-install/s-ui.env" ;;
    already_absent) safe_test_target "$root/running"; command rm -f -- "$root/running" ;;
    no_discovery) args=() ;;
    unknown_arg) args+=(--anything) ;;
    empty_arg) args=(--install-dir '') ;;
    duplicate_arg) args+=(--install-dir "$root/custom-install") ;;
    missing_arg) args=(--install-dir) ;;
    invalid_consent) input=$'typo\n' ;;
    truncated_consent) input='n' ;;
    relative_path) args=(--install-dir custom-install) ;;
    root_path) args=(--install-dir /) ;;
    critical_path) args=(--install-dir /usr/local) ;;
    dot_path) args=(--install-dir "$root/./custom-install") ;;
    dotdot_path) args=(--install-dir "$root/outside/../custom-install") ;;
    double_slash) args=(--install-dir "$root//custom-install") ;;
    trailing_slash) args=(--install-dir "$root/custom-install/") ;;
    space_path) args=(--install-dir "$root/custom install") ;;
    env_command) printf 'touch "%s/executed"\n' "$root" >> "$root/custom-install/s-ui.env" ;;
    env_unknown) echo 'PLIST_PATH=/another/service' >> "$root/custom-install/s-ui.env" ;;
    env_duplicate) printf 'INSTALL_DIR=%s\n' "$root/custom-install" >> "$root/custom-install/s-ui.env" ;;
    env_wrong_dir) printf 'INSTALL_DIR=%s\nSERVICE_NAME=s-ui\n' "$root/outside" > "$root/custom-install/s-ui.env" ;;
    env_wrong_service) echo 'SERVICE_NAME=other' > "$root/custom-install/s-ui.env" ;;
    missing_binary) safe_test_target "$root/custom-install/sui"; command rm -f -- "$root/custom-install/sui" ;;
    unknown_file) echo unrelated > "$root/custom-install/unrelated" ;;
    wrong_plist_dir) printf 'DIR=%s\nLABEL=com.s-ui\n' "$root/outside" > "$root/service.plist" ;;
    wrong_plist_label) printf 'DIR=%s\nLABEL=other\n' "$root/custom-install" > "$root/service.plist" ;;
    symlink_dir) ln -s "$root/custom-install" "$root/link"; args=(--install-dir "$root/link") ;;
    symlink_ancestor) ln -s "$root" "$root/ancestor"; args=(--install-dir "$root/ancestor/custom-install") ;;
    symlink_binary) safe_test_target "$root/custom-install/sui"; command rm -f -- "$root/custom-install/sui"; ln -s "$root/outside/sentinel" "$root/custom-install/sui" ;;
    symlink_config) mv "$root/custom-install/s-ui.env" "$root/outside/env"; ln -s "$root/outside/env" "$root/custom-install/s-ui.env" ;;
    symlink_plist) mv "$root/service.plist" "$root/outside/plist"; ln -s "$root/outside/plist" "$root/service.plist" ;;
    nested_symlink) ln -s "$root/outside" "$root/custom-install/db/link" ;;
  esac
  before=$(find "$root/custom-install" -type f -exec cksum {} \; | sort)
  (
    export ROOT="$root" SCENARIO="$scenario"
    if [[ "$scenario" == adjacent_relative ]]; then
      cd "$root/custom-install"; script=./uninstall-macos.sh
    fi
    printf '%s' "$input" | bash "$script" "${args[@]}"
  ) > "$root/output" 2>&1 && result=0 || result=$?
  if [[ "$expected" == success ]]; then
    [[ "$result" == 0 && ! -e "$root/service.plist" && ! -e "$root/custom-install/sui" ]] || { cat "$root/output"; echo "FAIL: $scenario"; exit 1; }
    case "$scenario" in
      purge_prompt|purge_flag) [[ ! -e "$root/custom-install" ]] ;;
      *)
        [[ "$(cat "$root/custom-install/db/main.db")" == original-db ]]
        [[ "$(cat "$root/custom-install/db/main.db-wal")" == wal ]]
        [[ "$(cat "$root/custom-install/logs/stdout.log")" == log ]]
        [[ "$(cat "$root/custom-install/cert/key.pem")" == key ]]
        [[ ! -e "$root/custom-install/s-ui.env" && ! -e "$root/custom-install/README.md" && ! -e "$root/custom-install/PROTOCOL_GUIDE.md" ]] ;;
    esac
  else
    [[ "$result" != 0 && ! -e "$root/rm-calls" ]] || { cat "$root/output"; echo "FAIL: $scenario"; exit 1; }
    after=$(find "$root/custom-install" -type f -exec cksum {} \; | sort)
    [[ "$before" == "$after" && -e "$root/service.plist" ]] || { echo "FAIL preservation: $scenario"; exit 1; }
    case "$scenario" in
      stop_*|still_loaded|busy_process|relative_process|pid_alive|pgrep_*|ps_error|query_error_after|swap_plist|late_symlink) ;;
      *)
        if [[ -e "$root/launch-calls" ]]; then
          ! grep -q '^bootout ' "$root/launch-calls" || { echo "Wrong service stopped: $scenario"; exit 1; }
        fi ;;
    esac
  fi
  [[ ! -e "$root/executed" && "$(cat "$root/outside/sentinel")" == outside-untouched ]]
  printf '%s: OK\n' "$scenario"
  passed=$((passed + 1))
done
printf 'macOS uninstall mock tests: %s passed, %s skipped (native macOS not tested)\n' "$passed" "$skipped"
