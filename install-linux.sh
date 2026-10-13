#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

# STATIC-CHECK: bash -n install-linux.sh && grep -Fq 'upgrade_requires_strict_proxy_mode' install-linux.sh
# The upgrade detector runs before the old unit is stopped, so a legacy default
# three-listener instance cannot be downgraded to a non-strict successful upgrade.

readonly PRODUCT_NAME="s-ui-go"
readonly SERVICE_NAME="s-ui"
readonly SERVICE_USER="s-ui"
readonly SERVICE_GROUP="s-ui"
readonly DEFAULT_INSTALL_DIR="/usr/local/s-ui"
readonly UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
readonly DEFAULT_SING_REALITY_PORT="443"
readonly DEFAULT_XRAY_REALITY_PORT="10443"
readonly DEFAULT_XRAY_XHTTP_PORT="2088"
readonly DEFAULT_REALITY_TARGET_HOST="go.microsoft.com"

INSTALL_DIR="${SUI_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
INSTALL_DIR_FIXED=0
[[ -n "${SUI_INSTALL_DIR:-}" ]] && INSTALL_DIR_FIXED=1
UPGRADE_ONLY=0
PANEL_PORT="2095"
PANEL_PATH="/app/"
SUB_PORT="2096"
SUB_PATH="/sub/"
INSTALL_LANG=""

FRESH_INSTALL=0
LEGACY_INSTALL=0
XRAY_AUTOCONFIGURED=0
REQUIRE_PROXY_CORES=0
STRICT_PROXY_HEALTH=0
DEFAULT_THREE_LISTENER_CONFIG=0
INSTALL_STATE_WRITTEN=0
UFW_RULES_ADDED_THIS_RUN=0
UFW_COMMENT=""
BACKUP_DIR=""
BACKUP_READY=0
UPGRADE_SERVICE_STOPPED=0
SERVICE_UNIT_EXISTED=0
SERVICE_WAS_ACTIVE=0
SERVICE_WAS_ENABLED=0
TEMPORARILY_STARTED_FOR_HEALTH=0
TTY_ECHO_DISABLED=0

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

normalize_language() {
    local value
    value="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
    case "$value" in
        zh|zh-cn|zh_cn|zh-hans|zh_hans|cn|chinese|中文) printf 'zh' ;;
        en|en-us|en_us|english) printf 'en' ;;
        *) printf '' ;;
    esac
}

select_install_language() {
    local selected default_choice="1"
    selected="$(normalize_language "${SUI_LANG:-}")"
    if [[ -n "$selected" ]]; then
        INSTALL_LANG="$selected"
        return
    fi
    [[ "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" == zh* ]] || default_choice="2"
    printf '请选择安装语言 / Please choose installation language\n'
    printf '1. 中文\n2. English\n'
    selected="$default_choice"
    if [[ -t 0 ]]; then
        read -r -p "请输入选项 / Enter choice [1-2] (default: $default_choice): " selected || true
        selected="${selected:-$default_choice}"
    fi
    case "$selected" in
        1|zh|ZH|cn|CN|中文) INSTALL_LANG="zh" ;;
        *) INSTALL_LANG="en" ;;
    esac
}

is_zh() {
    [[ "${INSTALL_LANG:-en}" == "zh" ]]
}

say() {
    local zh_text="$1" en_text="$2"
    shift 2
    if is_zh; then
        printf "$zh_text" "$@"
    else
        printf "$en_text" "$@"
    fi
}

die() {
    say "${red}错误：%s${plain}\n" "${red}Error: %s${plain}\n" "$1" >&2
    exit 1
}

restore_terminal() {
    if [[ "$TTY_ECHO_DISABLED" -eq 1 ]]; then
        stty echo </dev/tty >/dev/null 2>&1 || true
        TTY_ECHO_DISABLED=0
    fi
}

trap 'restore_terminal; exit 130' INT TERM HUP
trap 'restore_terminal' EXIT

