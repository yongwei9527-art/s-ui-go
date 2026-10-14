#!/usr/bin/env bash
set -euo pipefail

PLIST_PATH="/Library/LaunchDaemons/com.s-ui.plist"
PLIST_BUDDY="/usr/libexec/PlistBuddy"
INSTALL_DIR=""
REMOVE_DATA=0

fail() { printf '错误：%s\n' "$*" >&2; exit 1; }
usage() {
  printf '用法：sudo %s [--install-dir /绝对/专用目录] [--remove-data]\n' "$0"
  printf '未指定目录时仅读取脚本相邻的 s-ui.env；默认保留数据库、日志和证书。\n'
}
while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      [[ $# -ge 2 && -z "$INSTALL_DIR" ]] || fail '缺少或重复安装目录'
      INSTALL_DIR="$2"; [[ -n "$INSTALL_DIR" ]] || fail '安装目录不能为空'; shift ;;
    --install-dir=*)
      [[ -z "$INSTALL_DIR" ]] || fail '重复安装目录'
      INSTALL_DIR="${1#*=}"
      [[ -n "$INSTALL_DIR" ]] || fail '安装目录不能为空' ;;
    --remove-data)
      [[ "$REMOVE_DATA" == 0 ]] || fail '重复删除数据参数'
      REMOVE_DATA=1 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "未知参数：$1" ;;
  esac
  shift
done

# Same lexical/component checks as install-macos.sh, with extra critical roots.
validate_install_path() {
  local path="$1" part current="" rest
  case "$path" in
    /|/usr|/usr/local|/usr/bin|/usr/sbin|/usr/local/bin|/usr/local/lib|/usr/local/share|/Library|/Library/LaunchDaemons|/System|/Applications|/etc|/var|/tmp|/private|/private/tmp|/private/var|/private/etc|/opt|/Users|/bin|/sbin|/dev|/Volumes|/home) return 1 ;;
  esac
  [[ "$path" == /*/* && "$path" != */ && "$path" != *//* ]] || return 1
  [[ "$path" != *[!a-zA-Z0-9_./-]* ]] || return 1
  rest="${path#/}"
  while [[ -n "$rest" ]]; do
    part="${rest%%/*}"
    [[ "$part" != . && "$part" != .. && -n "$part" ]] || return 1
    current="$current/$part"
    [[ ! -L "$current" ]] || return 1
    if [[ "$rest" == */* ]]; then rest="${rest#*/}"; else rest=""; fi
  done
  [[ -d "$path" ]]
}

# Never source/eval the installer-generated env. Only identity fields are used.
read_config_identity() {
  local file="$1" line key value dir_seen=0 service_seen=0
  CONFIG_INSTALL_DIR=""
  [[ -f "$file" && ! -L "$file" ]] || return 1
  while IFS= read -r line || [[ -n "$line" ]]; do
    case "$line" in ''|'#'*) continue ;; esac
    [[ "$line" == *=* ]] || return 1
    key="${line%%=*}"; value="${line#*=}"
    case "$key" in
      INSTALL_DIR)
        [[ "$dir_seen" == 0 ]] || return 1
        dir_seen=1; CONFIG_INSTALL_DIR="$value" ;;
      SERVICE_NAME)
        [[ "$service_seen" == 0 && "$value" == s-ui ]] || return 1
        service_seen=1 ;;
      PANEL_PORT|SUB_PORT) [[ "$value" =~ ^[0-9]+$ ]] || return 1 ;;
      PANEL_PATH|SUB_PATH) [[ "$value" == /* ]] || return 1 ;;
      *) return 1 ;;
    esac
  done < "$file"
  [[ "$dir_seen" == 1 && "$service_seen" == 1 ]]
}

assert_safe_targets() {
  local links target name
  validate_install_path "$INSTALL_DIR" || return 1
  # Markers and a known installation layout distinguish a dedicated directory.
  [[ -f "$INSTALL_DIR/sui" && -x "$INSTALL_DIR/sui" ]] || return 1
  links=$(find "$INSTALL_DIR" -type l -print) || return 1
  [[ -z "$links" ]] || return 1
  for target in "$INSTALL_DIR"/* "$INSTALL_DIR"/.[!.]* "$INSTALL_DIR"/..?*; do
    [[ -e "$target" || -L "$target" ]] || continue
    name="${target##*/}"
    case "$name" in
      db|logs|cert) [[ -d "$target" ]] || return 1 ;;
      sui|s-ui.env|README.md|PROTOCOL_GUIDE.md|uninstall-macos.sh) [[ -f "$target" ]] || return 1 ;;
      *) return 1 ;;
    esac
  done
  if [[ -e "$INSTALL_DIR/s-ui.env" ]]; then
    read_config_identity "$INSTALL_DIR/s-ui.env" || return 1
    [[ "$CONFIG_INSTALL_DIR" == "$INSTALL_DIR" ]] || return 1
  fi
  # Fixed launchd path; config/environment cannot redirect it.
  [[ ! -L /Library && ! -L /Library/LaunchDaemons && ! -L "$PLIST_PATH" && -f "$PLIST_PATH" ]]
}

