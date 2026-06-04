#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/s-ui"
PLIST_PATH="/Library/LaunchDaemons/com.s-ui.plist"
CONFIG_FILE="$INSTALL_DIR/s-ui.env"

printf '========================================\n'
printf 'S-UI macOS 卸载程序\n'
printf '========================================\n\n'

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  printf '错误：请使用管理员权限运行，例如：sudo ./uninstall-macos.sh\n'
  exit 1
fi

if [ -f "$CONFIG_FILE" ]; then
  # shellcheck disable=SC1090
  . "$CONFIG_FILE"
fi

printf '正在卸载 launchd 服务...\n'
launchctl unload "$PLIST_PATH" >/dev/null 2>&1 || true
rm -f "$PLIST_PATH"

printf '是否保留数据库、日志和证书等数据文件？[Y/n]：'
read -r KEEP_DATA
if [ -z "$KEEP_DATA" ]; then KEEP_DATA="Y"; fi

if [ "$KEEP_DATA" = "Y" ] || [ "$KEEP_DATA" = "y" ]; then
  printf '正在保留数据文件，仅删除程序和服务文件...\n'
  rm -f "$INSTALL_DIR/sui" "$INSTALL_DIR/README.md" "$INSTALL_DIR/s-ui.env"
  printf '数据文件已保留在：%s\n' "$INSTALL_DIR"
else
  printf '正在删除所有文件...\n'
  rm -rf "$INSTALL_DIR"
fi

printf '\n卸载完成。\n'