read_secret() {
    local prompt="$1" variable_name="$2" value=""
    if [[ ! -t 0 || ! -e /dev/tty ]]; then
        say '错误：交互式密码输入需要 TTY，非交互模式已安全拒绝。\n' 'Error: interactive password input requires a TTY; non-interactive mode was safely rejected.\n' >&2
        return 1
    fi
    printf '%s' "$prompt" >/dev/tty
    if ! stty -echo </dev/tty; then
        say '错误：无法关闭终端回显。\n' 'Error: failed to disable terminal echo.\n' >&2
        return 1
    fi
    TTY_ECHO_DISABLED=1
    if ! IFS= read -r -s value </dev/tty; then
        restore_terminal
        printf '\n' >/dev/tty
        say '错误：读取密码失败。\n' 'Error: failed to read password.\n' >&2
        return 1
    fi
    restore_terminal
    printf '\n' >/dev/tty
    printf -v "$variable_name" '%s' "$value"
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
    command -v realpath >/dev/null 2>&1 || die '需要 GNU coreutils 的 realpath 命令 / GNU realpath from coreutils is required.'
    realpath -m -- / >/dev/null 2>&1 || die '需要支持 -m 的 GNU realpath / GNU realpath with -m support is required.'
    [[ -n "$raw" && "$raw" == /* ]] || die '安装目录必须是绝对路径 / installation directory must be absolute.'
    [[ "$raw" =~ ^[A-Za-z0-9._/-]+$ ]] || die '安装目录包含不安全字符；仅允许字母、数字、点、下划线、连字符和 /。'
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

assert_no_symlinks_below() {
    local directory="$1" label="$2" first_link=""
    [[ -e "$directory" || -L "$directory" ]] || return 0
    [[ -d "$directory" && ! -L "$directory" ]] || die "$label 不是安全目录或是符号链接 / $label is not a safe directory or is a symlink."
    first_link="$(find -P "$directory" -xdev -type l -print -quit 2>/dev/null || true)"
    [[ -z "$first_link" ]] || die "$label 包含符号链接：$first_link / $label contains a symlink."
}

assert_safe_regular_file() {
    local file="$1" label="$2"
    [[ -e "$file" || -L "$file" ]] || return 0
    [[ -f "$file" && ! -L "$file" ]] || die "$label 必须是普通文件且不能是符号链接 / $label must be a regular non-symlink file."
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

ensure_runtime_dependencies() {
    local -a packages=()
    local command_name
    for command_name in openssl timeout ss realpath; do
        command -v "$command_name" >/dev/null 2>&1 || {
            case "$command_name" in
                openssl) packages+=(openssl) ;;
                timeout|realpath) packages+=(coreutils) ;;
                ss) packages+=(iproute2) ;;
            esac
        }
    done
    command -v update-ca-certificates >/dev/null 2>&1 || packages+=(ca-certificates)
    if (( ${#packages[@]} > 0 )); then
        if command -v apt-get >/dev/null 2>&1; then
            say '正在安装 Ubuntu/Debian 运行依赖：%s\n' 'Installing Ubuntu/Debian runtime dependencies: %s\n' "${packages[*]}"
            apt-get update
            DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"
        else
            die '缺少 Ubuntu 运行依赖，且未找到 apt-get。 / Missing Ubuntu runtime dependencies and apt-get is unavailable.'
        fi
    fi
    for command_name in openssl timeout ss realpath stat getent groupadd useradd; do
        command -v "$command_name" >/dev/null 2>&1 || die "缺少必要命令：$command_name / required command is missing: $command_name."
    done
}

require_real_systemd() {
    if ! command -v systemctl >/dev/null 2>&1 ||
       [[ ! -d /run/systemd/system ]] ||
       [[ ! -r /proc/1/comm ]] ||
       [[ "$(cat /proc/1/comm)" != "systemd" ]] ||
       ! systemctl show-environment >/dev/null 2>&1; then
        die '需要正在运行的真实 systemd（PID 1）才能安全安装。 / A real running systemd instance (PID 1) is required for safe installation.'
    fi
}

platform_arch() {
    case "$(uname -m)" in
        x86_64|x64|amd64) printf 'amd64' ;;
        arm64|aarch64) printf 'arm64' ;;
        armv7*|armv6*|arm) printf 'arm' ;;
        *) die "不支持的 CPU 架构：$(uname -m) / unsupported CPU architecture." ;;
    esac
}

normalize_url_path() {
    local value="$1"
    [[ "$value" != *$'\n'* && "$value" != *$'\r'* ]] || die '路径不能包含换行符 / path cannot contain line breaks.'
    [[ -n "$value" ]] || { printf '/'; return; }
    [[ "${value:0:1}" == / ]] || value="/$value"
    [[ "${value: -1}" == / ]] || value="$value/"
    printf '%s' "$value"
}

validate_port() {
    local port="$1" label="$2"
    [[ "$port" =~ ^[0-9]{1,5}$ ]] && (( 10#$port >= 1 && 10#$port <= 65535 )) || die "$label 端口无效：$port / invalid $label port: $port."
}

run_sui_required() {
    local zh_desc="$1" en_desc="$2" output=""
    shift 2
    if ! output="$("$INSTALL_DIR/sui" "$@" 2>&1)"; then
        say '错误：%s失败。\n' 'Error: %s failed.\n' "$zh_desc" "$en_desc"
        [[ -z "$output" ]] || printf '%s\n' "$output"
        exit 1
    fi
    [[ -z "$output" ]] || printf '%s\n' "$output"
}

port_is_available() {
    local port="$1"
    ! ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .
}

wait_for_tcp_listener() {
    local port="$1" attempt
    for attempt in 1 2 3 4 5 6 7 8 9 10; do
        if ss -H -ltn "sport = :$port" 2>/dev/null | grep -q .; then
            return 0
        fi
        sleep 1
    done
    return 1
}

xray_vless_inbound_config_present() {
    # xray-xhttp.json is generated only while an Xray VLESS inbound is enabled.
    # Do not follow a symlink here: the installation-directory safety checks have
    # already rejected them, and a missing runtime config is not evidence of Xray.
    local xray_config="$INSTALL_DIR/xray-xhttp.json"
    [[ -f "$xray_config" && ! -L "$xray_config" ]] || return 1
    grep -Fq -- '"inbounds"' "$xray_config" &&
        grep -Eq -- '"protocol"[[:space:]]*:[[:space:]]*"vless"' "$xray_config"
}

service_has_default_three_listeners() {
    local port
    [[ "$SERVICE_WAS_ACTIVE" -eq 1 ]] || return 1
    for port in "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"; do
        ss -H -ltn "sport = :$port" 2>/dev/null | grep -q . || return 1
    done
}

upgrade_requires_strict_proxy_mode() {
    # Persist strict mode when either condition provides verifiable evidence that
    # this preserved installation uses embedded Xray/default proxy listeners.
    [[ "$REQUIRE_PROXY_CORES" -eq 1 ]] && return 0
    xray_vless_inbound_config_present || service_has_default_three_listeners
}

reality_target_is_usable() {
    local host="$1" output=""
    if ! output="$(timeout 12 openssl s_client -tls1_3 -connect "$host:443" -servername "$host" -verify_return_error </dev/null 2>&1)"; then
        return 1
    fi
    grep -Eq 'Verify return code: 0 \(ok\)|Verification: OK' <<< "$output"
}

validate_fresh_listener_ports() {
    local i j port
    local -a ports=("$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT" "$PANEL_PORT" "$SUB_PORT")
    validate_port "$PANEL_PORT" 'panel'
    validate_port "$SUB_PORT" 'subscription'
    for ((i = 0; i < ${#ports[@]}; i++)); do
        for ((j = i + 1; j < ${#ports[@]}; j++)); do
            if (( 10#${ports[i]} == 10#${ports[j]} )); then
                die "Fresh-install listener ports must be unique; ${ports[i]} conflicts with ${ports[j]}."
            fi
        done
    done
    for port in "${ports[@]}"; do
        port_is_available "$port" || die "Fresh-install listener port $port is already in use."
    done
}

service_is_inactive() {
    [[ "$(systemctl show -p ActiveState --value "$SERVICE_NAME" 2>/dev/null || true)" == "inactive" ]]
}

unit_belongs_to_install_dir() {
    [[ -f "$UNIT_PATH" && ! -L "$UNIT_PATH" ]] || return 1
    grep -Fqx -- "WorkingDirectory=$INSTALL_DIR" "$UNIT_PATH" &&
        grep -Fqx -- "ExecStart=$INSTALL_DIR/sui" "$UNIT_PATH"
}

manager_belongs_to_install_dir() {
    [[ -f "$INSTALL_DIR/s-ui.sh" && ! -L "$INSTALL_DIR/s-ui.sh" ]] || return 1
    [[ -f /usr/local/bin/s-ui && ! -L /usr/local/bin/s-ui ]] || return 1
    cmp -s -- "$INSTALL_DIR/s-ui.sh" /usr/local/bin/s-ui
}

snapshot_service_state() {
    local enabled_state="" fragment=""
    if [[ -e "$UNIT_PATH" || -L "$UNIT_PATH" ]]; then
        [[ -f "$UNIT_PATH" && ! -L "$UNIT_PATH" ]] || die '现有 systemd unit 不是安全的普通文件。 / Existing systemd unit is not a safe regular file.'
        unit_belongs_to_install_dir || die '现有 systemd unit 的 ExecStart/WorkingDirectory 不属于目标目录，已拒绝覆盖。 / Existing systemd unit does not belong to the target directory; refusing to overwrite it.'
        fragment="$(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || true)"
        [[ -z "$fragment" || "$fragment" == "$UNIT_PATH" ]] || die 'systemd 已加载同名但不同位置的 unit，已拒绝操作。 / systemd loaded a same-name unit from a different location; refusing to operate.'
        SERVICE_UNIT_EXISTED=1
        systemctl is-active --quiet "$SERVICE_NAME" && SERVICE_WAS_ACTIVE=1 || true
        enabled_state="$(systemctl is-enabled "$SERVICE_NAME" 2>/dev/null || true)"
        case "$enabled_state" in
            enabled|enabled-runtime|linked|linked-runtime|alias) SERVICE_WAS_ENABLED=1 ;;
            *) SERVICE_WAS_ENABLED=0 ;;
        esac
    else
        fragment="$(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || true)"
        [[ -z "$fragment" ]] || die '检测到同名 systemd unit 但不在 /etc/systemd/system，已拒绝操作。 / A same-name systemd unit exists outside /etc/systemd/system; refusing to operate.'
    fi
}

ensure_service_account() {
    local existing_group="" uid=""
    if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
        groupadd --system "$SERVICE_GROUP"
    fi
    if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
        useradd --system --gid "$SERVICE_GROUP" --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin --no-create-home "$SERVICE_USER"
    fi
    uid="$(id -u "$SERVICE_USER" 2>/dev/null || true)"
    [[ "$uid" =~ ^[0-9]+$ && "$uid" != "0" ]] || die "Service account $SERVICE_USER must not have UID 0."
    existing_group="$(id -gn "$SERVICE_USER" 2>/dev/null || true)"
    [[ "$existing_group" == "$SERVICE_GROUP" ]] || die "System user $SERVICE_USER does not use primary group $SERVICE_GROUP."
}

prepare_runtime_directories() {
    install -d -m 0750 -- "$INSTALL_DIR"
    install -d -m 0750 -- "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/cert"
    install -d -m 0700 -- "$INSTALL_DIR/backups"
    chown -h root:"$SERVICE_GROUP" -- "$INSTALL_DIR"
    chown -h "$SERVICE_USER":"$SERVICE_GROUP" -- "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/cert"
    chown -h root:root -- "$INSTALL_DIR/backups"
    assert_no_symlinks_below "$INSTALL_DIR/db" '数据库目录 / database directory'
    assert_no_symlinks_below "$INSTALL_DIR/logs" '日志目录 / log directory'
    assert_no_symlinks_below "$INSTALL_DIR/cert" '证书目录 / certificate directory'
}

apply_runtime_permissions() {
    local directory
    for directory in "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/cert"; do
        assert_no_symlinks_below "$directory" "$directory"
        find -P "$directory" -xdev -exec chown -h "$SERVICE_USER":"$SERVICE_GROUP" -- {} +
    done
    find -P "$INSTALL_DIR/db" -xdev -type d -exec chmod 0750 -- {} +
    find -P "$INSTALL_DIR/db" -xdev -type f -exec chmod 0600 -- {} +
    find -P "$INSTALL_DIR/logs" -xdev -type d -exec chmod 0750 -- {} +
    find -P "$INSTALL_DIR/logs" -xdev -type f -exec chmod 0640 -- {} +
    find -P "$INSTALL_DIR/cert" -xdev -type d -exec chmod 0750 -- {} +
    find -P "$INSTALL_DIR/cert" -xdev -type f -exec chmod 0600 -- {} +
    [[ ! -L "$INSTALL_DIR/sui" ]] || die '主程序不能是符号链接 / executable must not be a symlink.'
    chown -h root:root -- "$INSTALL_DIR/sui"
    chmod 0755 -- "$INSTALL_DIR/sui"
}

write_install_state() {
    local state="$1" temp_file=""
    [[ ! -L "$INSTALL_STATE_FILE" ]] || die '安装状态文件不能是符号链接 / install-state file must not be a symlink.'
    temp_file="$(mktemp "$INSTALL_DIR/.s-ui-install-state.XXXXXX")"
    printf 'STATE=%s\nINSTALL_DIR=%s\n' "$state" "$INSTALL_DIR" > "$temp_file"
    chmod 0600 -- "$temp_file"
    chown -h root:root -- "$temp_file"
    mv -f -- "$temp_file" "$INSTALL_STATE_FILE"
    INSTALL_STATE_WRITTEN=1
}

write_installer_env() {
    local temp_file=""
    [[ ! -L "$CONFIG_FILE" ]] || die '安装配置文件不能是符号链接 / installer configuration file must not be a symlink.'
    temp_file="$(mktemp "$INSTALL_DIR/.s-ui.env.XXXXXX")"
    cat > "$temp_file" <<EOF
INSTALL_DIR=$INSTALL_DIR
SERVICE_NAME=$SERVICE_NAME
PANEL_PORT=$PANEL_PORT
PANEL_PATH=$PANEL_PATH
SUB_PORT=$SUB_PORT
SUB_PATH=$SUB_PATH
XRAY_AUTOCONFIGURED=$XRAY_AUTOCONFIGURED
REQUIRE_PROXY_CORES=$REQUIRE_PROXY_CORES
EOF
    chmod 0600 -- "$temp_file"
    chown -h root:root -- "$temp_file"
    mv -f -- "$temp_file" "$CONFIG_FILE"
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

remove_ufw_rules_with_comment() {
    local comment="$1" index number failure=0
    local -a numbers=()
    command -v ufw >/dev/null 2>&1 || return 1
    mapfile -t numbers < <(ufw_rule_numbers_for_comment "$comment")
    for ((index=${#numbers[@]} - 1; index >= 0; index--)); do
        number="${numbers[$index]}"
        [[ "$number" =~ ^[0-9]+$ ]] || { failure=1; continue; }
        if ! ufw --force delete "$number" >/dev/null 2>&1; then
            failure=1
        fi
    done
    return "$failure"
}

write_ufw_state() {
    local temp_file=""
    [[ ! -e "$UFW_STATE_FILE" && ! -L "$UFW_STATE_FILE" ]] || die 'UFW 状态文件已存在，拒绝覆盖。 / UFW state file already exists; refusing to overwrite.'
    temp_file="$(mktemp "$INSTALL_DIR/.s-ui-ufw-state.XXXXXX")"
    printf 'COMMENT=%s\nPORTS=%s %s %s\n' "$UFW_COMMENT" "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT" > "$temp_file"
    chmod 0600 -- "$temp_file"
    chown -h root:root -- "$temp_file"
    mv -f -- "$temp_file" "$UFW_STATE_FILE"
}

configure_active_ufw() {
    local port after_numbers="" added_any=0 output=""
    command -v ufw >/dev/null 2>&1 || return 0
    ufw status 2>/dev/null | grep -q '^Status: active' || return 0
    UFW_COMMENT="s-ui-go-$(openssl rand -hex 12)"
    for port in "$DEFAULT_SING_REALITY_PORT" "$DEFAULT_XRAY_REALITY_PORT" "$DEFAULT_XRAY_XHTTP_PORT"; do
        if ! output="$(ufw allow "$port/tcp" comment "$UFW_COMMENT" 2>&1)"; then
            remove_ufw_rules_with_comment "$UFW_COMMENT" || true
            say '警告：无法在 UFW 中添加带唯一标记的 TCP %s 规则：%s\n' 'Warning: failed to add uniquely marked UFW TCP %s rule: %s\n' "$port" "$output"
            return 1
        fi
        after_numbers="$(ufw_rule_numbers_for_comment "$UFW_COMMENT")"
        [[ -z "$after_numbers" ]] || added_any=1
    done
    if [[ "$added_any" -eq 1 ]]; then
        if ! write_ufw_state; then
            remove_ufw_rules_with_comment "$UFW_COMMENT" || true
            return 1
        fi
        UFW_RULES_ADDED_THIS_RUN=1
        say '已在活动 UFW 中添加本安装实例专属的 Reality TCP 规则。\n' 'Added installation-specific Reality TCP rules to active UFW.\n'
    else
        say '活动 UFW 已有相应规则；本安装器未新增规则，也不会在卸载时删除现有规则。\n' 'Active UFW already has matching rules; this installer added none and will not remove existing rules on uninstall.\n'
    fi
}

copy_bundle_file() {
    local source_file="$SCRIPT_DIR/$1" target_file="$INSTALL_DIR/$1"
    [[ -f "$source_file" && ! -L "$source_file" ]] || die "Release package is missing a safe $1."
    assert_safe_regular_file "$target_file" "Local release target $target_file"
    if [[ "$source_file" == "$target_file" ]]; then
        return 0
    fi
    install -o root -g root -m 0755 -- "$source_file" "$target_file"
}

install_manager() {
    local source_file="$INSTALL_DIR/s-ui.sh"
    [[ -f "$source_file" && ! -L "$source_file" ]] || die 'Release package is missing a safe s-ui.sh manager.'
    assert_safe_regular_file /usr/local/bin/s-ui '/usr/local/bin/s-ui'
    install -o root -g root -m 0755 -- "$source_file" /usr/local/bin/s-ui
}

create_upgrade_backup() {
    local state_file=""
    BACKUP_DIR="$INSTALL_DIR/backups/pre-upgrade-$(date +%Y%m%d-%H%M%S)-$$"
    install -d -m 0700 -- "$BACKUP_DIR"
    assert_safe_regular_file "$INSTALL_DIR/sui" 'existing executable'
    [[ -f "$INSTALL_DIR/sui" ]] && cp -a -- "$INSTALL_DIR/sui" "$BACKUP_DIR/sui"
    if [[ -e "$INSTALL_DIR/db" || -L "$INSTALL_DIR/db" ]]; then
        assert_no_symlinks_below "$INSTALL_DIR/db" '数据库目录 / database directory'
        cp -a -- "$INSTALL_DIR/db" "$BACKUP_DIR/db"
    fi
    for _file in "$CONFIG_FILE" "$INSTALL_STATE_FILE" "$INSTALL_DIR/install.sh" "$INSTALL_DIR/install-linux.sh" "$INSTALL_DIR/uninstall-linux.sh" "$INSTALL_DIR/s-ui.sh" /usr/local/bin/s-ui; do
        if [[ -e "$_file" || -L "$_file" ]]; then
            assert_safe_regular_file "$_file" "备份文件 / backup file"
            cp -a -- "$_file" "$BACKUP_DIR/$(basename "$_file")"
        fi
    done
    if [[ "$SERVICE_UNIT_EXISTED" -eq 1 ]]; then
        cp -a -- "$UNIT_PATH" "$BACKUP_DIR/s-ui.service"
    fi
    state_file="$BACKUP_DIR/service-state"
    cat > "$state_file" <<EOF
UNIT_EXISTED=$SERVICE_UNIT_EXISTED
WAS_ACTIVE=$SERVICE_WAS_ACTIVE
WAS_ENABLED=$SERVICE_WAS_ENABLED
EOF
    chmod 0600 -- "$state_file"
    BACKUP_READY=1
}

rollback_error() {
    say "${red}回滚恢复失败：%s${plain}\n" "${red}Rollback restoration failed: %s${plain}\n" "$1" >&2
    ROLLBACK_FAILED=1
}

restore_file_from_backup() {
    local name="$1" destination="$2"
    if [[ -e "$destination" || -L "$destination" ]]; then
        [[ -f "$destination" && ! -L "$destination" ]] || return 1
    fi
    if [[ -f "$BACKUP_DIR/$name" && ! -L "$BACKUP_DIR/$name" ]]; then
        cp -a -- "$BACKUP_DIR/$name" "$destination" || return 1
    else
        rm -f -- "$destination" || return 1
    fi
}

restore_upgrade() {
    ROLLBACK_FAILED=0
    say '安装失败，正在恢复升级前的程序、数据库、unit、环境和服务状态...\n' 'Installation failed; restoring the pre-upgrade executable, database, unit, environment, and service state...\n' >&2
    if ! systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || ! service_is_inactive; then
        rollback_error '无法停止当前服务 / could not stop the current service'
        return 1
    fi
    restore_file_from_backup sui "$INSTALL_DIR/sui" || rollback_error '无法恢复主程序 / executable'
    if [[ -d "$BACKUP_DIR/db" && ! -L "$BACKUP_DIR/db" ]]; then
        rm -rf -- "$INSTALL_DIR/db" || rollback_error '无法清理新的数据库目录 / new database directory'
        cp -a -- "$BACKUP_DIR/db" "$INSTALL_DIR/db" || rollback_error '无法恢复数据库 / database'
    else
        rm -rf -- "$INSTALL_DIR/db" || rollback_error '无法恢复数据库缺失状态 / database absence state'
    fi
    restore_file_from_backup s-ui.env "$CONFIG_FILE" || rollback_error '无法恢复安装环境文件 / installer environment'
    restore_file_from_backup .s-ui-install-state "$INSTALL_STATE_FILE" || rollback_error '无法恢复安装状态文件 / install-state file'
    restore_file_from_backup install.sh "$INSTALL_DIR/install.sh" || rollback_error '无法恢复本地 install.sh / local install.sh'
    restore_file_from_backup install-linux.sh "$INSTALL_DIR/install-linux.sh" || rollback_error '无法恢复本地 install-linux.sh / local install-linux.sh'
    restore_file_from_backup uninstall-linux.sh "$INSTALL_DIR/uninstall-linux.sh" || rollback_error '无法恢复本地 uninstall-linux.sh / local uninstall-linux.sh'
    restore_file_from_backup s-ui.sh "$INSTALL_DIR/s-ui.sh" || rollback_error '无法恢复本地 s-ui.sh / local s-ui.sh'
    restore_file_from_backup s-ui /usr/local/bin/s-ui || rollback_error '无法恢复管理器 / manager'
    if [[ "$SERVICE_UNIT_EXISTED" -eq 1 ]]; then
        cp -a -- "$BACKUP_DIR/s-ui.service" "$UNIT_PATH" || rollback_error 'unable to restore systemd unit'
    elif [[ -e "$UNIT_PATH" || -L "$UNIT_PATH" ]]; then
        if [[ ! -f "$UNIT_PATH" || -L "$UNIT_PATH" ]] || ! unit_belongs_to_install_dir; then
            rollback_error 'refusing to remove an unexpected systemd unit'
        else
            systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || rollback_error 'unable to remove new enabled state'
            rm -f -- "$UNIT_PATH" || rollback_error 'unable to remove new systemd unit'
        fi
    fi
    systemctl daemon-reload >/dev/null 2>&1 || rollback_error '无法重新加载 systemd / reload systemd'
    if [[ "$SERVICE_UNIT_EXISTED" -eq 1 ]]; then
        if [[ "$SERVICE_WAS_ENABLED" -eq 1 ]]; then
            systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || rollback_error '无法恢复 enabled 状态 / enabled state'
        else
            systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || rollback_error '无法恢复 disabled 状态 / disabled state'
        fi
        if [[ "$SERVICE_WAS_ACTIVE" -eq 1 ]]; then
            if ! systemctl start "$SERVICE_NAME" >/dev/null 2>&1 || ! systemctl is-active --quiet "$SERVICE_NAME"; then
                rollback_error '无法恢复 active 状态 / active state'
            fi
        elif ! systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || ! service_is_inactive; then
            rollback_error '无法恢复 inactive 状态 / inactive state'
        fi
    fi
    [[ "$ROLLBACK_FAILED" -eq 0 ]] || say "${red}严重：升级失败且回滚未完全恢复；请立即检查上述错误。${plain}\n" "${red}Critical: upgrade failed and rollback was not fully restored; inspect the errors above immediately.${plain}\n" >&2
}

rollback_fresh_install() {
    local cleanup_failed=0 fragment=""
    say 'Installation failed; removing this fresh install state and installer-managed firewall rules...\n' 'Installation failed; removing this fresh install state and installer-managed firewall rules...\n' >&2
    if [[ -n "$UFW_COMMENT" ]]; then
        remove_ufw_rules_with_comment "$UFW_COMMENT" || cleanup_failed=1
    fi
    if [[ -e "$UNIT_PATH" || -L "$UNIT_PATH" ]]; then
        if [[ ! -f "$UNIT_PATH" || -L "$UNIT_PATH" ]] || ! unit_belongs_to_install_dir; then
            cleanup_failed=1
        else
            if ! systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || ! service_is_inactive; then
                cleanup_failed=1
                say '服务未停止；保留文件和数据库，拒绝破坏性清理。\n' 'Service not stopped; preserving files and database.\n' >&2
                return 1
            fi
            systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || cleanup_failed=1
            rm -f -- "$UNIT_PATH" || cleanup_failed=1
            systemctl daemon-reload >/dev/null 2>&1 || cleanup_failed=1
        fi
    else
        fragment="$(systemctl show -p FragmentPath --value "$SERVICE_NAME" 2>/dev/null || true)"
        [[ -z "$fragment" ]] || cleanup_failed=1
    fi
    if manager_belongs_to_install_dir; then
        rm -f -- /usr/local/bin/s-ui || cleanup_failed=1
    fi
    rm -f -- "$INSTALL_DIR/sui" "$INSTALL_DIR/install.sh" "$INSTALL_DIR/install-linux.sh" "$INSTALL_DIR/uninstall-linux.sh" "$INSTALL_DIR/s-ui.sh" || cleanup_failed=1
    rm -rf -- "$INSTALL_DIR/db" || cleanup_failed=1
    rm -f -- "$INSTALL_DIR/xray-xhttp.json" "$CONFIG_FILE" "$INSTALL_STATE_FILE" "$UFW_STATE_FILE" || cleanup_failed=1
    if [[ "$cleanup_failed" -ne 0 ]]; then
        say "${red}Critical: fresh-install cleanup was incomplete; inspect systemd, UFW, the install directory, and /usr/local/bin/s-ui manually.${plain}\n" "${red}Critical: fresh-install cleanup was incomplete; inspect systemd, UFW, the install directory, and /usr/local/bin/s-ui manually.${plain}\n" >&2
    fi
}

rollback_on_exit() {
    local status=$?
    restore_terminal
    trap - EXIT
    set +e
    if [[ "$status" -ne 0 ]]; then
        if [[ "$FRESH_INSTALL" -eq 0 && "$BACKUP_READY" -eq 1 ]]; then
            restore_upgrade
        elif [[ "$FRESH_INSTALL" -eq 0 && "$UPGRADE_SERVICE_STOPPED" -eq 1 && "$SERVICE_WAS_ACTIVE" -eq 1 ]]; then
            systemctl start "$SERVICE_NAME" || say '备份失败，恢复原服务启动也失败。\n' 'Backup failed; could not restart original service.\n' >&2
        elif [[ "$FRESH_INSTALL" -eq 1 && "$INSTALL_STATE_WRITTEN" -eq 1 ]]; then
            rollback_fresh_install
        fi
    fi
    exit "$status"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --lang=*) SUI_LANG="${1#*=}" ;;
        --lang|-l)
            [[ $# -ge 2 ]] || { printf 'Missing language value / 缺少语言参数\n' >&2; exit 1; }
            SUI_LANG="$2"
            shift
            ;;
        --install-dir=*) INSTALL_DIR="${1#*=}"; INSTALL_DIR_FIXED=1 ;;
        --install-dir)
            [[ $# -ge 2 ]] || { printf 'Missing installation directory / 缺少安装目录参数\n' >&2; exit 1; }
            INSTALL_DIR="$2"
            INSTALL_DIR_FIXED=1
            shift
            ;;
        --upgrade) UPGRADE_ONLY=1 ;;
        --help|-h)
            printf 'Usage: install-linux.sh [--lang zh|en] [--install-dir /usr/local/name|/opt/name|/srv/name] [--upgrade]\n'
            exit 0
            ;;
        *)
            printf 'Unsupported option / 不支持的参数: %s\n' "$1" >&2
            exit 1
            ;;
    esac
    shift
done

select_install_language
printf '========================================\n'
say 's-ui-go Linux 安装程序\n' 's-ui-go Linux Installer\n'
printf '========================================\n\n'

[[ "${EUID:-$(id -u)}" -eq 0 ]] || die '请使用 root 权限运行，例如：sudo ./install-linux.sh / run as root, for example: sudo ./install-linux.sh.'
ensure_runtime_dependencies
require_real_systemd

SCRIPT_DIR="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
ARCH="$(platform_arch)"
BUNDLE_BINARY="$SCRIPT_DIR/sui"
[[ -f "$BUNDLE_BINARY" && ! -L "$BUNDLE_BINARY" ]] || die "The release package has no safe top-level Linux executable for $ARCH."


if [[ "$INSTALL_DIR_FIXED" -eq 0 && -t 0 ]]; then
    if is_zh; then
        printf '安装目录，默认 %s：' "$INSTALL_DIR"
    else
        printf 'Installation directory, default %s: ' "$INSTALL_DIR"
    fi
    IFS= read -r input_install_dir || die '读取安装目录失败 / failed to read installation directory.'
    [[ -z "$input_install_dir" ]] || INSTALL_DIR="$input_install_dir"
fi
normalize_install_dir "$INSTALL_DIR"
CONFIG_FILE="$INSTALL_DIR/s-ui.env"
INSTALL_STATE_FILE="$INSTALL_DIR/.s-ui-install-state"
UFW_STATE_FILE="$INSTALL_DIR/.s-ui-ufw-state"

assert_no_symlinks_below "$INSTALL_DIR" 'installation directory'
assert_safe_regular_file "$CONFIG_FILE" 'installer configuration file'
assert_root_private_file "$INSTALL_STATE_FILE" 'install-state file'
assert_root_private_file "$UFW_STATE_FILE" 'UFW state file'


STATE_VALUE=""
STATE_INSTALL_DIR=""
if [[ -f "$INSTALL_STATE_FILE" ]]; then
    STATE_VALUE="$(sed -n 's/^STATE=//p' "$INSTALL_STATE_FILE")"
    STATE_INSTALL_DIR="$(sed -n 's/^INSTALL_DIR=//p' "$INSTALL_STATE_FILE")"
    case "$STATE_VALUE" in
        ready|installing) ;;
        *) die 'Install-state file is invalid; refusing to continue.' ;;
    esac
    [[ "$STATE_INSTALL_DIR" == "$INSTALL_DIR" ]] || die 'Install-state directory does not match the requested installation directory.'
fi

DB_EXISTS=0
if [[ -e "$INSTALL_DIR/db" || -L "$INSTALL_DIR/db" ]]; then
    assert_no_symlinks_below "$INSTALL_DIR/db" '数据库目录 / database directory'
    [[ -f "$INSTALL_DIR/db/s-ui.db" && ! -L "$INSTALL_DIR/db/s-ui.db" ]] && DB_EXISTS=1
fi
# Never classify unknown pre-existing data as a fresh installation: its rollback
# is allowed to remove only a database directory created by this installer.
if [[ "$DB_EXISTS" -eq 0 && -e "$INSTALL_DIR/db" ]]; then
    [[ -d "$INSTALL_DIR/db" ]] || die 'Database path exists but is not a directory; refusing to continue.'
    [[ -z "$(find "$INSTALL_DIR/db" -mindepth 1 -maxdepth 1 -print -quit)" ]] || die 'Unrecognized nonempty database directory; refusing to overwrite or delete existing data.'
fi
if [[ "$STATE_VALUE" == "installing" ]]; then
    die '检测到一次中断的新安装，拒绝将半初始化状态当作升级。请先审查或使用卸载器清理目标目录后重试。 / Interrupted fresh installation detected; refusing to treat a partial state as an upgrade. Review it or use the uninstaller to clean the target directory, then retry.'
fi
if [[ "$STATE_VALUE" == "ready" && "$DB_EXISTS" -ne 1 ]]; then
    die '安装状态显示已完成，但数据库缺失；已拒绝继续。 / Install state is ready but the database is missing; refusing to continue.'
fi
if [[ "$DB_EXISTS" -eq 1 ]]; then
    FRESH_INSTALL=0
    [[ -n "$STATE_VALUE" ]] || LEGACY_INSTALL=1
else
    FRESH_INSTALL=1
fi
if [[ "$UPGRADE_ONLY" -eq 1 && "$FRESH_INSTALL" -eq 1 ]]; then
    die "--upgrade 指定的目录中没有可升级数据库：$INSTALL_DIR / --upgrade was requested but no upgradeable database exists in: $INSTALL_DIR."
fi

if [[ -f "$CONFIG_FILE" ]]; then
    while IFS='=' read -r key value; do
        case "$key" in
            PANEL_PORT) PANEL_PORT="$value" ;;
            PANEL_PATH) PANEL_PATH="$value" ;;
            SUB_PORT) SUB_PORT="$value" ;;
            SUB_PATH) SUB_PATH="$value" ;;
            XRAY_AUTOCONFIGURED) XRAY_AUTOCONFIGURED="$value" ;;
            REQUIRE_PROXY_CORES) REQUIRE_PROXY_CORES="$value" ;;
        esac
    done < "$CONFIG_FILE"
fi
case "$XRAY_AUTOCONFIGURED" in 1) ;; *) XRAY_AUTOCONFIGURED=0 ;; esac
case "$REQUIRE_PROXY_CORES" in 1) ;; *) REQUIRE_PROXY_CORES=0 ;; esac
if [[ -f "$UNIT_PATH" && ! -L "$UNIT_PATH" ]] && grep -Fqx -- 'Environment=SUI_REQUIRE_PROXY_CORES=1' "$UNIT_PATH"; then
    REQUIRE_PROXY_CORES=1
fi

snapshot_service_state
if [[ "$FRESH_INSTALL" -eq 0 ]] && upgrade_requires_strict_proxy_mode; then
    REQUIRE_PROXY_CORES=1
    STRICT_PROXY_HEALTH=1
    if service_has_default_three_listeners; then
        DEFAULT_THREE_LISTENER_CONFIG=1
    fi
    say '检测到启用的 Xray 入站或旧的默认三监听运行态；本次升级将持久化严格代理核心模式并验证后才写入 ready。\n' 'Detected an enabled Xray inbound or legacy default three-listener runtime; this upgrade will persist strict proxy-core mode and verify it before writing ready.\n'
fi
if [[ "$FRESH_INSTALL" -eq 1 && "$SERVICE_UNIT_EXISTED" -eq 1 ]]; then
    die '新安装目标已存在同名 systemd 服务；拒绝覆盖可能残留或其他实例。 / Fresh-install target already has a same-name systemd service; refusing to overwrite a possible remnant or another instance.'
fi

if [[ "$FRESH_INSTALL" -eq 1 ]]; then
    if [[ -t 0 ]]; then
        if is_zh; then printf '面板端口，默认 %s：' "$PANEL_PORT"; else printf 'Panel port, default %s: ' "$PANEL_PORT"; fi
        IFS= read -r input_value || die '读取面板端口失败 / failed to read panel port.'; [[ -z "$input_value" ]] || PANEL_PORT="$input_value"
        if is_zh; then printf '面板路径，默认 %s：' "$PANEL_PATH"; else printf 'Panel path, default %s: ' "$PANEL_PATH"; fi
        IFS= read -r input_value || die '读取面板路径失败 / failed to read panel path.'; [[ -z "$input_value" ]] || PANEL_PATH="$input_value"
        if is_zh; then printf '订阅端口，默认 %s：' "$SUB_PORT"; else printf 'Subscription port, default %s: ' "$SUB_PORT"; fi
        IFS= read -r input_value || die '读取订阅端口失败 / failed to read subscription port.'; [[ -z "$input_value" ]] || SUB_PORT="$input_value"
        if is_zh; then printf '订阅路径，默认 %s：' "$SUB_PATH"; else printf 'Subscription path, default %s: ' "$SUB_PATH"; fi
        IFS= read -r input_value || die '读取订阅路径失败 / failed to read subscription path.'; [[ -z "$input_value" ]] || SUB_PATH="$input_value"
        if is_zh; then printf '管理员用户名，默认 admin：'; else printf 'Admin username, default admin: '; fi
        IFS= read -r ADMIN_USERNAME || die '读取管理员用户名失败 / failed to read admin username.'
    else
        ADMIN_USERNAME="admin"
    fi
    PANEL_PATH="$(normalize_url_path "$PANEL_PATH")"
    SUB_PATH="$(normalize_url_path "$SUB_PATH")"
    ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
    validate_port "$PANEL_PORT" '面板 / panel'
    validate_port "$SUB_PORT" '订阅 / subscription'
    if is_zh; then secret_prompt='管理员密码：'; else secret_prompt='Admin password: '; fi
    if ! read_secret "$secret_prompt" ADMIN_PASSWORD; then
        exit 1
    fi
    [[ -n "$ADMIN_PASSWORD" ]] || die '密码不能为空 / password cannot be empty.'
    validate_fresh_listener_ports
    reality_target_is_usable "$DEFAULT_REALITY_TARGET_HOST" || die "Reality 目标 ${DEFAULT_REALITY_TARGET_HOST}:443 未通过 TLS 1.3 验证。 / Reality target ${DEFAULT_REALITY_TARGET_HOST}:443 did not pass TLS 1.3 verification."
    XRAY_AUTOCONFIGURED=1
    REQUIRE_PROXY_CORES=1
    STRICT_PROXY_HEALTH=1
    DEFAULT_THREE_LISTENER_CONFIG=1
    export SUI_AUTOCONFIGURE_XRAY_REALITY=1
    say 'Reality 目标已验证；将新建 sing-box 与嵌入式 Xray 的三监听默认配置。\n' 'Reality target verified; creating the fresh sing-box and embedded-Xray three-listener defaults.\n'
else
    say '检测到现有数据库；将保留面板、订阅、管理员和协议配置。\n' 'Existing database detected; panel, subscription, administrator, and protocol settings will be preserved.\n'
    [[ "$LEGACY_INSTALL" -eq 0 ]] || say '这是无 install-state 标记的旧安装；兼容升级，成功后会写入 ready 标记。\n' 'This is a legacy installation without an install-state marker; it will be upgraded compatibly and receive a ready marker on success.\n'
    REQUIRE_PROXY_CORES=1
    STRICT_PROXY_HEALTH=1
fi

trap rollback_on_exit EXIT

if [[ "$FRESH_INSTALL" -eq 0 ]]; then
    install -d -m 0700 -- "$INSTALL_DIR/backups"
    chown -h root:root -- "$INSTALL_DIR/backups"
    if ! systemctl stop "$SERVICE_NAME" || ! service_is_inactive; then
        die '无法停止并确认现有服务已 inactive，已中止升级以保护数据库。 / Could not stop the existing service and confirm inactive state; upgrade aborted to protect the database.'
    fi
    UPGRADE_SERVICE_STOPPED=1
    create_upgrade_backup
fi

ensure_service_account
prepare_runtime_directories
if [[ "$FRESH_INSTALL" -eq 1 ]]; then
    write_install_state installing
fi

say '正在复制主程序和已验证的本地安装脚本...\n' 'Copying executable and verified local installer scripts...\n'
assert_safe_regular_file "$INSTALL_DIR/sui" 'existing executable target'
install -o root -g root -m 0755 -- "$BUNDLE_BINARY" "$INSTALL_DIR/sui"
for _script in install.sh install-linux.sh uninstall-linux.sh s-ui.sh; do
    [[ -f "$SCRIPT_DIR/$_script" && ! -L "$SCRIPT_DIR/$_script" ]] || die "发布包缺少安全的 $_script；无法提供可信的本地更新/卸载链。 / Release package is missing safe $_script, so a trusted local update/uninstall chain cannot be provided."
    copy_bundle_file "$_script"
done

say '正在执行数据库迁移...\n' 'Running database migration...\n'
cd -- "$INSTALL_DIR"
run_sui_required '数据库迁移' 'database migration' migrate
if [[ "$FRESH_INSTALL" -eq 1 ]]; then
    say '正在应用面板、订阅和管理员配置...\n' 'Applying panel, subscription, and administrator configuration...\n'
    run_sui_required '应用面板和订阅配置' 'applying panel and subscription settings' setting -port "$PANEL_PORT" -path "$PANEL_PATH" -subPort "$SUB_PORT" -subPath "$SUB_PATH"
    run_sui_required '设置管理员账号' 'setting admin credentials' admin -username "$ADMIN_USERNAME" -password "$ADMIN_PASSWORD"
    unset ADMIN_PASSWORD
fi

apply_runtime_permissions
write_installer_env
install_manager

REQUIRE_CORES_SYSTEMD_ENV=""
[[ "$REQUIRE_PROXY_CORES" -eq 1 ]] && REQUIRE_CORES_SYSTEMD_ENV='Environment=SUI_REQUIRE_PROXY_CORES=1'
say '正在写入受限 systemd 服务...\n' 'Writing restricted systemd service...\n'
cat > "$UNIT_PATH" <<EOF
[Unit]
Description=s-ui-go Proxy Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/sui
Restart=on-failure
RestartSec=10
UMask=0077
Environment=SUI_DB_FOLDER=db
Environment=SUI_DEBUG=false
$REQUIRE_CORES_SYSTEMD_ENV
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=full
ReadWritePaths=$INSTALL_DIR/db $INSTALL_DIR/logs $INSTALL_DIR/cert

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 -- "$UNIT_PATH"
chown -h root:root -- "$UNIT_PATH"
systemctl daemon-reload

if [[ "$FRESH_INSTALL" -eq 1 || "$SERVICE_WAS_ENABLED" -eq 1 ]]; then
    systemctl enable "$SERVICE_NAME"
fi
if [[ "$FRESH_INSTALL" -eq 1 || "$SERVICE_WAS_ACTIVE" -eq 1 ]]; then
    systemctl start "$SERVICE_NAME"
    systemctl is-active --quiet "$SERVICE_NAME" || die '服务未处于 active 状态，请检查 systemd 日志。 / Service is not active; inspect systemd logs.'
fi
# An upgrade that began inactive must still prove the replacement can start
# before its install-state becomes ready; it is stopped again below to preserve
# the caller's original inactive state. Strict upgrades additionally prove core
# startup, and default three-listener instances prove all three TCP listeners.
if { [[ "$FRESH_INSTALL" -eq 0 && "$SERVICE_WAS_ACTIVE" -eq 0 ]]; } || { [[ "$STRICT_PROXY_HEALTH" -eq 1 ]] && ! systemctl is-active --quiet "$SERVICE_NAME"; }; then
    systemctl start "$SERVICE_NAME"
    TEMPORARILY_STARTED_FOR_HEALTH=1
    systemctl is-active --quiet "$SERVICE_NAME" || die '无法启动服务以验证升级后的运行态。 / Could not start service to validate the upgraded runtime.'
fi

# Type=simple can report active before initialization fails. Require the same
# process to remain active; restarting during this window is an upgrade failure.
_health_pid="$(systemctl show -p MainPID --value "$SERVICE_NAME")"
[[ "$_health_pid" =~ ^[1-9][0-9]*$ ]] || die '服务进程缺失 / Service process is missing.'
for _health_tick in {1..15}; do
    sleep 1
    if ! systemctl is-active --quiet "$SERVICE_NAME" || [[ "$(systemctl show -p MainPID --value "$SERVICE_NAME")" != "$_health_pid" ]]; then
        journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
        die '服务初始化失败或反复重启，升级已中止。 / Service initialization failed or restarted; upgrade aborted.'
    fi
done

if [[ "$STRICT_PROXY_HEALTH" -eq 1 && "$DEFAULT_THREE_LISTENER_CONFIG" -eq 1 ]]; then
    for _name_port in "sing-box Reality TCP:$DEFAULT_SING_REALITY_PORT" "Xray Reality TCP:$DEFAULT_XRAY_REALITY_PORT" "Xray Reality XHTTP:$DEFAULT_XRAY_XHTTP_PORT"; do
        _name="${_name_port%%:*}"
        _port="${_name_port##*:}"
        if ! wait_for_tcp_listener "$_port"; then
            journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
            die "$_name $_port 未监听，严格三监听健康检查失败。 / $_name $_port is not listening; strict three-listener health check failed."
        fi
    done
    say '已通过严格三监听健康检查：sing-box 443、Xray TCP 10443、Xray XHTTP 2088。\n' 'Strict three-listener health check passed: sing-box 443, Xray TCP 10443, and Xray XHTTP 2088.\n'
    if [[ "$FRESH_INSTALL" -eq 1 ]]; then
        configure_active_ufw || die '活动 UFW 事务更新失败；已回滚本安装器新增的规则。 / Active-UFW transaction failed; installer-added rules were rolled back.'
    fi
elif [[ "$STRICT_PROXY_HEALTH" -eq 1 ]]; then
    say '已通过严格代理核心启动验证；未对保留的非默认监听端口作假设。\n' 'Strict proxy-core startup verification passed; no assumptions were made about preserved non-default listener ports.\n'
fi
if [[ "$TEMPORARILY_STARTED_FOR_HEALTH" -eq 1 ]]; then
    if ! systemctl stop "$SERVICE_NAME" || ! service_is_inactive; then
        die '严格健康检查后无法恢复服务 inactive 状态。 / Could not restore inactive service state after strict health check.'
    fi
fi

write_install_state ready
printf '\n========================================\n'
say '安装完成\n' 'Installation completed\n'
printf '========================================\n'
say '安装目录：%s\n' 'Installation directory: %s\n' "$INSTALL_DIR"
if [[ "$STRICT_PROXY_HEALTH" -eq 1 && "$DEFAULT_THREE_LISTENER_CONFIG" -eq 1 ]]; then
    say '本次安装执行并通过了严格三监听端口健康检查。\n' 'This installation executed and passed the strict three-listener port health check.\n'
elif [[ "$STRICT_PROXY_HEALTH" -eq 1 ]]; then
    say '本次安装持久化并验证了严格代理核心模式。\n' 'This installation persisted and verified strict proxy-core mode.\n'
elif [[ "$SERVICE_WAS_ACTIVE" -eq 1 ]]; then
    say '本次升级通过 15 秒进程稳定性检查；保留的旧配置未要求默认端口监听检查。\n' 'This upgrade passed a 15-second process stability check; the preserved legacy configuration did not request default listener checks.\n'
else
    say '服务原本处于 inactive；已临时启动验证升级后的运行态并恢复 inactive。\n' 'The service was inactive; the upgraded runtime was started for validation and then restored to inactive.\n'
fi
say '服务名称：%s，专用用户/组：%s:%s\n' 'Service: %s, dedicated user/group: %s:%s\n' "$SERVICE_NAME" "$SERVICE_USER" "$SERVICE_GROUP"
