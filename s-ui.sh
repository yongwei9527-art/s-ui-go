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
    [[ -n "$raw" && "$raw" == /* ]] || { echo '閿欒锛氬畨瑁呯洰褰曞繀椤绘槸缁濆璺緞 / Error: installation directory must be absolute.' >&2; exit 1; }
    [[ "$raw" =~ ^[A-Za-z0-9._/-]+$ ]] || { echo '閿欒锛氬畨瑁呯洰褰曞寘鍚笉瀹夊叏瀛楃 / Error: installation directory contains unsafe characters.' >&2; exit 1; }
    wrapped="/${raw#/}/"
    [[ "$wrapped" != *'/../'* && "$wrapped" != *'/./'* ]] || { echo '閿欒锛氬畨瑁呯洰褰曚笉寰楀寘鍚矾寰勭┛瓒?/ Error: path traversal is not allowed.' >&2; exit 1; }
    path_has_symlink_component "$raw" && { echo '閿欒锛氬畨瑁呯洰褰曟垨鐖剁洰褰曚笉鑳芥槸绗﹀彿閾炬帴 / Error: installation path cannot contain symlinks.' >&2; exit 1; }
    normalized="$(realpath -m -- "$raw")" || exit 1
    path_has_symlink_component "$normalized" && { echo '閿欒锛氳鑼冨寲鐩綍鍖呭惈绗﹀彿閾炬帴 / Error: normalized path contains a symlink.' >&2; exit 1; }
    case "$normalized" in
        /usr/local/*|/opt/*|/srv/*) ;;
        *) echo '閿欒锛氬畨瑁呯洰褰曚粎鍏佽鍦?/usr/local/*銆?opt/* 鎴?/srv/* / Error: install directory must be under /usr/local/*, /opt/*, or /srv/*.' >&2; exit 1 ;;
    esac
    case "$normalized" in
        /usr/local|/opt|/srv|/usr/local/bin|/usr/local/sbin|/usr/local/lib|/usr/local/lib64|/usr/local/share|/usr/local/include|/usr/local/src)
            echo '閿欒锛氬畨瑁呯洰褰曡繃浜庡娉涙垨鍗遍櫓 / Error: installation directory is too broad or dangerous.' >&2
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

[[ $EUID -ne 0 ]] && LOGE "閿欒锛氳浣跨敤 root 鏉冮檺杩愯姝よ剼鏈紒\n" && exit 1

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
        LOGE "浜や簰寮忓瘑鐮?API Key 杈撳叆闇€瑕?TTY锛屽凡瀹夊叏鎷掔粷闈炰氦浜掓ā寮忋€?
        return 1
    fi
    printf '%s' "$prompt" >/dev/tty
    if ! stty -echo </dev/tty; then
        LOGE "鏃犳硶鍏抽棴缁堢鍥炴樉銆?
        return 1
    fi
    TTY_ECHO_DISABLED=1
    if ! IFS= read -r -s value </dev/tty; then
        restore_terminal
        printf '\n' >/dev/tty
        LOGE "璇诲彇鏁忔劅杈撳叆澶辫触銆?
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
    echo "妫€娴嬬郴缁熷彂琛岀増澶辫触锛岃鑱旂郴浣滆€咃紒" >&2
    exit 1
fi

echo "绯荤粺鍙戣鐗堬細$release"

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [榛樿锛?2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]: " temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" || x"${temp}" == x"yes" || x"${temp}" == x"YES" || x"${temp}" == x"1" || x"${temp}" == x"鏄? || x"${temp}" == x"濂? || x"${temp}" == x"纭畾" || x"${temp}" == x"纰哄畾" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "鏄惁閲嶅惎 ${1} 鏈嶅姟" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}鎸夊洖杞﹁繑鍥炰富鑿滃崟锛?{plain}" && read temp
    show_menu
}

verify_local_release_file() {
    local file="$1" mode="" owner=""
    [[ -f "$file" && ! -L "$file" ]] || { LOGE "缂哄皯瀹夊叏鐨勬湰鍦板彂甯冭剼鏈細$file"; return 1; }
    owner="$(stat -c '%u:%g' -- "$file" 2>/dev/null || true)"
    mode="$(stat -c '%a' -- "$file" 2>/dev/null || true)"
    if [[ "$owner" != "0:0" || ! "$mode" =~ ^[0-7]{3,4}$ ]] || (( (8#$mode) & 022 )); then
        LOGE "鏈湴鍙戝竷鑴氭湰鐨勬墍鏈夎€呮垨鏉冮檺涓嶅畨鍏細$file"
        return 1
    fi
}

run_local_release_installer() {
    local installer="$SUI_HOME/install.sh"
    verify_local_release_file "$installer" || {
        LOGE "娌℃湁鍙俊鐨勬湰鍦?bootstrap锛涗负闃叉鎵ц鍙彉杩滅▼鍒嗘敮鑴氭湰锛屾洿鏂板凡瀹夊叏鎷掔粷銆傝浣跨敤宸叉牎楠屽彂甯?ZIP 閲嶆柊閮ㄧ讲銆?
        return 1
    }
    "$installer" --lang="${SUI_LANG:-zh}" --install-dir="$SUI_HOME" "$@"
}

install() {
    run_local_release_installer "$@"
}

update() {
    confirm "璇ュ姛鑳戒細浠庢湰鍦般€佺粡宸查獙璇佸彂甯?ZIP 甯︽潵鐨?install.sh 鏇存柊锛屾暟鎹笉浼氫涪澶便€傛槸鍚︾户缁紵" "n"
    if [[ $? != 0 ]]; then
        LOGE "宸插彇娑?
        [[ $# == 0 ]] && before_show_menu
        return 0
    fi
    if run_local_release_installer --upgrade; then
        LOGI "鏇存柊瀹屾垚锛涘畨瑁呭櫒宸叉寜鍘熸湇鍔＄姸鎬佸鐞嗗惎鍔ㄣ€?
        exit 0
    fi
}

custom_version() {
    echo "璇疯緭鍏ラ潰鏉跨増鏈爣绛撅紙渚嬪 s-ui-go-v0.1.0锛夛細"
    read -r panel_version
    if [[ -z "$panel_version" ]]; then
        LOGE "闈㈡澘鐗堟湰涓嶈兘涓虹┖锛屽凡閫€鍑恒€?
        return 1
    fi
    run_local_release_installer --upgrade --version "$panel_version"
}

uninstall() {
    local uninstaller="$SUI_HOME/uninstall-linux.sh"
    confirm "纭畾瑕佸嵏杞介潰鏉垮悧锛? "n"
    if [[ $? != 0 ]]; then
        [[ $# == 0 ]] && show_menu
        return 0
    fi
    verify_local_release_file "$uninstaller" || {
        LOGE "鏈湴鍗歌浇鍣ㄤ笉瀛樺湪鎴栦笉鍙俊锛屽凡鎷掔粷鐩存帴鍒犻櫎鏂囦欢銆?
        return 1
    }
    "$uninstaller" --install-dir "$SUI_HOME"
    local status=$?
    if [[ "$status" -eq 0 ]]; then
        LOGI "鍗歌浇瀹屾垚銆?
    fi
    return "$status"
}
reset_admin() {
    echo "涓嶅缓璁皢绠＄悊鍛樿处鍙峰瘑鐮佹仮澶嶄负榛樿鍊硷紒"
    confirm "纭畾瑕佸皢绠＄悊鍛樿处鍙峰瘑鐮侀噸缃负榛樿鍊煎悧锛? "n"
    if [[ $? == 0 ]]; then
        "$SUI_HOME/sui" admin -reset
    fi
    before_show_menu
}

set_admin() {
    echo "璇疯缃究浜庤蹇嗕笖瓒冲瀹夊叏鐨勭鐞嗗憳璐﹀彿瀵嗙爜銆?
    read -r -p "璇疯缃鐞嗗憳鐢ㄦ埛鍚嶏細" config_account
    if ! read_secret "璇疯缃鐞嗗憳瀵嗙爜锛? config_password; then
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
    confirm "纭畾瑕佸皢闈㈡澘璁剧疆閲嶇疆涓洪粯璁ゅ€煎悧锛? "n"
    if [[ $? == 0 ]]; then
        "$SUI_HOME/sui" setting -reset
    fi
    before_show_menu
}

set_setting() {
    echo -e "璇疯緭鍏?{yellow}闈㈡澘绔彛${plain}锛堢暀绌哄垯浣跨敤褰撳墠/榛樿鍊硷級锛?
    read config_port
    echo -e "璇疯緭鍏?{yellow}闈㈡澘璺緞${plain}锛堢暀绌哄垯浣跨敤褰撳墠/榛樿鍊硷級锛?
    read config_path

    echo -e "璇疯緭鍏?{yellow}璁㈤槄绔彛${plain}锛堢暀绌哄垯浣跨敤褰撳墠/榛樿鍊硷級锛?
    read config_subPort
    echo -e "璇疯緭鍏?{yellow}璁㈤槄璺緞${plain}锛堢暀绌哄垯浣跨敤褰撳墠/榛樿鍊硷級锛?
    read config_subPath

    echo -e "${yellow}姝ｅ湪鍒濆鍖栵紝璇风◢鍊?..${plain}"
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
        LOGE "鑾峰彇褰撳墠璁块棶鍦板潃澶辫触"
        before_show_menu
    fi
    LOGI "鍙€氳繃浠ヤ笅瀹屾暣鍦板潃璁块棶闈㈡澘锛?
    echo -e "${green}${info}${plain}"
}

start() {
    check_status $1
    if [[ $? == 0 ]]; then
        echo ""
        LOGI "${1} 宸插湪杩愯锛屾棤闇€閲嶅鍚姩銆傚闇€閲嶅惎锛岃閫夋嫨閲嶅惎銆?
    else
        systemctl start $1
        sleep 2
        check_status $1
        if [[ $? == 0 ]]; then
            LOGI "${1} 鍚姩鎴愬姛"
        else
            LOGE "${1} 鍚姩澶辫触锛屽彲鑳藉惎鍔ㄦ椂闂磋秴杩?2 绉掞紝璇风◢鍚庢煡鐪嬫棩蹇椼€?
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
        LOGI "${1} 宸插仠姝紝鏃犻渶閲嶅鍋滄锛?
    else
        systemctl stop $1
        sleep 2
        check_status $1
        if [[ $? == 1 ]]; then
            LOGI "${1} 鍋滄鎴愬姛"
        else
            LOGE "${1} 鍋滄澶辫触锛屽彲鑳藉仠姝㈡椂闂磋秴杩?2 绉掞紝璇风◢鍚庢煡鐪嬫棩蹇椼€?
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
        LOGI "${1} 閲嶅惎鎴愬姛"
    else
        LOGE "${1} 閲嶅惎澶辫触锛屽彲鑳藉惎鍔ㄦ椂闂磋秴杩?2 绉掞紝璇风◢鍚庢煡鐪嬫棩蹇椼€?
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
        LOGI "宸叉垚鍔熻缃?${1} 寮€鏈鸿嚜鍚?
    else
        LOGE "璁剧疆 ${1} 寮€鏈鸿嚜鍚け璐?
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable $1
    if [[ $? == 0 ]]; then
        LOGI "宸叉垚鍔熷彇娑?${1} 寮€鏈鸿嚜鍚?
    else
        LOGE "鍙栨秷 ${1} 寮€鏈鸿嚜鍚け璐?
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
    LOGE "宸茬鐢ㄨ繙绋嬪垎鏀剼鏈洿鏂般€傜鐞嗗櫒闅忕粡杩?ZIP/SHA-256 鏍￠獙鐨勫彂甯冨寘鏇存柊锛涜浣跨敤 s-ui update銆?
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
        LOGE "闈㈡澘宸插畨瑁咃紝璇蜂笉瑕侀噸澶嶅畨瑁呫€?
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
        LOGE "璇峰厛瀹夎闈㈡澘銆?
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
        echo -e "${1} 鐘舵€侊細${green}杩愯涓?{plain}"
        show_enable_status $1
        ;;
    1)
        echo -e "${1} 鐘舵€侊細${yellow}鏈繍琛?{plain}"
        show_enable_status $1
        ;;
    2)
        echo -e "${1} 鐘舵€侊細${red}鏈畨瑁?{plain}"
        ;;
    esac
}

show_enable_status() {
    check_enabled $1
    if [[ $? == 0 ]]; then
        echo -e "${1} 寮€鏈鸿嚜鍚細${green}鏄?{plain}"
    else
        echo -e "${1} 寮€鏈鸿嚜鍚細${red}鍚?{plain}"
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
        echo -e "s-ui 鐘舵€侊細${green}杩愯涓?{plain}"
    else
        echo -e "s-ui 鐘舵€侊細${red}鏈繍琛?{plain}"
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} 鍚敤 BBR"
    echo -e "${green}\t2.${plain} 绂佺敤 BBR"
    echo -e "${green}\t0.${plain} 杩斿洖涓昏彍鍗?
    read -p "璇烽€夋嫨鎿嶄綔锛? choice
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
    *) echo "鏃犳晥閫夋嫨" ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${yellow}褰撳墠鏈惎鐢?BBR銆?{plain}"
        exit 0
    fi
    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "cubic" ]]; then
        echo -e "${green}宸叉垚鍔熷皢 BBR 鍒囨崲涓?CUBIC銆?{plain}"
    else
        echo -e "${red}鍒囨崲澶辫触锛岃妫€鏌ョ郴缁熼厤缃€?{plain}"
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR 宸茬粡鍚敤锛?{plain}"
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
        echo -e "${red}涓嶆敮鎸佺殑鎿嶄綔绯荤粺锛岃妫€鏌ヨ剼鏈苟鎵嬪姩瀹夎蹇呰渚濊禆銆?{plain}\n"
        exit 1
        ;;
    esac
    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR 鍚敤鎴愬姛銆?{plain}"
    else
        echo -e "${red}BBR 鍚敤澶辫触锛岃妫€鏌ョ郴缁熼厤缃€?{plain}"
    fi
}

install_acme() {
    LOGE "鍑轰簬瀹夊叏鍘熷洜锛屽凡绂佺敤 curl | sh 褰㈠紡鐨勫彲鍙樿繙绋?acme.sh 瀹夎銆傝鍏堥€氳繃鍙椾俊浠荤殑杞欢婧愭垨宸查獙璇佺殑鍥哄畾鐗堟湰瀹夎 acme.sh銆?
    return 1
}
ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} 鐢宠 SSL 璇佷功"
    echo -e "${green}\t2.${plain} 鍚婇攢璇佷功"
    echo -e "${green}\t3.${plain} 寮哄埗缁璇佷功"
    echo -e "${green}\t4.${plain} 鐢熸垚鑷鍚嶈瘉涔?
    read -p "璇烽€夋嫨鎿嶄綔锛? choice
    case "$choice" in
        1) ssl_cert_issue ;;
        2)
            local domain=""
            read -p "璇疯緭鍏ヨ鍚婇攢璇佷功鐨勫煙鍚嶏細" domain
            ~/.acme.sh/acme.sh --revoke -d ${domain}
            LOGI "璇佷功宸插悐閿€"
            ;;
        3)
            local domain=""
            read -p "璇疯緭鍏ヨ寮哄埗缁璇佷功鐨勫煙鍚嶏細" domain
            ~/.acme.sh/acme.sh --renew -d ${domain} --force ;;
        4)
            generate_self_signed_cert
            ;;
        *) echo "鏃犳晥閫夋嫨" ;;
    esac
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "鏈娴嬪埌 acme.sh锛屽皢鑷姩瀹夎銆?
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "瀹夎 acme.sh 澶辫触锛岃妫€鏌ユ棩蹇?
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
        echo -e "${red}涓嶆敮鎸佺殑鎿嶄綔绯荤粺锛岃妫€鏌ヨ剼鏈苟鎵嬪姩瀹夎蹇呰渚濊禆銆?{plain}\n"
        exit 1
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "瀹夎 socat 澶辫触锛岃妫€鏌ユ棩蹇?
        exit 1
    else
        LOGI "瀹夎 socat 鎴愬姛..."
    fi

    local domain=""
    read -p "璇疯緭鍏ュ煙鍚嶏細" domain
    LOGD "褰撳墠鍩熷悕涓猴細${domain}锛屾鍦ㄦ鏌?.."
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')

    if [[ "${currentCert}" == "${domain}" ]]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "绯荤粺宸插瓨鍦ㄨ鍩熷悕璇佷功锛屾棤娉曢噸澶嶇鍙戯紝褰撳墠璇佷功淇℃伅锛?
        LOGI "$certInfo"
        exit 1
    else
        LOGI "鍩熷悕妫€鏌ラ€氳繃锛屽彲浠ュ紑濮嬬鍙戣瘉涔?.."
    fi

    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf -- "$certPath"
        mkdir -p "$certPath"
    fi

    local WebPort=80
    read -p "璇烽€夋嫨鐢ㄤ簬绛惧彂璇佷功鐨勭鍙ｏ紝榛樿 80锛? WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        LOGE "杈撳叆鐨勭鍙?${WebPort} 鏃犳晥锛屽皢浣跨敤榛樿绔彛"
    fi
    LOGI "灏嗕娇鐢ㄧ鍙?${WebPort} 绛惧彂璇佷功锛岃纭繚璇ョ鍙ｅ凡鏀捐..."
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --standalone --httpport ${WebPort}
    if [ $? -ne 0 ]; then
        LOGE "璇佷功绛惧彂澶辫触锛岃妫€鏌ユ棩蹇?
        rm -rf -- "$HOME/.acme.sh/${domain}"
        exit 1
    else
        LOGI "璇佷功绛惧彂鎴愬姛锛屾鍦ㄥ畨瑁呰瘉涔?.."
    fi
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem

    if [ $? -ne 0 ]; then
        LOGE "瀹夎璇佷功澶辫触锛屽凡閫€鍑?
        rm -rf -- "$HOME/.acme.sh/${domain}"
        exit 1
    else
        LOGI "瀹夎璇佷功鎴愬姛锛屾鍦ㄥ惎鐢ㄨ嚜鍔ㄧ画绛?.."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "鑷姩缁璁剧疆澶辫触锛岃瘉涔﹁鎯咃細"
        ls -lah cert/*
        chmod 755 $certPath/*
        exit 1
    else
        LOGI "鑷姩缁璁剧疆鎴愬姛锛岃瘉涔﹁鎯咃細"
        ls -lah cert/*
        chmod 755 $certPath/*
    fi
}

ssl_cert_issue_CF() {
    echo -E ""
    LOGD "******浣跨敤璇存槑******"
    echo "1) 閫氳繃 Cloudflare 鐢宠鏂拌瘉涔?
    echo "2) 寮哄埗缁宸叉湁璇佷功"
    echo "3) 杩斿洖涓昏彍鍗?
    read -p "璇疯緭鍏ラ€夐」 [1-3]锛? choice

    certPath="/root/cert-CF"

    case $choice in
        1|2)
            force_flag=""
            if [ "$choice" -eq 2 ]; then
                force_flag="--force"
                echo "姝ｅ湪寮哄埗閲嶆柊绛惧彂 SSL 璇佷功..."
            else
                echo "姝ｅ湪寮€濮嬬鍙?SSL 璇佷功..."
            fi

            LOGD "******浣跨敤璇存槑******"
            LOGI "姝?acme.sh 鑴氭湰闇€瑕佷互涓嬩俊鎭細"
            LOGI "1. Cloudflare 娉ㄥ唽閭"
            LOGI "2. Cloudflare Global API Key"
            LOGI "3. 宸查€氳繃 Cloudflare DNS 瑙ｆ瀽鍒板綋鍓嶆湇鍔″櫒鐨勫煙鍚?
            LOGI "4. 璇佷功榛樿瀹夎璺緞涓?/root/cert"
            confirm "纭缁х画锛? "y"
            if [ $? -eq 0 ]; then
                if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                    echo "鏈娴嬪埌 acme.sh锛屾鍦ㄥ畨瑁?.."
                    install_acme
                    if [ $? -ne 0 ]; then
                        LOGE "瀹夎 acme.sh 澶辫触锛岃妫€鏌ユ棩蹇?
                        exit 1
                    fi
                fi

                CF_Domain=""
                if [ ! -d "$certPath" ]; then
                    mkdir -p $certPath
                else
                    rm -rf -- "$certPath"
                    mkdir -p $certPath
                fi

                LOGD "璇疯缃煙鍚嶏細"
                read -p "璇疯緭鍏ュ煙鍚嶏細" CF_Domain
                LOGD "褰撳墠鍩熷悕涓猴細${CF_Domain}"

                CF_GlobalKey=""
                CF_AccountEmail=""
                LOGD "璇疯缃?API Key锛?
                if ! read_secret "璇疯緭鍏?API Key锛? CF_GlobalKey; then
                    return 1
                fi
                LOGD "API Key 宸茶緭鍏ャ€?

                LOGD "璇疯缃?Cloudflare 娉ㄥ唽閭锛?
                read -p "璇疯緭鍏ラ偖绠憋細" CF_AccountEmail
                LOGD "褰撳墠娉ㄥ唽閭涓猴細${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                if [ $? -ne 0 ]; then
                    LOGE "璁剧疆榛樿璇佷功棰佸彂鏈烘瀯 Let's Encrypt 澶辫触锛岃剼鏈凡閫€鍑?.."
                    exit 1
                fi

                export CF_Key="${CF_GlobalKey}"
                export CF_Email="${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} $force_flag --log
                if [ $? -ne 0 ]; then
                    LOGE "璇佷功绛惧彂澶辫触锛岃剼鏈凡閫€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功绛惧彂鎴愬姛锛屾鍦ㄥ畨瑁?.."
                fi

                mkdir -p ${certPath}/${CF_Domain}
                if [ $? -ne 0 ]; then
                    LOGE "鍒涘缓鐩綍澶辫触锛?{certPath}/${CF_Domain}"
                    exit 1
                fi

                ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} \
                    --fullchain-file ${certPath}/${CF_Domain}/fullchain.pem \
                    --key-file ${certPath}/${CF_Domain}/privkey.pem

                if [ $? -ne 0 ]; then
                    LOGE "璇佷功瀹夎澶辫触锛岃剼鏈凡閫€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功瀹夎鎴愬姛锛屾鍦ㄥ紑鍚嚜鍔ㄦ洿鏂?.."
                fi

                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                if [ $? -ne 0 ]; then
                    LOGE "鑷姩鏇存柊璁剧疆澶辫触锛岃剼鏈凡閫€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功宸插畨瑁咃紝骞跺凡寮€鍚嚜鍔ㄧ画绛俱€?
                    ls -lah ${certPath}/${CF_Domain}
                    chmod 755 ${certPath}/${CF_Domain}
                fi
            fi
            show_menu
            ;;
        3)
            echo "姝ｅ湪杩斿洖涓昏彍鍗?.."
            show_menu
            ;;
        *)
            echo "鏃犳晥閫夋嫨锛岃閲嶆柊閫夋嫨銆?
            show_menu
            ;;
    esac
}

generate_self_signed_cert() {
    cert_dir="/etc/sing-box"
    mkdir -p "$cert_dir"
    LOGI "璇烽€夋嫨璇佷功绫诲瀷锛?
    echo -e "${green}\t1.${plain} Ed25519锛堟帹鑽愶級"
    echo -e "${green}\t2.${plain} RSA 2048"
    echo -e "${green}\t3.${plain} RSA 4096"
    echo -e "${green}\t4.${plain} ECDSA prime256v1"
    echo -e "${green}\t5.${plain} ECDSA secp384r1"
    read -p "璇疯緭鍏ラ€夐」 [1-5锛岄粯璁?1]锛? cert_type
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

    LOGI "姝ｅ湪鐢熸垚鑷鍚嶈瘉涔︼紙$algo锛?.."
    sudo openssl req -x509 -nodes -days 3650 $key_opt \
        -keyout "${cert_dir}/self.key" \
        -out "${cert_dir}/self.crt" \
        -subj "/CN=myserver"
    if [[ $? -eq 0 ]]; then
        sudo chmod 600 "${cert_dir}/self."*
        LOGI "鑷鍚嶈瘉涔︾敓鎴愭垚鍔燂紒"
        LOGI "璇佷功璺緞锛?{cert_dir}/self.crt"
        LOGI "瀵嗛挜璺緞锛?{cert_dir}/self.key"
    else
        LOGE "鐢熸垚鑷鍚嶈瘉涔﹀け璐ャ€?
    fi
    before_show_menu
}

show_usage() {
    echo -e "S-UI 绠＄悊鑴氭湰浣跨敤璇存槑"
    echo -e "------------------------------------------"
    echo -e "瀛愬懡浠わ細"
    echo -e "s-ui              - 鎵撳紑绠＄悊鑿滃崟"
    echo -e "s-ui start        - 鍚姩 s-ui"
    echo -e "s-ui stop         - 鍋滄 s-ui"
    echo -e "s-ui restart      - 閲嶅惎 s-ui"
    echo -e "s-ui status       - 鏌ョ湅 s-ui 褰撳墠鐘舵€?
    echo -e "s-ui enable       - 璁剧疆寮€鏈鸿嚜鍚?
    echo -e "s-ui disable      - 鍙栨秷寮€鏈鸿嚜鍚?
    echo -e "s-ui log          - 鏌ョ湅 s-ui 鏃ュ織"
    echo -e "s-ui update       - 鏇存柊"
    echo -e "s-ui install      - 瀹夎"
    echo -e "s-ui uninstall    - 鍗歌浇"
    echo -e "s-ui help         - 鏌ョ湅浣跨敤璇存槑"
    echo -e "------------------------------------------"
}

show_menu() {
  echo -e "
  ${green}S-UI 绠＄悊鑴氭湰 ${plain}
鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}0.${plain} 閫€鍑?鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}1.${plain} 瀹夎
  ${green}2.${plain} 鏇存柊
  ${green}3.${plain} 鑷畾涔夌増鏈?  ${green}4.${plain} 鍗歌浇
鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}5.${plain} 閲嶇疆绠＄悊鍛樿处鍙峰瘑鐮佷负榛樿鍊?  ${green}6.${plain} 璁剧疆绠＄悊鍛樿处鍙峰瘑鐮?  ${green}7.${plain} 鏌ョ湅绠＄悊鍛樿处鍙蜂俊鎭?鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}8.${plain} 閲嶇疆闈㈡澘璁剧疆
  ${green}9.${plain} 璁剧疆闈㈡澘璁剧疆
  ${green}10.${plain} 鏌ョ湅闈㈡澘璁剧疆鍜岃闂湴鍧€
鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}11.${plain} 鍚姩 S-UI
  ${green}12.${plain} 鍋滄 S-UI
  ${green}13.${plain} 閲嶅惎 S-UI
  ${green}14.${plain} 鏌ョ湅 S-UI 鐘舵€?  ${green}15.${plain} 鏌ョ湅 S-UI 鏃ュ織
  ${green}16.${plain} 璁剧疆 S-UI 寮€鏈鸿嚜鍚?  ${green}17.${plain} 鍙栨秷 S-UI 寮€鏈鸿嚜鍚?鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€?  ${green}18.${plain} 鍚敤鎴栫鐢?BBR
  ${green}19.${plain} SSL 璇佷功绠＄悊
  ${green}20.${plain} Cloudflare SSL 璇佷功
鈥斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€斺€? "
    show_status s-ui
    echo && read -p "璇疯緭鍏ラ€夐」 [0-20]锛? num

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
        LOGE "璇疯緭鍏ユ纭殑鏁板瓧 [0-20]"
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
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
