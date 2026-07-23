#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="s-ui"
INSTALL_DIR="/opt/s-ui"
CONFIG_FILE="$INSTALL_DIR/s-ui.env"
PANEL_PORT="2095"
PANEL_PATH="/app/"
SUB_PORT="2096"
SUB_PATH="/sub/"
DEFAULT_SING_REALITY_PORT="443"
DEFAULT_XRAY_REALITY_PORT="10443"
DEFAULT_XRAY_XHTTP_PORT="2088"
DEFAULT_REALITY_TARGET_HOST="go.microsoft.com"

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

get_server_ip() {
  local ip=""
  if command -v curl >/dev/null 2>&1; then
    ip="$(curl -fsSL --max-time 3 https://api.ipify.org 2>/dev/null || true)"
  fi
  if [[ -z "$ip" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  printf '%s' "$ip"
}

format_host() {
  local host="$1"
  if [[ "$host" == *:* && "${host:0:1}" != "[" ]]; then
    printf '[%s]' "$host"
  else
    printf '%s' "$host"
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

# A fresh packaged installation can be made immediately usable without
# enabling a firewall globally or changing an existing firewall policy.
# Cloud security groups cannot be inspected or changed from inside the VPS.
port_is_available() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ! ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .
    return
  fi
  # ss is part of iproute2 on supported distributions. Without it, leave the
  # authoritative listener check to the core at startup instead of guessing.
  return 0
}

reality_target_is_usable() {
  local host="$1"
  if ! command -v openssl >/dev/null 2>&1; then
    say '未找到 openssl，无法自动验证 Reality 目标；Xray 模板将保持手动启用。\n' 'openssl was not found, so the Reality target cannot be verified automatically. The Xray templates will remain manual.\n'
    return 1
  fi
  timeout 12 openssl s_client -connect "$host:443" -servername "$host" -verify_return_error </dev/null 2>/dev/null | grep -q 'Verify return code: 0 (ok)'
}

configure_active_ufw() {
  local port
  if ! command -v ufw >/dev/null 2>&1; then
    return 0
  fi
  if ! ufw status 2>/dev/null | grep -q '^Status: active'; then
    return 0
  fi
  for port in "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"; do
    if ! ufw allow "$port/tcp" comment 'S-UI Reality' >/dev/null; then
      say '警告：无法在 UFW 中添加 TCP %s 放行规则。\n' 'Warning: could not add the UFW allow rule for TCP %s.\n' "$port"
      return 1
    fi
  done
  say '已在活动 UFW 中放行 sing-box Reality TCP %s 及 Xray Reality TCP/XHTTP %s/%s。\n' 'Allowed sing-box Reality TCP %s and Xray Reality TCP/XHTTP %s/%s in active UFW.\n' "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"
}

wait_for_tcp_listener() {
  local port="$1" attempt
  for attempt in 1 2 3 4 5; do
    if command -v ss >/dev/null 2>&1 && ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .; then
      return 0
    fi
    sleep 1
  done
  return 1
}

select_install_language

printf '========================================\n'
say 'S-UI Linux 安装程序\n' 'S-UI Linux Installer\n'
printf '========================================\n\n'

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  say '错误：请使用 root 权限运行，例如：sudo ./install-linux.sh\n' 'Error: please run with root privileges, for example: sudo ./install-linux.sh\n'
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "sui" ] && [ ! -f "sui-linux-amd64" ] && [ ! -f "sui-linux-arm64" ] && [ ! -f "sui-linux-arm" ]; then
  say '错误：当前目录未找到 Linux 版 S-UI 主程序。\n' 'Error: Linux S-UI executable was not found in the current directory.\n'
  say '请将主程序命名为 sui，或使用 build-windows.ps1 构建 linux 版本。\n' 'Please name the executable sui, or use build-windows.ps1 to build a Linux version.\n'
  exit 1
fi

say '安装目录，默认 %s：' 'Installation directory, default %s: ' "$INSTALL_DIR"
read -r input_install_dir
if [ -n "$input_install_dir" ]; then INSTALL_DIR="$input_install_dir"; fi
CONFIG_FILE="$INSTALL_DIR/s-ui.env"
FRESH_INSTALL=0
if [ ! -f "$INSTALL_DIR/db/s-ui.db" ]; then
  FRESH_INSTALL=1
fi

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
stty -echo
read -r ADMIN_PASSWORD
stty echo
printf '\n'
if [ -z "$ADMIN_PASSWORD" ]; then
  say '错误：密码不能为空。\n' 'Error: password cannot be empty.\n'
  exit 1
fi

say '正在创建目录...\n' 'Creating directories...\n'
mkdir -p "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/cert"

say '正在复制主程序...\n' 'Copying executable...\n'
if [ -f "sui" ]; then
  cp -f "sui" "$INSTALL_DIR/sui"
elif [ -f "sui-linux-$(uname -m)" ]; then
  cp -f "sui-linux-$(uname -m)" "$INSTALL_DIR/sui"
elif [ -f "sui-linux-amd64" ]; then
  cp -f "sui-linux-amd64" "$INSTALL_DIR/sui"
elif [ -f "sui-linux-arm64" ]; then
  cp -f "sui-linux-arm64" "$INSTALL_DIR/sui"
else
  cp -f "sui-linux-arm" "$INSTALL_DIR/sui"
fi
chmod +x "$INSTALL_DIR/sui"

XRAY_BINARY=""
if [ -f "xray" ]; then
  say '正在安装随包附带的 Xray-core...\n' 'Installing bundled Xray-core...\n'
  cp -f "xray" "$INSTALL_DIR/xray"
  chmod 0755 "$INSTALL_DIR/xray"
  XRAY_BINARY="$INSTALL_DIR/xray"
  say 'Xray-core 已安装：%s\n' 'Xray-core installed: %s\n' "$XRAY_BINARY"
fi

XRAY_AUTOCONFIGURED=0
if [ "$FRESH_INSTALL" -eq 1 ] && [ -n "$XRAY_BINARY" ]; then
  if ! port_is_available "$DEFAULT_SING_REALITY_PORT"; then
    say '警告：TCP %s 已被占用，无法创建可直接使用的双核心 Reality 默认配置；Xray 模板将保持手动启用。\n' 'Warning: TCP %s is already occupied, so the ready-to-use dual-core Reality defaults cannot be created; the Xray templates will remain manual.\n' "$DEFAULT_SING_REALITY_PORT"
  elif ! port_is_available "$DEFAULT_XRAY_REALITY_PORT"; then
    say '警告：TCP %s 已被占用，无法创建可直接使用的 Xray Reality 默认入站；模板将保持手动启用。\n' 'Warning: TCP %s is already occupied, so the ready-to-use Xray Reality inbounds cannot be created; the templates will remain manual.\n' "$DEFAULT_XRAY_REALITY_PORT"
  elif ! port_is_available "$DEFAULT_XRAY_XHTTP_PORT"; then
    say '警告：TCP %s 已被占用，无法创建可直接使用的 Xray Reality XHTTP 入站；模板将保持手动启用。\n' 'Warning: TCP %s is already occupied, so the ready-to-use Xray Reality XHTTP inbound cannot be created; the templates will remain manual.\n' "$DEFAULT_XRAY_XHTTP_PORT"
  elif ! reality_target_is_usable "$DEFAULT_REALITY_TARGET_HOST"; then
    say '警告：Reality 目标 %s:443 验证失败；模板将保持手动启用。\n' 'Warning: Reality target %s:443 verification failed; the templates will remain manual.\n' "$DEFAULT_REALITY_TARGET_HOST"
  else
    XRAY_AUTOCONFIGURED=1
    export SUI_AUTOCONFIGURE_XRAY_REALITY=1
    say 'Reality 目标 %s:443 已验证；Xray VLESS Reality TCP Vision 与 XHTTP 将自动启用。\n' 'Reality target %s:443 verified; Xray VLESS Reality TCP Vision and XHTTP will be enabled automatically.\n' "$DEFAULT_REALITY_TARGET_HOST"
  fi
fi

if [ -f "README.md" ]; then cp -f "README.md" "$INSTALL_DIR/README.md"; fi
if [ -f "PROTOCOL_GUIDE.md" ]; then cp -f "PROTOCOL_GUIDE.md" "$INSTALL_DIR/PROTOCOL_GUIDE.md"; fi

say '正在执行数据库迁移...\n' 'Running database migration...\n'
cd "$INSTALL_DIR"
run_sui_optional '数据库迁移' 'database migration' migrate

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

if [ -n "$XRAY_BINARY" ]; then
  printf 'SUI_XRAY_BINARY=%s\n' "$XRAY_BINARY" >> "$CONFIG_FILE"
fi

XRAY_SYSTEMD_ENV=""
if [ -n "$XRAY_BINARY" ]; then
  XRAY_SYSTEMD_ENV="Environment=SUI_XRAY_BINARY=$XRAY_BINARY"
fi

AUTO_XRAY_SYSTEMD_ENV=""
if [ "$XRAY_AUTOCONFIGURED" -eq 1 ]; then
  AUTO_XRAY_SYSTEMD_ENV="Environment=SUI_AUTOCONFIGURE_XRAY_REALITY=1"
fi

if command -v systemctl >/dev/null 2>&1; then
  say '正在安装 systemd 服务...\n' 'Installing systemd service...\n'
  cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=S-UI Proxy Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/sui
Restart=on-failure
RestartSec=10
Environment=SUI_DB_FOLDER=db
Environment=SUI_DEBUG=false
$XRAY_SYSTEMD_ENV
$AUTO_XRAY_SYSTEMD_ENV

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  if ! systemctl restart "$SERVICE_NAME"; then
    say '错误：服务启动失败，请查看 systemd 状态。\n' 'Error: service failed to start. Please check systemd status.\n'
    systemctl status "$SERVICE_NAME" -l --no-pager || true
    exit 1
  fi
  sleep 2
  if ! systemctl is-active --quiet "$SERVICE_NAME"; then
    say '错误：服务未处于运行状态，请查看 systemd 状态。\n' 'Error: service is not running. Please check systemd status.\n'
    systemctl status "$SERVICE_NAME" -l --no-pager || true
    exit 1
  fi
  if [ "$XRAY_AUTOCONFIGURED" -eq 1 ] && ! wait_for_tcp_listener "$DEFAULT_SING_REALITY_PORT"; then
    say '错误：sing-box Reality TCP %s 未监听；请检查服务日志。\n' 'Error: sing-box Reality TCP %s is not listening; please check the service log.\n' "$DEFAULT_SING_REALITY_PORT"
    journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
    exit 1
  fi
  if [ "$XRAY_AUTOCONFIGURED" -eq 1 ] && ! wait_for_tcp_listener "$DEFAULT_XRAY_REALITY_PORT"; then
    say '错误：Xray Reality TCP %s 未监听；请检查服务日志。\n' 'Error: Xray Reality TCP %s is not listening; please check the service log.\n' "$DEFAULT_XRAY_REALITY_PORT"
    journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
    exit 1
  fi
  if [ "$XRAY_AUTOCONFIGURED" -eq 1 ] && ! wait_for_tcp_listener "$DEFAULT_XRAY_XHTTP_PORT"; then
    say '错误：Xray Reality XHTTP %s 未监听；请检查服务日志。\n' 'Error: Xray Reality XHTTP %s is not listening; please check the service log.\n' "$DEFAULT_XRAY_XHTTP_PORT"
    journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
    exit 1
  fi
  if [ "$XRAY_AUTOCONFIGURED" -eq 1 ] && ! configure_active_ufw; then
    say '错误：Reality 监听已启动，但无法更新活动 UFW。使用生成节点前请放行 TCP %s、%s、%s。\n' 'Error: Reality listeners are running, but the active UFW rules could not be updated. Check TCP %s, %s, and %s before using the generated nodes.\n' "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"
    exit 1
  fi
else
  say '警告：未检测到 systemd，请手动运行：%s/sui\n' 'Warning: systemd was not detected. Please run manually: %s/sui\n' "$INSTALL_DIR"
fi

printf '\n========================================\n'
say '安装完成\n' 'Installation completed\n'
printf '========================================\n'
say '安装目录：%s\n' 'Installation directory: %s\n' "$INSTALL_DIR"
SERVER_IP="$(get_server_ip)"
if [ -n "$SERVER_IP" ]; then
  SERVER_HOST="$(format_host "$SERVER_IP")"
  say '面板完整地址：http://%s:%s%s\n' 'Panel URL: http://%s:%s%s\n' "$SERVER_HOST" "$PANEL_PORT" "$PANEL_PATH"
  say '订阅完整地址：http://%s:%s%s\n' 'Subscription URL: http://%s:%s%s\n' "$SERVER_HOST" "$SUB_PORT" "$SUB_PATH"
else
  say '未能自动读取服务器 IP，请将 localhost 替换为服务器公网 IP。\n' 'Failed to detect server IP automatically. Please replace localhost with your server public IP.\n'
fi
say '本机面板地址：http://localhost:%s%s\n' 'Local panel URL: http://localhost:%s%s\n' "$PANEL_PORT" "$PANEL_PATH"
say '本机订阅地址：http://localhost:%s%s\n' 'Local subscription URL: http://localhost:%s%s\n' "$SUB_PORT" "$SUB_PATH"
say '服务名称：%s\n' 'Service name: %s\n' "$SERVICE_NAME"
if [ "$XRAY_AUTOCONFIGURED" -eq 1 ]; then
  say '已自动启用：sing-box VLESS Reality TCP（%s）、Xray VLESS Reality TCP Vision（%s）及 Xray VLESS Reality XHTTP（%s）。仅在客户端支持 type=xhttp 时使用 XHTTP。\n' 'Enabled automatically: sing-box VLESS Reality TCP (%s), Xray VLESS Reality TCP Vision (%s), and Xray VLESS Reality XHTTP (%s). Use XHTTP only with a client that supports type=xhttp.\n' "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"
  say '请确认云安全组已放行 TCP %s、TCP %s、TCP %s；VPS 安装器无法自动修改云安全组。\n' 'Confirm that your cloud security group allows TCP %s, TCP %s, and TCP %s; a VPS installer cannot change cloud security groups automatically.\n' "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"
fi
