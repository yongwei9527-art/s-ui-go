#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="s-ui"
INSTALL_DIR="/usr/local/s-ui"
PLIST_PATH="/Library/LaunchDaemons/com.s-ui.plist"
CONFIG_FILE="$INSTALL_DIR/s-ui.env"
PANEL_PORT="2095"
PANEL_PATH="/app/"
SUB_PORT="2096"
SUB_PATH="/sub/"

script_args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --lang=*)
      SUI_LANG="${1#*=}"
      ;;
    --lang|-l)
      if [[ $# -lt 2 ]]; then
        printf 'Missing language value after %s / 缺少语言参数\n' "$1"
        exit 1
      fi
      SUI_LANG="$2"
      shift
      ;;
    *)
      script_args+=("$1")
      ;;
  esac
  shift
done
set -- "${script_args[@]}"

normalize_language() {
  local lang
  lang="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  case "$lang" in
    zh|zh-cn|zh_cn|zh-hans|zh_hans|cn|chinese|中文) printf 'zh' ;;
    en|en-us|en_us|english) printf 'en' ;;
    *) printf '' ;;
  esac
}

detect_default_language() {
  local env_lang
  env_lang="$(printf '%s' "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" | tr '[:upper:]' '[:lower:]')"
  if [[ "$env_lang" == zh* ]]; then
    printf 'zh'
  else
    printf 'zh'
  fi
}

select_install_language() {
  local normalized_lang default_lang default_choice lang_choice
  normalized_lang="$(normalize_language "${SUI_LANG:-}")"
  if [[ -n "$normalized_lang" ]]; then
    INSTALL_LANG="$normalized_lang"
    return
  fi

  default_lang="$(detect_default_language)"
  default_choice="2"
  if [[ "$default_lang" == "zh" ]]; then default_choice="1"; fi

  printf '请选择安装语言 / Please choose installation language\n'
  printf '1. 中文\n'
  printf '2. English\n'
  if [[ -t 0 ]]; then
    printf '请输入选项 / Enter choice [1-2] (default: %s): ' "$default_choice"
    read -r lang_choice
  fi
  lang_choice="${lang_choice:-$default_choice}"

  case "$lang_choice" in
    1|zh|ZH|cn|CN|中文) INSTALL_LANG="zh" ;;
    *) INSTALL_LANG="en" ;;
  esac
}

is_zh() {
  [[ "${INSTALL_LANG:-en}" == "zh" ]]
}

say() {
  local zh_text="$1"
  local en_text="$2"
  shift 2
  if is_zh; then
    printf "$zh_text" "$@"
  else
    printf "$en_text" "$@"
  fi
}

normalize_url_path() {
  local url_path="$1"
  if [[ -z "$url_path" ]]; then
    printf '/'
    return
  fi
  if [[ "${url_path:0:1}" != "/" ]]; then
    url_path="/${url_path}"
  fi
  if [[ "${url_path: -1}" != "/" ]]; then
    url_path="${url_path}/"
  fi
  printf '%s' "$url_path"
}

print_sui_failure_hint() {
  local output="$1"
  if [[ -n "$output" ]]; then printf '%s\n' "$output"; fi
}

run_sui_required() {
  local zh_desc="$1"
  local en_desc="$2"
  local output=""
  shift 2
  if ! output=$(./sui "$@" 2>&1); then
    if is_zh; then
      printf '错误：%s失败。\n' "$zh_desc"
    else
      printf 'Error: %s failed.\n' "$en_desc"
    fi
    print_sui_failure_hint "$output"
    exit 1
  fi
  if [[ -n "$output" ]]; then printf '%s\n' "$output"; fi
}

run_sui_optional() {
  local zh_desc="$1"
  local en_desc="$2"
  local output=""
  shift 2
  if ! output=$(./sui "$@" 2>&1); then
    say '%s未完成，可能是全新安装或旧数据库不存在。\n' '%s was not completed, possibly because this is a fresh install or the old database does not exist.\n' "$zh_desc"
    if [[ -n "$output" ]]; then printf '%s\n' "$output"; fi
    return 0
  fi
  if [[ -n "$output" ]]; then printf '%s\n' "$output"; fi
}