plist_matches() {
  local value
  value=$("$PLIST_BUDDY" -c 'Print :Label' "$PLIST_PATH") || return 1
  [[ "$value" == com.s-ui ]] || return 1
  value=$("$PLIST_BUDDY" -c 'Print :ProgramArguments:0' "$PLIST_PATH") || return 1
  [[ "$value" == "$INSTALL_DIR/sui" ]] || return 1
  value=$("$PLIST_BUDDY" -c 'Print :WorkingDirectory' "$PLIST_PATH") || return 1
  [[ "$value" == "$INSTALL_DIR" ]] || return 1
  # The installer generates one argument and no overriding Program key.
  if "$PLIST_BUDDY" -c 'Print :ProgramArguments:1' "$PLIST_PATH" >/dev/null 2>&1; then return 1; fi
  if "$PLIST_BUDDY" -c 'Print :Program' "$PLIST_PATH" >/dev/null 2>&1; then return 1; fi
}

query_service() {
  local rc
  if SERVICE_STATE=$(launchctl print system/com.s-ui 2>&1); then
    SERVICE_LOADED=1
  else
    rc=$?
    [[ "$rc" != 0 && ( "$SERVICE_STATE" == *'Could not find service'* || "$SERVICE_STATE" == *'Could not find specified service'* ) ]] || return 1
    SERVICE_LOADED=0
  fi
}

loaded_service_matches() {
  local program working path
  program=$(printf '%s\n' "$SERVICE_STATE" | awk '/^[[:space:]]*program = / {sub(/^[[:space:]]*program = /, ""); print}')
  working=$(printf '%s\n' "$SERVICE_STATE" | awk '/^[[:space:]]*working directory = / {sub(/^[[:space:]]*working directory = /, ""); print}')
  path=$(printf '%s\n' "$SERVICE_STATE" | awk '/^[[:space:]]*path = / {sub(/^[[:space:]]*path = /, ""); print}')
  [[ "$program" == "$INSTALL_DIR/sui" && "$working" == "$INSTALL_DIR" && "$path" == "$PLIST_PATH" ]] || return 1
  STOP_PID=$(printf '%s\n' "$SERVICE_STATE" | awk '$1 == "pid" && $2 == "=" {print $3}')
  [[ -z "$STOP_PID" || "$STOP_PID" =~ ^[1-9][0-9]*$ ]]
}

