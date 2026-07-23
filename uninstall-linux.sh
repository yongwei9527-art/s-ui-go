#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

readonly SERVICE_NAME="s-ui"
readonly DEFAULT_INSTALL_DIR="/usr/local/s-ui"
readonly UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

INSTALL_DIR="$DEFAULT_INSTALL_DIR"
INSTALL_DIR_FIXED=0
KEEP_DATA=""
INSTALL_LANG="${SUI_LANG:-zh}"

say() {
    local zh_text="$1" en_text="$2"
    shift 2
    if [[ "$INSTALL_LANG" == en* ]]; then
        printf "$en_text" "$@"
    else
        printf "$zh_text" "$@"
    fi
}

die() {
    say '错误：%s\n' 'Error: %s\n' "$1" >&2
    exit 1
}

path_has_symlink_component() {
    local candidate="$1" remainder component current="/"
    remainder="${candidate#/}"
    IFS='/' read -r -a _path_components <<< "$remainder"
    for component in "${_path_components[@]}"; do
        [[ -n "$component" ]] || continue
        current="${current%/}/$component"
        [[ -L "$current" ]] && return 0
    done
    return 1
}

normalize_install_dir() {
    local raw="$1" normalized wrapped
    command -v realpath >/dev/null 2>&1 || die '需要 GNU realpath / GNU realpath is required.'
    realpath -m -- / >/dev/null 2>&1 || die '需要支持 -m 的 GNU realpath / GNU realpath with -m support is required.'
    [[ -n "$raw" && "$raw" == /* ]] || die '安装目录必须是绝对路径 / installation directory must be absolute.'
    [[ "$raw" =~ ^[A-Za-z0-9._/-]+$ ]] || die '安装目录包含不安全字符 / installation directory contains unsafe characters.'
    wrapped="/${raw#/}/"
    [[ "$wrapped" != *'/../'* && "$wrapped" != *'/./'* ]] || die '安装目录不得包含 . 或 .. 路径穿越组件 / path traversal components are not allowed.'
    path_has_symlink_component "$raw" && die '安装目录或其父目录不能是符号链接 / installation directory and parents must not be symlinks.'
    normalized="$(realpath -m -- "$raw")" || die '无法规范化安装目录 / failed to normalize installation directory.'
    path_has_symlink_component "$normalized" && die '规范化后的安装目录包含符号链接 / normalized installation directory contains a symlink.'
    case "$normalized" in
        /usr/local/*|/opt/*|/srv/*) ;;
        *) die '安装目录仅允许位于 /usr/local/*、/opt/* 或 /srv/*。' ;;
    esac
    case "$normalized" in
        /usr/local|/opt|/srv|/usr/local/bin|/usr/local/sbin|/usr/local/lib|/usr/local/lib64|/usr/local/share|/usr/local/include|/usr/local/src)
            die '安装目录过于宽泛或危险 / installation directory is too broad or dangerous.'
            ;;
    esac
    INSTALL_DIR="$normalized"
}

assert_safe_regular_file() {
    local file="$1" label="$2"
    [[ -e "$file" || -L "$file" ]] || return 0
    [[ -f "$file" && ! -L "$file" ]] || die "$label must be a regular non-symlink file."
}

assert_root_private_file() {
    local file="$1" label="$2" owner="" mode=""
    [[ -e "$file" || -L "$file" ]] || return 0
    assert_safe_regular_file "$file" "$label"
    owner="$(stat -c '%u:%g' -- "$file" 2>/dev/null || true)"
    mode="$(stat -c '%a' -- "$file" 2>/dev/null || true)"
    [[ "$owner" == "0:0" && "$mode" =~ ^[0-7]{3,4}$ ]] || die "$label must be root-owned with a numeric mode."
    (( (8#$mode & 077) == 0 )) || die "$label must not be readable or writable by group/other users."
}

assert_root_owned_nonwritable_file() {
    local file="$1" label="$2" owner="" mode=""
    assert_safe_regular_file "$file" "$label"
    [[ -e "$file" ]] || return 0
    owner="$(stat -c '%u:%g' -- "$file" 2>/dev/null || true)"
    mode="$(stat -c '%a' -- "$file" 2>/dev/null || true)"
    [[ "$owner" == "0:0" && "$mode" =~ ^[0-7]{3,4}$ ]] || die "$label must be root-owned with a numeric mode."
    (( (8#$mode & 022) == 0 )) || die "$label must not be writable by group/other users."
}

verify_install_state_identity() {
    local state_file="$INSTALL_DIR/.s-ui-install-state" state="" state_dir=""
    [[ -e "$state_file" || -L "$state_file" ]] || return 1
    assert_root_private_file "$state_file" 'install-state file'
    state="$(sed -n 's/^STATE=//p' "$state_file")"
    state_dir="$(sed -n 's/^INSTALL_DIR=//p' "$state_file")"
    case "$state" in
        ready|installing) ;;
        *) die 'Install-state file has an invalid state.' ;;
    esac
    [[ "$state_dir" == "$INSTALL_DIR" ]] || die 'Install-state directory does not match the requested installation directory.'
    return 0
}


require_real_systemd() {
    if ! command -v systemctl >/dev/null 2>&1 || [[ ! -d /run/systemd/system ]] ||
       [[ ! -r /proc/1/comm ]] || [[ "$(cat /proc/1/comm)" != systemd ]] ||
       ! systemctl show-environment >/dev/null 2>&1; then
        die '需要运行中的真实 systemd（PID 1）/ a real running systemd (PID 1) is required.'
    fi
}

service_is_inactive() {
    [[ "$(systemctl show -p ActiveState --value "$SERVICE_NAME" 2>/dev/null || true)" == inactive ]]
}

unit_belongs_to_install_dir() {
    [[ -f "$UNIT_PATH" && ! -L "$UNIT_PATH" ]] || return 1
    grep -Fqx -- "WorkingDirectory=$INSTALL_DIR" "$UNIT_PATH" &&
        grep -Fqx -- "ExecStart=$INSTALL_DIR/sui" "$UNIT_PATH"
}

ufw_rule_numbers_for_comment() {
    local comment="$1" line=""
    while IFS= read -r line; do
        [[ "$line" == *"# $comment"* ]] || continue
        if [[ "$line" =~ ^[[:space:]]*\[[[:space:]]*([0-9]+)\] ]]; then
            printf '%s\n' "${BASH_REMATCH[1]}"
        fi
    done < <(ufw status numbered 2>/dev/null || true)
}

remove_managed_ufw_rules() {
    local state_file="$INSTALL_DIR/.s-ui-ufw-state" key value comment="" ports="" index number
    local -a numbers=()
    [[ -e "$state_file" || -L "$state_file" ]] || return 0
    assert_root_private_file "$state_file" 'UFW state file'
    while IFS='=' read -r key value; do
        case "$key" in
            COMMENT) comment="$value" ;;
            PORTS) ports="$value" ;;
        esac
    done < "$state_file"
    [[ "$comment" =~ ^s-ui-go-[0-9a-f]{24}$ ]] || die 'UFW state-file marker is invalid.'
    [[ "$ports" == '443 10443 2088' ]] || die 'UFW state-file port list is invalid.'
    command -v ufw >/dev/null 2>&1 || die 'UFW is unavailable; cannot safely remove installer-managed rules.'
    mapfile -t numbers < <(ufw_rule_numbers_for_comment "$comment")
    for ((index=${#numbers[@]} - 1; index >= 0; index--)); do
        number="${numbers[$index]}"
        [[ "$number" =~ ^[0-9]+$ ]] || die 'Invalid UFW rule number.'
        ufw --force delete "$number" >/dev/null || die 'Failed to remove an installer-added UFW rule.'
    done
    rm -f -- "$state_file"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --install-dir=*) INSTALL_DIR="${1#*=}"; INSTALL_DIR_FIXED=1 ;;
        --install-dir)
            [[ $# -ge 2 ]] || die '缺少安装目录参数 / missing installation directory.'
            INSTALL_DIR="$2"; INSTALL_DIR_FIXED=1; shift
            ;;
        --purge) KEEP_DATA=N ;;
        --keep-data) KEEP_DATA=Y ;;
        --lang=*) INSTALL_LANG="${1#*=}" ;;
        --lang)
            [[ $# -ge 2 ]] || die '缺少语言参数 / missing language value.'
            INSTALL_LANG="$2"; shift
            ;;
        --help|-h)
            printf 'Usage: uninstall-linux.sh [--install-dir /usr/local/name|/opt/name|/srv/name] [--keep-data|--purge]\n'
            exit 0
            ;;
        *) die "不支持的参数：$1 / unsupported option: $1" ;;
    esac
    shift
done

[[ "${EUID:-$(id -u)}" -eq 0 ]] || die '请使用 root 权限运行 / run this uninstaller as root.'
require_real_systemd
SCRIPT_DIR="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
if [[ "$INSTALL_DIR_FIXED" -eq 0 ]] && { [[ -e "$SCRIPT_DIR/s-ui.env" ]] || [[ -L "$SCRIPT_DIR/s-ui.env" ]]; }; then
    assert_root_owned_nonwritable_file "$SCRIPT_DIR/s-ui.env" 'local installer configuration file'
    INSTALL_DIR="$SCRIPT_DIR"
fi
normalize_install_dir "$INSTALL_DIR"
CONFIG_FILE="$INSTALL_DIR/s-ui.env"
if [[ "$INSTALL_DIR_FIXED" -eq 0 ]] && { [[ -e "$CONFIG_FILE" ]] || [[ -L "$CONFIG_FILE" ]]; }; then
    assert_root_owned_nonwritable_file "$CONFIG_FILE" 'installer configuration file'
    configured_dir="$(sed -n 's/^INSTALL_DIR=//p' "$CONFIG_FILE")"
    if [[ -n "$configured_dir" ]]; then
        normalize_install_dir "$configured_dir"
        CONFIG_FILE="$INSTALL_DIR/s-ui.env"
    fi
fi
[[ -d "$INSTALL_DIR" && ! -L "$INSTALL_DIR" ]] || die '目标安装目录不存在、不是目录或是符号链接 / target installation directory is missing, not a directory, or is a symlink.'

printf '========================================\n'
say 's-ui-go Linux 卸载程序\n' 's-ui-go Linux Uninstaller\n'
printf '========================================\n'
say '目标安装目录：%s\n' 'Target installation directory: %s\n' "$INSTALL_DIR"

MANAGER_OWNED=0
if [[ -f "$INSTALL_DIR/s-ui.sh" && ! -L "$INSTALL_DIR/s-ui.sh" &&
      -f /usr/local/bin/s-ui && ! -L /usr/local/bin/s-ui ]] && cmp -s "$INSTALL_DIR/s-ui.sh" /usr/local/bin/s-ui; then
    MANAGER_OWNED=1
fi

INSTALL_IDENTITY_VERIFIED=0
if verify_install_state_identity; then
    INSTALL_IDENTITY_VERIFIED=1
fi

if [[ -e "$UNIT_PATH" || -L "$UNIT_PATH" ]]; then
    [[ -f "$UNIT_PATH" && ! -L "$UNIT_PATH" ]] || die 'Existing systemd unit is not a safe regular file.'
    unit_belongs_to_install_dir || die 'systemd unit does not belong to the target directory; refusing to delete files.'
    fragment="$(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || true)"
    [[ -z "$fragment" || "$fragment" == "$UNIT_PATH" ]] || die 'systemd loaded a same-name unit from another location; refusing to delete files.'
    INSTALL_IDENTITY_VERIFIED=1
    systemctl stop "$SERVICE_NAME" || die 'Failed to stop service; refusing to delete files.'
    service_is_inactive || die 'Service is not confirmed inactive; refusing to delete files.'
    systemctl disable "$SERVICE_NAME" || die 'Failed to disable service; refusing to delete files.'
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        die 'Service remains enabled; refusing to delete files.'
    fi
    rm -f -- "$UNIT_PATH"
    systemctl daemon-reload
    systemctl reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true
else
    fragment="$(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || true)"
    [[ -z "$fragment" ]] || die 'A same-name systemd unit exists elsewhere; refusing to delete files.'
fi

[[ "$INSTALL_IDENTITY_VERIFIED" -eq 1 ]] || die 'Refusing to delete this directory: no matching s-ui-go unit or private install-state record was found.'

remove_managed_ufw_rules

if [[ -z "$KEEP_DATA" ]]; then
    KEEP_DATA=Y
    if [[ -t 0 ]]; then
        printf '保留数据库、日志、证书和备份数据？[Y/n] / Keep database, logs, certificates, and backups? [Y/n]: '
        read -r answer || die '读取卸载选项失败 / failed to read uninstall choice.'
        case "${answer:-Y}" in
            N|n|No|NO|no) KEEP_DATA=N ;;
            *) KEEP_DATA=Y ;;
        esac
    fi
fi

if [[ "$KEEP_DATA" == Y || "$KEEP_DATA" == y ]]; then
    say '正在保留数据，仅删除程序、脚本和安装状态...\n' 'Keeping data; removing only program, scripts, and install state...\n'
    rm -f -- "$INSTALL_DIR/sui" "$INSTALL_DIR/install.sh" "$INSTALL_DIR/install-linux.sh" "$INSTALL_DIR/uninstall-linux.sh" "$INSTALL_DIR/s-ui.sh" "$INSTALL_DIR/README.md" "$INSTALL_DIR/PROTOCOL_GUIDE.md" "$INSTALL_DIR/s-ui.env" "$INSTALL_DIR/.s-ui-install-state" "$INSTALL_DIR/xray-xhttp.json"
else
    say '正在删除目标安装目录...\n' 'Removing target installation directory...\n'
    rm -rf -- "$INSTALL_DIR"
fi
if [[ "$MANAGER_OWNED" -eq 1 ]]; then
    rm -f -- /usr/local/bin/s-ui
else
    say '未删除 /usr/local/bin/s-ui：它不是此安装目录的已验证管理器副本。\n' 'Did not remove /usr/local/bin/s-ui: it is not the verified manager copy from this installation directory.\n'
fi
say '卸载完成。专用 s-ui 用户/组为避免误删其他资源而保留。\n' 'Uninstall complete. The dedicated s-ui user/group was retained to avoid deleting resources that may be shared.\n'