# Transaction helpers (also exercised by mocked tests).
validate_install_path() {
  local path="$1" part current="" rest
  case "$path" in /usr|/usr/local|/Library|/System|/Applications|/etc|/var|/tmp|/private|/private/tmp|/opt|/Users) return 1 ;; esac
  [[ "$path" == /* && "$path" != / && "$path" != */ && "$path" != *//* ]] || return 1
  [[ "$path" != *[!a-zA-Z0-9_./-]* ]] || return 1
  rest="${path#/}"
  while [[ -n "$rest" ]]; do
    part="${rest%%/*}"
    [[ "$part" != . && "$part" != .. && -n "$part" ]] || return 1
    current="$current/$part"
    [[ ! -L "$current" ]] || return 1
    if [[ "$rest" == */* ]]; then rest="${rest#*/}"; else rest=""; fi
  done
  [[ ! -e "$path" || -d "$path" ]]
}

assert_safe_targets() {
  validate_install_path "$INSTALL_DIR" || return 1
  local target
  for target in db logs cert sui s-ui.env README.md PROTOCOL_GUIDE.md; do
    [[ ! -L "$INSTALL_DIR/$target" ]] || return 1
  done
  if [[ -d "$INSTALL_DIR/db" ]]; then
    [[ -z "$(find "$INSTALL_DIR/db" -type l -print)" ]] || return 1
  fi
  [[ ! -L /Library && ! -L /Library/LaunchDaemons && ! -L "$PLIST_PATH" ]]
}

service_absent() {
  local output rc
  if output=$(launchctl print system/com.s-ui 2>&1); then return 1; fi
  [[ "$output" == *"Could not find service"* || "$output" == *"Could not find specified service"* ]] || return 1
  if output=$(pgrep -f "$INSTALL_DIR/sui" 2>&1); then return 1; else rc=$?; fi
  [[ "$rc" == 1 && -z "$output" ]]
}

stop_service() {
  launchctl bootout system/com.s-ui >/dev/null 2>&1 || true
  local attempt
  for attempt in 1 2 3 4 5; do
    if service_absent; then return 0; fi
    sleep 1
  done
  echo 'Cannot confirm stopped service; refusing database overwrite.' >&2
  return 1
}

rollback() {
  local status=$? target
  trap - EXIT
  if [[ "$status" != 0 && -n "${BACKUP_DIR:-}" && "${MUTATED:-0}" == 1 ]]; then
    if stop_service && assert_safe_targets; then
      for target in db sui s-ui.env README.md PROTOCOL_GUIDE.md; do
        rm -rf "$INSTALL_DIR/$target"
        if [[ -e "$BACKUP_DIR/$target" ]]; then cp -pR "$BACKUP_DIR/$target" "$INSTALL_DIR/$target"; fi
      done
      rm -f "$PLIST_PATH"
      if [[ -f "$BACKUP_DIR/service.plist" ]]; then
        cp -p "$BACKUP_DIR/service.plist" "$PLIST_PATH"
        if [[ "${SERVICE_WAS_LOADED:-0}" == 1 ]]; then
          launchctl bootstrap system "$PLIST_PATH" || echo 'Old service restart failed.' >&2
        fi
      fi
    else
      echo 'Rollback refused: service may still be running.' >&2
    fi
    printf 'Backup retained: %s\n' "$BACKUP_DIR" >&2
  fi
  if [[ "$status" != 0 && "${MUTATED:-0}" == 0 && "${SERVICE_WAS_LOADED:-0}" == 1 && -f "$PLIST_PATH" ]]; then
    launchctl bootstrap system "$PLIST_PATH" || echo 'Original service restart failed after backup error.' >&2
  fi
  exit "$status"
}
# END TRANSACTION HELPERS

select_install_language

printf '========================================\n'
say 'S-UI macOS 安装程序\n' 'S-UI macOS Installer\n'
printf '========================================\n\n'

if [[ "$(uname -s)" != Darwin ]]; then echo 'macOS required' >&2; exit 1; fi
if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  say '错误：请使用管理员权限运行，例如：sudo ./install-macos.sh\n' 'Error: please run with administrator privileges, for example: sudo ./install-macos.sh\n'
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "sui" ] && [ ! -f "sui-darwin-amd64" ] && [ ! -f "sui-darwin-arm64" ]; then
  say '错误：当前目录未找到 macOS 版 S-UI 主程序。\n' 'Error: macOS S-UI executable was not found in the current directory.\n'
  say '请将主程序命名为 sui，或使用 build-windows.ps1 构建 darwin 版本。\n' 'Please name the executable sui, or use build-windows.ps1 to build a darwin version.\n'
  exit 1
fi

say '安装目录，默认 %s：' 'Installation directory, default %s: ' "$INSTALL_DIR"
read -r input_install_dir
if [ -n "$input_install_dir" ]; then INSTALL_DIR="$input_install_dir"; fi
CONFIG_FILE="$INSTALL_DIR/s-ui.env"

assert_safe_targets || { echo 'Unsafe installation path or symlink' >&2; exit 1; }
EXISTING_DB=0
if [[ -d "$INSTALL_DIR/db" && -n "$(find "$INSTALL_DIR/db" -type f -print)" ]]; then EXISTING_DB=1; fi
if [[ "$EXISTING_DB" == 0 ]]; then
say '面板端口，默认 %s：' 'Panel port, default %s: ' "$PANEL_PORT"
read -r input_panel_port
if [ -n "$input_panel_port" ]; then PANEL_PORT="$input_panel_port"; fi

say '面板路径，默认 %s：' 'Panel path, default %s: ' "$PANEL_PATH"
read -r input_panel_path
if [ -n "$input_panel_path" ]; then PANEL_PATH="$input_panel_path"; fi
PANEL_PATH="$(normalize_url_path "$PANEL_PATH")"

say '订阅端口，默认 %s：' 'Subscription port, default %s: ' "$SUB_PORT"
read -r input_sub_port
if [ -n "$input_sub_port" ]; then SUB_PORT="$input_sub_port"; fi

say '订阅路径，默认 %s：' 'Subscription path, default %s: ' "$SUB_PATH"
read -r input_sub_path
if [ -n "$input_sub_path" ]; then SUB_PATH="$input_sub_path"; fi
SUB_PATH="$(normalize_url_path "$SUB_PATH")"

say '管理员用户名，默认 admin：' 'Admin username, default admin: '
read -r ADMIN_USERNAME
if [ -z "$ADMIN_USERNAME" ]; then ADMIN_USERNAME="admin"; fi

say '管理员密码：' 'Admin password: '
if [[ -t 0 ]]; then stty -echo; fi
read -r ADMIN_PASSWORD
if [[ -t 0 ]]; then stty echo; fi
printf '\n'
if [ -z "$ADMIN_PASSWORD" ]; then
  say '错误：密码不能为空。\n' 'Error: password cannot be empty.\n'
  exit 1
fi

fi
assert_safe_targets || exit 1
SERVICE_WAS_LOADED=0
if launchctl print system/com.s-ui >/dev/null 2>&1; then SERVICE_WAS_LOADED=1; fi
stop_service || exit 1
INSTALL_PARENT="${INSTALL_DIR%/*}"
INSTALL_PARENT="${INSTALL_PARENT:-/}"
mkdir -p "$INSTALL_PARENT"
BACKUP_DIR="$(mktemp -d "$INSTALL_PARENT/.s-ui-backup.XXXXXXXX")"
MUTATED=0
trap rollback EXIT
for target in db sui s-ui.env README.md PROTOCOL_GUIDE.md; do
  if [[ -e "$INSTALL_DIR/$target" ]]; then cp -pR "$INSTALL_DIR/$target" "$BACKUP_DIR/$target"; fi
done
if [[ -f "$PLIST_PATH" ]]; then cp -p "$PLIST_PATH" "$BACKUP_DIR/service.plist"; fi
assert_safe_targets || exit 1
MUTATED=1
say '正在创建目录...\n' 'Creating directories...\n'
mkdir -p "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/cert"

say '正在复制主程序...\n' 'Copying executable...\n'
if [ -f "sui" ]; then
  cp -f "sui" "$INSTALL_DIR/sui"
elif [ "$(uname -m)" = "arm64" ] && [ -f "sui-darwin-arm64" ]; then
  cp -f "sui-darwin-arm64" "$INSTALL_DIR/sui"
elif [ -f "sui-darwin-amd64" ]; then
  cp -f "sui-darwin-amd64" "$INSTALL_DIR/sui"
else
  cp -f "sui-darwin-arm64" "$INSTALL_DIR/sui"
fi
chmod +x "$INSTALL_DIR/sui"

if [ -f "README.md" ]; then cp -f "README.md" "$INSTALL_DIR/README.md"; fi
if [ -f "PROTOCOL_GUIDE.md" ]; then cp -f "PROTOCOL_GUIDE.md" "$INSTALL_DIR/PROTOCOL_GUIDE.md"; fi

say '正在执行数据库迁移...\n' 'Running database migration...\n'
cd "$INSTALL_DIR"
run_sui_required '数据库迁移' 'database migration' migrate

if [[ "$EXISTING_DB" == 0 ]]; then
say '正在应用配置...\n' 'Applying configuration...\n'
run_sui_required '应用面板和订阅配置' 'applying panel and subscription settings' setting -port "$PANEL_PORT" -path "$PANEL_PATH" -subPort "$SUB_PORT" -subPath "$SUB_PATH"

say '正在设置管理员账号...\n' 'Setting admin credentials...\n'
run_sui_required '设置管理员账号' 'setting admin credentials' admin -username "$ADMIN_USERNAME" -password "$ADMIN_PASSWORD"
unset ADMIN_PASSWORD

cat > "$CONFIG_FILE" <<EOF
INSTALL_DIR=$INSTALL_DIR
SERVICE_NAME=$SERVICE_NAME
PANEL_PORT=$PANEL_PORT
PANEL_PATH=$PANEL_PATH
SUB_PORT=$SUB_PORT
SUB_PATH=$SUB_PATH
EOF

fi
say '正在安装 launchd 服务...\n' 'Installing launchd service...\n'
cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.s-ui</string>
  <key>ProgramArguments</key>
  <array>
    <string>$INSTALL_DIR/sui</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$INSTALL_DIR</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$INSTALL_DIR/logs/stdout.log</string>
  <key>StandardErrorPath</key>
  <string>$INSTALL_DIR/logs/stderr.log</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SUI_DB_FOLDER</key>
    <string>db</string>
    <key>SUI_REQUIRE_PROXY_CORES</key>
    <string>1</string>
    <key>SUI_DEBUG</key>
    <string>false</string>
  </dict>
</dict>
</plist>
EOF
chmod 644 "$PLIST_PATH"
launchctl bootstrap system "$PLIST_PATH"
launchctl kickstart system/com.s-ui
health_pid=""
for attempt in 1 2 3 4 5; do
  state="$(launchctl print system/com.s-ui)"
  pid="$(printf '%s\n' "$state" | awk '$1 == "pid" && $2 == "=" {print $3}')"
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || exit 1
  kill -0 "$pid" || exit 1
  [[ -z "$health_pid" || "$health_pid" == "$pid" ]] || exit 1
  health_pid="$pid"
  sleep 1
done
if [[ "$EXISTING_DB" == 1 && "$SERVICE_WAS_LOADED" == 0 ]]; then stop_service || exit 1; fi
trap - EXIT
printf 'Backup retained: %s\n' "$BACKUP_DIR"

printf '\n========================================\n'
say '安装完成\n' 'Installation completed\n'
printf '========================================\n'
say '安装目录：%s\n' 'Installation directory: %s\n' "$INSTALL_DIR"
if [[ "$EXISTING_DB" == 0 ]]; then
  say '面板地址：http://localhost:%s%s\n' 'Panel URL: http://localhost:%s%s\n' "$PANEL_PORT" "$PANEL_PATH"
  say '订阅地址：http://localhost:%s%s\n' 'Subscription URL: http://localhost:%s%s\n' "$SUB_PORT" "$SUB_PATH"
else
  say '已保留数据库中的配置和管理员。\n' 'Existing database settings and administrator preserved.\n'
fi
say '服务配置：%s\n' 'Service configuration: %s\n' "$PLIST_PATH"