service_absent() {
  local output rc pattern
  query_service && [[ "$SERVICE_LOADED" == 0 ]] || return 1
  # Escape dots in the validated path; match executable tokens, not other dirs.
  pattern="${INSTALL_DIR//./[.]}"
  if output=$(pgrep -f "(^|[[:space:]])$pattern/sui([[:space:]]|$)" 2>&1); then return 1; else rc=$?; fi
  [[ "$rc" == 1 && -z "$output" ]] || return 1
  # A manual ./sui invocation has no absolute path in argv. Match the executable
  # name too; conservatively refuse deletion if any such process is still alive.
  if output=$(pgrep -x sui 2>&1); then return 1; else rc=$?; fi
  [[ "$rc" == 1 && -z "$output" ]] || return 1
  if [[ -n "$STOP_PID" ]]; then
    if output=$(ps -p "$STOP_PID" -o pid= 2>&1); then return 1; else rc=$?; fi
    [[ "$rc" == 1 && -z "$output" ]] || return 1
  fi
}

stop_service() {
  local attempt
  query_service || return 1
  STOP_PID=""
  if [[ "$SERVICE_LOADED" == 1 ]]; then
    loaded_service_matches || return 1
    # A failed bootout is NOT success, even if a later query looks absent.
    launchctl bootout system/com.s-ui || return 1
  fi
  for attempt in 1 2 3 4 5; do
    if service_absent; then return 0; fi
    if [[ "$attempt" -lt 5 ]]; then sleep 1; fi
  done
  return 1
}
# END UNINSTALL HELPERS

[[ "$(uname -s)" == Darwin ]] || fail '仅支持 macOS'
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then fail '请使用 sudo 运行卸载程序'; fi

if [[ -z "$INSTALL_DIR" ]]; then
  # No default-directory fallback: package copies are not installed instances.
  SCRIPT_PARENT="$(dirname "${BASH_SOURCE[0]}")"
  if [[ "$SCRIPT_PARENT" == . ]]; then
    SCRIPT_PARENT="$PWD"
  elif [[ "$SCRIPT_PARENT" != /* ]]; then
    SCRIPT_PARENT="$PWD/${SCRIPT_PARENT#./}"
  fi
  validate_install_path "$SCRIPT_PARENT" || fail '无法安全发现安装目录，请指定 --install-dir'
  read_config_identity "$SCRIPT_PARENT/s-ui.env" || fail '缺少或无效的相邻已安装配置，请指定 --install-dir'
  [[ "$CONFIG_INSTALL_DIR" == "$SCRIPT_PARENT" ]] || fail '相邻配置与脚本目录不匹配'
  INSTALL_DIR="$CONFIG_INSTALL_DIR"
fi
assert_safe_targets || fail '安装目录不是明确的专用目录，或包含未知文件、无效标记或 symlink'
plist_matches || fail 'launchd 配置不匹配所选安装目录'

# Validate consent before stopping the service. EOF/Enter keep data; typos abort.
if [[ "$REMOVE_DATA" == 0 ]]; then
  printf '是否保留数据库、日志和证书等数据文件？[Y/n]：'
  KEEP_DATA=""
  read -r KEEP_DATA || { [[ -z "$KEEP_DATA" ]] || fail '未完整读取确认输入'; }
  case "$KEEP_DATA" in
    ''|Y|y) ;;
    N|n) REMOVE_DATA=1 ;;
    *) fail '未知确认输入；仅接受 Y/y/N/n 或回车' ;;
  esac
fi
printf '正在确认并停止 launchd 服务...\n'
stop_service || fail '停止失败或无法确认服务及进程已停止；未删除任何文件'
# Recheck identity, filesystem and stopped state immediately before deletion.
if ! assert_safe_targets || ! plist_matches || ! service_absent; then
  fail '删除前安全复核失败；未删除任何文件'
fi
rm -f -- "$PLIST_PATH"
if [[ "$REMOVE_DATA" == 1 ]]; then
  rm -rf -- "$INSTALL_DIR"
else
  rm -f -- "$INSTALL_DIR/sui" "$INSTALL_DIR/README.md" "$INSTALL_DIR/PROTOCOL_GUIDE.md" "$INSTALL_DIR/s-ui.env"
  printf '数据文件已保留在：%s\n' "$INSTALL_DIR"
fi
printf '卸载完成。\n'
