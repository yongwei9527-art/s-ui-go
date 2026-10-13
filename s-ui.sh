#!/usr/bin/env bash
set -o pipefail

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

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

normalize_sui_home() {
    local raw="$1" normalized wrapped
    command -v realpath >/dev/null 2>&1 || { echo 'Error: GNU realpath is required.' >&2; exit 1; }
    realpath -m -- / >/dev/null 2>&1 || { echo 'Error: GNU realpath with -m is required.' >&2; exit 1; }
    [[ -n "$raw" && "$raw" == /* ]] || { echo '错误：安装目录必须是绝对路径 / Error: installation directory must be absolute.' >&2; exit 1; }
    [[ "$raw" =~ ^[A-Za-z0-9._/-]+$ ]] || { echo '错误：安装目录包含不安全字符 / Error: installation directory contains unsafe characters.' >&2; exit 1; }
    wrapped="/${raw#/}/"
    [[ "$wrapped" != *'/../'* && "$wrapped" != *'/./'* ]] || { echo '错误：安装目录不得包含路径穿越 / Error: path traversal is not allowed.' >&2; exit 1; }
    path_has_symlink_component "$raw" && { echo '错误：安装目录或父目录不能是符号链接 / Error: installation path cannot contain symlinks.' >&2; exit 1; }
    normalized="$(realpath -m -- "$raw")" || exit 1
    path_has_symlink_component "$normalized" && { echo '错误：规范化目录包含符号链接 / Error: normalized path contains a symlink.' >&2; exit 1; }
    case "$normalized" in
        /usr/local/*|/opt/*|/srv/*) ;;
        *) echo '错误：安装目录仅允许在 /usr/local/*、/opt/* 或 /srv/* / Error: install directory must be under /usr/local/*, /opt/*, or /srv/*.' >&2; exit 1 ;;
    esac
    case "$normalized" in
        /usr/local|/opt|/srv|/usr/local/bin|/usr/local/sbin|/usr/local/lib|/usr/local/lib64|/usr/local/share|/usr/local/include|/usr/local/src)
            echo '错误：安装目录过于宽泛或危险 / Error: installation directory is too broad or dangerous.' >&2
            exit 1
            ;;
    esac
    SUI_HOME="$normalized"
}

SUI_HOME="${SUI_HOME:-}"
if [[ -z "$SUI_HOME" && -f /etc/systemd/system/s-ui.service && ! -L /etc/systemd/system/s-ui.service ]]; then
    _unit_workdir="$(sed -n 's/^WorkingDirectory=//p' /etc/systemd/system/s-ui.service | sed -n '1p')"
    _unit_exec="$(sed -n 's/^ExecStart=//p' /etc/systemd/system/s-ui.service | sed -n '1p')"
    if [[ -n "$_unit_workdir" && "$_unit_exec" == "$_unit_workdir/sui" ]]; then
        SUI_HOME="$_unit_workdir"
    else
        echo 'Error: s-ui.service WorkingDirectory and ExecStart do not point to the same s-ui-go installation; refusing to infer SUI_HOME.' >&2
        exit 1
    fi
fi
if [[ -z "$SUI_HOME" ]]; then
    if [[ -x /usr/local/s-ui/sui && ! -L /usr/local/s-ui ]]; then
        SUI_HOME=/usr/local/s-ui
    elif [[ -x /opt/s-ui/sui && ! -L /opt/s-ui ]]; then
        SUI_HOME=/opt/s-ui
    else
        SUI_HOME=/usr/local/s-ui
    fi
fi
normalize_sui_home "$SUI_HOME"
function LOGD() {
    echo -e "${yellow}[DEG] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[ERR] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[INF] $* ${plain}"
}

[[ $EUID -ne 0 ]] && LOGE "错误：请使用 root 权限运行此脚本！\n" && exit 1

TTY_ECHO_DISABLED=0
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
        LOGE "交互式密码/API Key 输入需要 TTY，已安全拒绝非交互模式。"
        return 1
    fi
    printf '%s' "$prompt" >/dev/tty
    if ! stty -echo </dev/tty; then
        LOGE "无法关闭终端回显。"
        return 1
    fi
    TTY_ECHO_DISABLED=1
    if ! IFS= read -r -s value </dev/tty; then
        restore_terminal
        printf '\n' >/dev/tty
        LOGE "读取敏感输入失败。"
        return 1
    fi
    restore_terminal
    printf '\n' >/dev/tty
    printf -v "$variable_name" '%s' "$value"
}

if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "检测系统发行版失败，请联系作者！" >&2
    exit 1
fi

echo "系统发行版：$release"

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [默认：$2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]: " temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" || x"${temp}" == x"yes" || x"${temp}" == x"YES" || x"${temp}" == x"1" || x"${temp}" == x"是" || x"${temp}" == x"好" || x"${temp}" == x"确定" || x"${temp}" == x"確定" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "是否重启 ${1} 服务" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}按回车返回主菜单${plain}" && read temp
    show_menu
}

verify_local_release_file() {
    local file="$1" mode="" owner=""
    [[ -f "$file" && ! -L "$file" ]] || { LOGE "缺少安全的本地发布脚本：$file"; return 1; }
    owner="$(stat -c '%u:%g' -- "$file" 2>/dev/null || true)"
    mode="$(stat -c '%a' -- "$file" 2>/dev/null || true)"
    if [[ "$owner" != "0:0" || ! "$mode" =~ ^[0-7]{3,4}$ ]] || (( (8#$mode) & 022 )); then
        LOGE "本地发布脚本的所有者或权限不安全：$file"
        return 1
    fi
}

run_local_release_installer() {
    local installer="$SUI_HOME/install.sh"
    verify_local_release_file "$installer" || {
        LOGE "没有可信的本地bootstrap；为防止执行可变远程分支脚本，更新已安全拒绝。请使用已校验发布 ZIP 重新部署。"
        return 1
    }
    "$installer" --lang="${SUI_LANG:-zh}" --install-dir="$SUI_HOME" "$@"
}

install() {
    run_local_release_installer "$@"
}

update() {
    confirm "该功能会从本地、经已验证发布 ZIP 带来的install.sh 更新，数据不会丢失。是否继续？" "n"
    if [[ $? != 0 ]]; then
        LOGE "已取。"
        [[ $# == 0 ]] && before_show_menu
        return 0
    fi
    if run_local_release_installer --upgrade; then
        LOGI "更新完成；安装器已按原服务状态处理启动。"
        exit 0
    fi
}

custom_version() {
    echo "请输入面板版本标签（例如 s-ui-go-v0.1.0）："
    read -r panel_version
    if [[ -z "$panel_version" ]]; then
        LOGE "面板版本不能为空，已退出。"
        return 1
    fi
    run_local_release_installer --upgrade --version "$panel_version"
}

uninstall() {
    local uninstaller="$SUI_HOME/uninstall-linux.sh"
    confirm "确定要卸载面板吗。" "n"
    if [[ $? != 0 ]]; then
        [[ $# == 0 ]] && show_menu
        return 0
    fi
    verify_local_release_file "$uninstaller" || {
        LOGE "本地卸载器不存在或不可信，已拒绝直接删除文件。"
        return 1
    }
    "$uninstaller" --install-dir "$SUI_HOME"
    local status=$?
    if [[ "$status" -eq 0 ]]; then
        LOGI "卸载完成。"
    fi
    return "$status"
}
reset_admin() {
    echo "不建议将管理员账号密码恢复为默认值！"
    confirm "确定要将管理员账号密码重置为默认值吗。" "n"
    if [[ $? == 0 ]]; then
        "$SUI_HOME/sui" admin -reset
    fi
    before_show_menu
}

set_admin() {
    echo "请设置便于记忆且足够安全的管理员账号密码。"
    read -r -p "请设置管理员用户名：" config_account
    if ! read_secret "请设置管理员密码。" config_password; then
        before_show_menu
        return 1
    fi
    "$SUI_HOME/sui" admin -username "$config_account" -password "$config_password"
    unset config_password
    before_show_menu
}

view_admin() {
    "$SUI_HOME/sui" admin -show
    before_show_menu
}

reset_setting() {
    confirm "确定要将面板设置重置为默认值吗。" "n"
    if [[ $? == 0 ]]; then
        "$SUI_HOME/sui" setting -reset
    fi
    before_show_menu
}

set_setting() {
    echo -e "请输入${yellow}面板端口${plain}（留空则使用当前/默认值）。"
    read config_port
    echo -e "请输入${yellow}面板路径${plain}（留空则使用当前/默认值）。"
    read config_path

    echo -e "请输入${yellow}订阅端口${plain}（留空则使用当前/默认值）。"
    read config_subPort
    echo -e "请输入${yellow}订阅路径${plain}（留空则使用当前/默认值）。"
    read config_subPath

    echo -e "${yellow}正在初始化，请稍等..${plain}"
    params=()
    [ -z "$config_port" ] || params+=("-port" "$config_port")
    [ -z "$config_path" ] || params+=("-path" "$config_path")
    [ -z "$config_subPort" ] || params+=("-subPort" "$config_subPort")
    [ -z "$config_subPath" ] || params+=("-subPath" "$config_subPath")
    "$SUI_HOME/sui" setting "${params[@]}"
    before_show_menu
}

view_setting() {
    "$SUI_HOME/sui" setting -show
    view_uri
    before_show_menu
}

view_uri() {
    info=$("$SUI_HOME/sui" uri)
    if [[ $? != 0 ]]; then
        LOGE "获取当前访问地址失败"
        before_show_menu
    fi
    LOGI "可通过以下完整地址访问面板。"
    echo -e "${green}${info}${plain}"
}

start() {
    check_status $1
    if [[ $? == 0 ]]; then
        echo ""
        LOGI "${1} 已在运行，无需重复启动。如需重启，请选择重启。"
    else
        systemctl start $1
        sleep 2
        check_status $1
        if [[ $? == 0 ]]; then
            LOGI "${1} 启动成功"
        else
            LOGE "${1} 启动失败，可能启动时间超过 2 秒，请稍后查看日志。"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status $1
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "${1} 已停止，无需重复停止。"
    else
        systemctl stop $1
        sleep 2
        check_status $1
        if [[ $? == 1 ]]; then
            LOGI "${1} 停止成功"
        else
            LOGE "${1} 停止失败，可能停止时间超过 2 秒，请稍后查看日志。"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

restart() {
    systemctl restart $1
    sleep 2
    check_status $1
    if [[ $? == 0 ]]; then
        LOGI "${1} 重启成功"
    else
        LOGE "${1} 重启失败，可能启动时间超过 2 秒，请稍后查看日志。"
    fi
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

status() {
    systemctl status s-ui -l
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    systemctl enable $1
    if [[ $? == 0 ]]; then
        LOGI "已成功设。${1} 开机自。"
    else
        LOGE "设置 ${1} 开机自启失。"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable $1
    if [[ $? == 0 ]]; then
        LOGI "已成功取。${1} 开机自。"
    else
        LOGE "取消 ${1} 开机自启失。"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

show_log() {
    journalctl -u $1.service -e --no-pager -f
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

update_shell() {
    LOGE "已禁用远程分支脚本更新。管理器随经。ZIP/SHA-256 校验的发布包更新；请使用 s-ui update。"
    return 1
}
check_status() {
    if [[ ! -f "/etc/systemd/system/$1.service" ]]; then
        return 2
    fi
    temp=$(systemctl status "$1" | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
    if [[ x"${temp}" == x"running" ]]; then
        return 0
    else
        return 1
    fi
}

check_enabled() {
    temp=$(systemctl is-enabled $1)
    if [[ x"${temp}" == x"enabled" ]]; then
        return 0
    else
        return 1
    fi
}

check_uninstall() {
    check_status s-ui
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "面板已安装，请不要重复安装。"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status s-ui
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "请先安装面板。"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status $1
    case $? in
    0)
        echo -e "${1} 状态：${green}运行${plain}"
        show_enable_status $1
        ;;
    1)
        echo -e "${1} 状态：${yellow}未运${plain}"
        show_enable_status $1
        ;;
    2)
        echo -e "${1} 状态：${red}未安${plain}"
        ;;
    esac
}

show_enable_status() {
    check_enabled $1
    if [[ $? == 0 ]]; then
        echo -e "${1} 开机自启：${green}${plain}"
    else
        echo -e "${1} 开机自启：${red}${plain}"
    fi
}

check_s-ui_status() {
    count=$(ps -ef | grep "sui" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_s-ui_status() {
    check_s-ui_status
    if [[ $? == 0 ]]; then
        echo -e "s-ui 状态：${green}运行${plain}"
    else
        echo -e "s-ui 状态：${red}未运${plain}"
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} 启用 BBR"
    echo -e "${green}\t2.${plain} 禁用 BBR"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -p "请选择操作。" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        ;;
    2)
        disable_bbr
        ;;
    *) echo "无效选择" ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${yellow}当前未启。BBR${plain}"
        exit 0
    fi
    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "cubic" ]]; then
        echo -e "${green}已成功将 BBR 切换。CUBIC${plain}"
    else
        echo -e "${red}切换失败，请检查系统配置${plain}"
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR 已经启用${plain}"
        exit 0
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -yqq --no-install-recommends ca-certificates
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install ca-certificates
        ;;
    fedora)
        dnf -y update && dnf -y install ca-certificates
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm ca-certificates
        ;;
    *)
        echo -e "${red}不支持的操作系统，请检查脚本并手动安装必要依赖${plain}\n"
        exit 1
        ;;
    esac
    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR 启用成功${plain}"
    else
        echo -e "${red}BBR 启用失败，请检查系统配置${plain}"
    fi
}

install_acme() {
    LOGE "出于安全原因，已禁用 curl | sh 形式的可变远。acme.sh 安装。请先通过受信任的软件源或已验证的固定版本安装 acme.sh。"
    return 1
}
validate_certificate_domain() {
    local name="$1"
    [[ ${#name} -le 253 && "$name" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ && "$name" != *..* ]] || {
        LOGE '域名包含不安全字符；拒绝操作证书目录。'
        return 1
    }
}
prepare_certificate_directory() {
    local directory="$1" group=root file
    path_has_symlink_component "$directory" && { LOGE '证书目录不能包含符号链接。'; return 1; }
    [[ "$directory" == "$SUI_HOME/cert" || "$directory" == "$SUI_HOME/cert/"* ]] || return 1
    for file in "$directory/privkey.pem" "$directory/fullchain.pem" "$directory/self.key" "$directory/self.crt"; do
        [[ -L "$file" ]] && return 1
    done
    getent group s-ui >/dev/null && group=s-ui
    install -d -o root -g "$group" -m 0750 -- "$SUI_HOME/cert" "$directory"
}
set_certificate_permissions() {
    local directory="$1" group=root file
    prepare_certificate_directory "$directory" || return 1
    getent group s-ui >/dev/null && group=s-ui
    for file in "$directory/privkey.pem" "$directory/fullchain.pem" "$directory/self.key" "$directory/self.crt"; do
        [[ -L "$file" ]] && return 1
        [[ -f "$file" ]] || continue
        chown root:"$group" -- "$file" && chmod 0640 -- "$file" || return 1
    done
}

ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} 申请 SSL 证书"
    echo -e "${green}\t2.${plain} 吊销证书"
    echo -e "${green}\t3.${plain} 强制续签证书"
    echo -e "${green}\t4.${plain} 生成自签名证书"
    read -p "请选择操作。" choice
    case "$choice" in
        1) ssl_cert_issue ;;
        2)
            local domain=""
            read -p "请输入要吊销证书的域名：" domain
            ~/.acme.sh/acme.sh --revoke -d ${domain}
            LOGI "证书已吊销"
            ;;
        3)
            local domain=""
            read -p "请输入要强制续签证书的域名：" domain
            ~/.acme.sh/acme.sh --renew -d ${domain} --force ;;
        4)
            generate_self_signed_cert
            ;;
        *) echo "无效选择" ;;
    esac
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "未检测到 acme.sh，将自动安装。"
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "安装 acme.sh 失败，请检查日。"
            exit 1
        fi
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt update && apt install socat -y
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install socat
        ;;
    fedora)
        dnf -y update && dnf -y install socat
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat
        ;;
    *)
        echo -e "${red}不支持的操作系统，请检查脚本并手动安装必要依赖${plain}\n"
        exit 1
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "安装 socat 失败，请检查日。"
        exit 1
    else
        LOGI "安装 socat 成功..."
    fi

    local domain=""
    read -r -p "请输入域名：" domain
    validate_certificate_domain "$domain" || return 1
    LOGD "当前域名为：${domain}，正在检。.."
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')

    if [[ "${currentCert}" == "${domain}" ]]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "系统已存在该域名证书，无法重复签发，当前证书信息。"
        LOGI "$certInfo"
        exit 1
    else
        LOGI "域名检查通过，可以开始签发证书.."
    fi

    certPath="$SUI_HOME/cert/${domain}"
    prepare_certificate_directory "$certPath" || return 1

    local WebPort=80
    read -p "请选择用于签发证书的端口，默认 80。" WebPort
    if [[ ! "$WebPort" =~ ^[0-9]{1,5}$ ]] || (( 10#$WebPort < 1 || 10#$WebPort > 65535 )); then
        LOGE "输入的端口 ${WebPort} 无效，将使用默认端口 80"
        WebPort=80
    fi
    LOGI "将使用端。${WebPort} 签发证书，请确保该端口已放行..."
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --standalone --httpport ${WebPort}
    if [ $? -ne 0 ]; then
        LOGE "证书签发失败，请检查日。"
        rm -rf -- "$HOME/.acme.sh/${domain}"
        exit 1
    else
        LOGI "证书签发成功，正在安装证书.."
    fi
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file "${certPath}/privkey.pem" \
        --fullchain-file "${certPath}/fullchain.pem"

    if [ $? -ne 0 ]; then
        LOGE "安装证书失败，已退。"
        rm -rf -- "$HOME/.acme.sh/${domain}"
        exit 1
    else
        LOGI "安装证书成功，正在启用自动续签.."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "自动续签设置失败，证书详情："
        ls -lah cert/*
        set_certificate_permissions "$certPath" || return 1
        exit 1
    else
        LOGI "自动续签设置成功，证书详情："
        ls -lah cert/*
        set_certificate_permissions "$certPath" || return 1
    fi
}

ssl_cert_issue_CF() {
    echo -E ""
    LOGD "******使用说明******"
    echo "1) 通过 Cloudflare 申请新证书"
    echo "2) 强制续签已有证书"
    echo "3) 返回主菜单"
    read -p "请输入选项 [1-3]。" choice
    certPath="$SUI_HOME/cert"

    case $choice in
        1|2)
            force_flag=""
            if [ "$choice" -eq 2 ]; then
                force_flag="--force"
                echo "正在强制重新签发 SSL 证书..."
            else
                echo "正在开始签。SSL 证书..."
            fi

            LOGD "******使用说明******"
            LOGI "。acme.sh 脚本需要以下信息："
            LOGI "1. Cloudflare 注册邮箱"
            LOGI "2. Cloudflare Global API Key"
            LOGI "3. 已通过 Cloudflare DNS 解析到当前服务器的域。"
            LOGI "4. 证书默认安装路径：$SUI_HOME/cert"
            confirm "确认继续签" "y"
            if [ $? -eq 0 ]; then
                if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                    echo "未检测到 acme.sh，正在安。.."
                    install_acme
                    if [ $? -ne 0 ]; then
                        LOGE "安装 acme.sh 失败，请检查日。"
                        exit 1
                    fi
                fi

                CF_Domain=""
                LOGD "请设置域名："
                read -r -p "请输入域名：" CF_Domain
                validate_certificate_domain "$CF_Domain" || return 1
                LOGD "当前域名为：${CF_Domain}"

                CF_GlobalKey=""
                CF_AccountEmail=""
                LOGD "请设。API Key。"
                if ! read_secret "请输入?API Key。" CF_GlobalKey; then
                    return 1
                fi
                LOGD "API Key 已输入。"

                LOGD "请设。Cloudflare 注册邮箱。"
                read -p "请输入邮箱：" CF_AccountEmail
                LOGD "当前注册邮箱为：${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                if [ $? -ne 0 ]; then
                    LOGE "设置默认证书颁发机构 Let's Encrypt 失败，脚本已退。.."
                    exit 1
                fi

                export CF_Key="${CF_GlobalKey}"
                export CF_Email="${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --issue --dns dns_cf -d "${CF_Domain}" -d "*.${CF_Domain}" $force_flag --log
                if [ $? -ne 0 ]; then
                    LOGE "证书签发失败，脚本已退。.."
                    exit 1
                else
                    LOGI "证书签发成功，正在安。.."
                fi

                prepare_certificate_directory "${certPath}/${CF_Domain}"
                if [ $? -ne 0 ]; then
                    LOGE "创建目录失败${certPath}/${CF_Domain}"
                    exit 1
                fi

                ~/.acme.sh/acme.sh --installcert -d "${CF_Domain}" -d "*.${CF_Domain}" \
                    --fullchain-file "${certPath}/${CF_Domain}/fullchain.pem" \
                    --key-file "${certPath}/${CF_Domain}/privkey.pem"

                if [ $? -ne 0 ]; then
                    LOGE "证书安装失败，脚本已退。.."
                    exit 1
                else
                    LOGI "证书安装成功，正在开启自动更。.."
                fi

                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                if [ $? -ne 0 ]; then
                    LOGE "自动更新设置失败，脚本已退。.."
                    exit 1
                else
                    LOGI "证书已安装，并已开启自动续签。"
                    ls -lah ${certPath}/${CF_Domain}
                    set_certificate_permissions "${certPath}/${CF_Domain}" || return 1
                fi
            fi
            show_menu
            ;;
        3)
            echo "正在返回主菜单.."
            show_menu
            ;;
        *)
            echo "无效选择，请重新选择。"
            show_menu
            ;;
    esac
}

generate_self_signed_cert() {
    cert_dir="$SUI_HOME/cert"
    prepare_certificate_directory "$cert_dir" || return 1
    LOGI "请选择证书类型。"
    echo -e "${green}\t1.${plain} Ed25519（推荐）"
    echo -e "${green}\t2.${plain} RSA 2048"
    echo -e "${green}\t3.${plain} RSA 4096"
    echo -e "${green}\t4.${plain} ECDSA prime256v1"
    echo -e "${green}\t5.${plain} ECDSA secp384r1"
    read -p "请输入选项 [1-5，默。1]。" cert_type
    cert_type=${cert_type:-1}

    case "$cert_type" in
        1)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
        2)
            algo="rsa"
            key_opt="-newkey rsa:2048"
            ;;
        3)
            algo="rsa"
            key_opt="-newkey rsa:4096"
            ;;
        4)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:prime256v1"
            ;;
        5)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:secp384r1"
            ;;
        *)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
    esac

    LOGI "正在生成自签名证书（$algo。.."
    sudo openssl req -x509 -nodes -days 3650 $key_opt \
        -keyout "${cert_dir}/self.key" \
        -out "${cert_dir}/self.crt" \
        -subj "/CN=myserver"
    if [[ $? -eq 0 ]]; then
        set_certificate_permissions "$cert_dir" || return 1
        LOGI "自签名证书生成成功！"
        LOGI "证书路径${cert_dir}/self.crt"
        LOGI "密钥路径${cert_dir}/self.key"
    else
        LOGE "生成自签名证书失败。"
    fi
    before_show_menu
}

show_usage() {
    echo -e "S-UI 管理脚本使用说明"
    echo -e "------------------------------------------"
    echo -e "子命令："
    echo -e "s-ui              - 打开管理菜单"
    echo -e "s-ui start        - 启动 s-ui"
    echo -e "s-ui stop         - 停止 s-ui"
    echo -e "s-ui restart      - 重启 s-ui"
    echo -e "s-ui status       - 查看 s-ui 当前状。"
    echo -e "s-ui enable       - 设置开机自。"
    echo -e "s-ui disable      - 取消开机自。"
    echo -e "s-ui log          - 查看 s-ui 日志"
    echo -e "s-ui update       - 更新"
    echo -e "s-ui install      - 安装"
    echo -e "s-ui uninstall    - 卸载"
    echo -e "s-ui help         - 查看使用说明"
    echo -e "------------------------------------------"
}

show_menu() {
    echo -e "
${green}S-UI 管理菜单${plain}
0. 退出
1. 安装    2. 更新    3. 自定义版本    4. 卸载
5. 重置管理员    6. 设置管理员    7. 查看管理员
8. 重置面板设置    9. 设置面板    10. 查看访问地址
11. 启动    12. 停止    13. 重启    14. 状态
15. 日志    16. 开机启动    17. 取消开机启动
18. BBR    19. SSL 证书    20. Cloudflare SSL 证书
"
    show_status s-ui
    echo && read -r -p "请输入选项 [0-20]：" num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && custom_version
        ;;
    4)
        check_install && uninstall
        ;;
    5)
        check_install && reset_admin
        ;;
    6)
        check_install && set_admin
        ;;
    7)
        check_install && view_admin
        ;;
    8)
        check_install && reset_setting
        ;;
    9)
        check_install && set_setting
        ;;
    10)
        check_install && view_setting
        ;;
    11)
        check_install && start s-ui
        ;;
    12)
        check_install && stop s-ui
        ;;
    13)
        check_install && restart s-ui
        ;;
    14)
        check_install && status s-ui
        ;;
    15)
        check_install && show_log s-ui
        ;;
    16)
        check_install && enable s-ui
        ;;
    17)
        check_install && disable s-ui
        ;;
    18)
        bbr_menu
        ;;
    19)
        ssl_cert_issue_main
        ;;
    20)
        ssl_cert_issue_CF
        ;;
    *)
        LOGE "请输入正确的数字 [0-20]"
        ;;
    esac
}

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start s-ui 0
        ;;
    "stop")
        check_install 0 && stop s-ui 0
        ;;
    "restart")
        check_install 0 && restart s-ui 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "enable")
        check_install 0 && enable s-ui 0
        ;;
    "disable")
        check_install 0 && disable s-ui 0
        ;;
    "log")
        check_install 0 && show_log s-ui 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "install")
        check_uninstall 0 && install
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
