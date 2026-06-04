#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="s-ui"
INSTALL_DIR="/opt/s-ui"
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
    printf 'en'
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

say '面板端口，默认 %s：' 'Panel port, default %s: ' "$PANEL_PORT"
read -r input_panel_port
if [ -n "$input_panel_port" ]; then PANEL_PORT="$input_panel_port"; fi

say '面板路径，默认 %s：' 'Panel path, default %s: ' "$PANEL_PATH"
read -r input_panel_path
if [ -n "$input_panel_path" ]; then PANEL_PATH="$input_panel_path"; fi

say '订阅端口，默认 %s：' 'Subscription port, default %s: ' "$SUB_PORT"
read -r input_sub_port
if [ -n "$input_sub_port" ]; then SUB_PORT="$input_sub_port"; fi

say '订阅路径，默认 %s：' 'Subscription path, default %s: ' "$SUB_PATH"
read -r input_sub_path
if [ -n "$input_sub_path" ]; then SUB_PATH="$input_sub_path"; fi

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

if [ -f "README.md" ]; then cp -f "README.md" "$INSTALL_DIR/README.md"; fi

say '正在执行数据库迁移...\n' 'Running database migration...\n'
cd "$INSTALL_DIR"
if ! ./sui migrate; then
  say '警告：数据库迁移失败，或当前是新数据库。\n' 'Warning: database migration failed, or this is a new database.\n'
fi

say '正在应用配置...\n' 'Applying configuration...\n'
if ! ./sui setting -port "$PANEL_PORT" -path "$PANEL_PATH" -subPort "$SUB_PORT" -subPath "$SUB_PATH"; then
  say '警告：网络配置应用失败。\n' 'Warning: network configuration failed.\n'
fi

say '正在设置管理员账号...\n' 'Setting admin credentials...\n'
if ! ./sui admin -username "$ADMIN_USERNAME" -password "$ADMIN_PASSWORD"; then
  say '警告：管理员账号设置失败。\n' 'Warning: admin credentials setup failed.\n'
fi
unset ADMIN_PASSWORD

cat > "$CONFIG_FILE" <<EOF
INSTALL_DIR=$INSTALL_DIR
SERVICE_NAME=$SERVICE_NAME
PANEL_PORT=$PANEL_PORT
PANEL_PATH=$PANEL_PATH
SUB_PORT=$SUB_PORT
SUB_PATH=$SUB_PATH
EOF

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

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
else
  say '警告：未检测到 systemd，请手动运行：%s/sui\n' 'Warning: systemd was not detected. Please run manually: %s/sui\n' "$INSTALL_DIR"
fi

printf '\n========================================\n'
say '安装完成\n' 'Installation completed\n'
printf '========================================\n'
say '安装目录：%s\n' 'Installation directory: %s\n' "$INSTALL_DIR"
say '面板地址：http://localhost:%s%s\n' 'Panel URL: http://localhost:%s%s\n' "$PANEL_PORT" "$PANEL_PATH"
say '订阅地址：http://localhost:%s%s\n' 'Subscription URL: http://localhost:%s%s\n' "$SUB_PORT" "$SUB_PATH"
say '服务名称：%s\n' 'Service name: %s\n' "$SERVICE_NAME"
