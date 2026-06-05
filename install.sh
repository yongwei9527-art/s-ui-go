#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

script_args=()
while [[ $# -gt 0 ]]; do
    case "$1" in
    --lang=*)
        SUI_LANG="${1#*=}"
        ;;
    --lang | -l)
        if [[ $# -lt 2 ]]; then
            echo "Missing language value after $1 / 缺少语言参数"
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
    lang=$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')
    case "$lang" in
    zh | zh-cn | zh_cn | zh-hans | zh_hans | cn | chinese | 中文)
        echo "zh"
        ;;
    en | en-us | en_us | english)
        echo "en"
        ;;
    *)
        echo ""
        ;;
    esac
}

detect_default_language() {
    local env_lang="${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}"
    env_lang=$(printf '%s' "$env_lang" | tr '[:upper:]' '[:lower:]')
    if [[ "$env_lang" == zh* ]]; then
        echo "zh"
    else
        echo "zh"
    fi
}

select_install_language() {
    local normalized_lang default_lang default_choice lang_choice
    normalized_lang=$(normalize_language "${SUI_LANG:-}")
    if [[ -n "$normalized_lang" ]]; then
        INSTALL_LANG="$normalized_lang"
        return
    fi

    default_lang=$(detect_default_language)
    default_choice="2"
    [[ "$default_lang" == "zh" ]] && default_choice="1"

    echo -e "${green}请选择安装语言 / Please choose installation language${plain}"
    echo -e "${green}1.${plain} 中文"
    echo -e "${green}2.${plain} English"
    if [[ -t 0 ]]; then
        read -r -p "请输入选项 / Enter choice [1-2] (default: ${default_choice}): " lang_choice
    fi
    lang_choice="${lang_choice:-$default_choice}"

    case "$lang_choice" in
    1 | zh | ZH | cn | CN | 中文)
        INSTALL_LANG="zh"
        ;;
    *)
        INSTALL_LANG="en"
        ;;
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

prompt_text() {
    if is_zh; then
        printf "%s" "$1"
    else
        printf "%s" "$2"
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

print_generated_urls() {
    local server_ip server_host panel_port panel_path sub_port sub_path
    server_ip="$(get_server_ip)"
    panel_port="${config_port:-2095}"
    panel_path="$(normalize_url_path "${config_path:-/app/}")"
    sub_port="${config_subPort:-2096}"
    sub_path="$(normalize_url_path "${config_subPath:-/sub/}")"

    if [[ -n "$server_ip" ]]; then
        server_host="$(format_host "$server_ip")"
        say "自动生成的面板完整地址：${green}http://%s:%s%s${plain}\n" "Generated panel URL: ${green}http://%s:%s%s${plain}\n" "$server_host" "$panel_port" "$panel_path"
        say "自动生成的订阅完整地址：${green}http://%s:%s%s${plain}\n" "Generated subscription URL: ${green}http://%s:%s%s${plain}\n" "$server_host" "$sub_port" "$sub_path"
    else
        say "未能自动读取服务器 IP，请将 localhost 替换为服务器公网 IP。\n" "Failed to detect server IP automatically. Please replace localhost with your server public IP.\n"
    fi
}

is_yes() {
    local answer lower_answer
    answer="${1:-}"
    lower_answer=$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]')
    case "$lower_answer" in
    y | yes | true | 1)
        return 0
        ;;
    esac
    case "$answer" in
    是 | 好 | 确定 | 確定)
        return 0
        ;;
    esac
    return 1
}

print_sui_command_output() {
    local output="$1"
    if [[ -n "$output" ]]; then
        printf '%s\n' "$output"
    fi
}

print_sui_failure_hint() {
    local output="$1"
    print_sui_command_output "$output"
}

run_sui_required() {
    local zh_desc="$1"
    local en_desc="$2"
    local output=""
    shift 2
    if ! output=$(/usr/local/s-ui/sui "$@" 2>&1); then
        if is_zh; then
            printf "${red}%s失败。${plain}\n" "$zh_desc"
        else
            printf "${red}%s failed.${plain}\n" "$en_desc"
        fi
        print_sui_failure_hint "$output"
        exit 1
    fi
    print_sui_command_output "$output"
}

run_sui_required_silent() {
    local zh_desc="$1"
    local en_desc="$2"
    local output=""
    shift 2
    if ! output=$(/usr/local/s-ui/sui "$@" 2>&1); then
        if is_zh; then
            printf "${red}%s失败。${plain}\n" "$zh_desc"
        else
            printf "${red}%s failed.${plain}\n" "$en_desc"
        fi
        print_sui_failure_hint "$output"
        exit 1
    fi
}

run_sui_optional() {
    local zh_desc="$1"
    local en_desc="$2"
    local output=""
    shift 2
    if ! output=$(/usr/local/s-ui/sui "$@" 2>&1); then
        if is_zh; then
            printf "${yellow}%s未完成，可能是全新安装或旧数据库不存在。${plain}\n" "$zh_desc"
        else
            printf "${yellow}%s was not completed, possibly because this is a fresh install or the old database does not exist.${plain}\n" "$en_desc"
        fi
        print_sui_command_output "$output"
        return 0
    fi
    print_sui_command_output "$output"
}

ensure_localized_manager_script() {
    local script_path="$1"
    if [[ ! -f "$script_path" ]]; then
        say "${red}发布包内缺少 s-ui.sh 管理脚本，无法继续安装。请重新打包并确保 Linux 发布包包含中文管理脚本。${plain}\n" "${red}s-ui.sh management script is missing from the package. Cannot continue. Please rebuild the Linux package with the localized management script included.${plain}\n"
        exit 1
    fi
    if ! grep -q "S-UI 管理脚本" "$script_path" || ! grep -q "请输入选项 \[0-20\]" "$script_path"; then
        say "${red}检测到 s-ui.sh 管理脚本仍为旧英文版，已停止安装以避免落地英文菜单。请先更新 latest-project-upload 分支中的 s-ui.sh，或重新打包包含中文管理脚本的 Linux 发布包。${plain}\n" "${red}Detected an old English s-ui.sh management script. Installation stopped to avoid installing the English menu. Please update s-ui.sh in the latest-project-upload branch, or rebuild the Linux package with the localized management script.${plain}\n"
        exit 1
    fi
}

select_install_language

# check root
[[ $EUID -ne 0 ]] && say "${red}致命错误：${plain}请使用 root 权限运行此脚本\n" "${red}Fatal error:${plain} Please run this script with root privilege\n" && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    say "检测系统发行版失败，请联系作者！\n" "Failed to check the system OS, please contact the author!\n" >&2
    exit 1
fi
say "系统发行版：%s\n" "The OS release is: %s\n" "$release"

arch() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm | armv6* | armv6 | armv5* | armv5) echo 'arm' ;;
    s390x) echo 's390x' ;;
    *) say "${green}不支持的 CPU 架构！${plain}\n" "${green}Unsupported CPU architecture! ${plain}\n" && rm -f install.sh && exit 1 ;;
    esac
}

say "CPU 架构：%s\n" "arch: %s\n" "$(arch)"

install_base() {
    case "${release}" in
    centos | almalinux | rocky | oracle)
        yum -y update && yum install -y -q wget curl tar unzip tzdata
        ;;
    fedora)
        dnf -y update && dnf install -y -q wget curl tar unzip tzdata
        ;;
    arch | manjaro | parch)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar unzip tzdata
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y wget curl tar unzip timezone
        ;;
    *)
        apt-get update && apt-get install -y -q wget curl tar unzip tzdata
        ;;
    esac
}

config_after_install() {
    say "${yellow}正在执行数据库迁移...${plain}\n" "${yellow}Migration... ${plain}\n"
    run_sui_optional "数据库迁移" "database migration" migrate

    say "${yellow}安装/更新完成！为了安全，建议修改面板配置。${plain}\n" "${yellow}Install/update finished! For security it's recommended to modify panel settings ${plain}\n"
    read -r -p "$(prompt_text "是否继续修改配置 [y/n]? " "Do you want to continue with the modification [y/n]? ")" config_confirm
    if is_yes "${config_confirm}"; then
        say "请输入${yellow}面板端口${plain}（留空则使用当前/默认值）：\n" "Enter the ${yellow}panel port${plain} (leave blank for existing/default value):\n"
        read -r config_port
        say "请输入${yellow}面板路径${plain}（留空则使用当前/默认值）：\n" "Enter the ${yellow}panel path${plain} (leave blank for existing/default value):\n"
        read -r config_path

        # Sub configuration
        say "请输入${yellow}订阅端口${plain}（留空则使用当前/默认值）：\n" "Enter the ${yellow}subscription port${plain} (leave blank for existing/default value):\n"
        read -r config_subPort
        say "请输入${yellow}订阅路径${plain}（留空则使用当前/默认值）：\n" "Enter the ${yellow}subscription path${plain} (leave blank for existing/default value):\n"
        read -r config_subPath

        # Set configs
        say "${yellow}正在初始化，请稍候...${plain}\n" "${yellow}Initializing, please wait...${plain}\n"
        params=()
        [ -z "$config_port" ] || params+=("-port" "$config_port")
        [ -z "$config_path" ] || params+=("-path" "$config_path")
        [ -z "$config_subPort" ] || params+=("-subPort" "$config_subPort")
        [ -z "$config_subPath" ] || params+=("-subPath" "$config_subPath")
        run_sui_required "应用面板和订阅配置" "applying panel and subscription settings" setting "${params[@]}"

        read -r -p "$(prompt_text "是否修改管理员账号密码 [y/n]? " "Do you want to change admin credentials [y/n]? ")" admin_confirm
        if is_yes "${admin_confirm}"; then
            # First admin credentials
            read -r -p "$(prompt_text "请设置管理员用户名：" "Please set up your username:")" config_account
            read -r -p "$(prompt_text "请设置管理员密码：" "Please set up your password:")" config_password

            # Set credentials
            say "${yellow}正在初始化，请稍候...${plain}\n" "${yellow}Initializing, please wait...${plain}\n"
            run_sui_required "设置管理员账号密码" "setting admin credentials" admin -username "${config_account}" -password "${config_password}"
        else
            say "${yellow}当前管理员账号信息：${plain}\n" "${yellow}Your current admin credentials: ${plain}\n"
            run_sui_required "读取管理员账号信息" "reading admin credentials" admin -show
        fi
    else
        say "${red}已取消...${plain}\n" "${red}cancel...${plain}\n"
        if [[ ! -f "/usr/local/s-ui/db/s-ui.db" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head -c 6 /dev/urandom | base64)
            run_sui_required "生成随机管理员账号密码" "generating random admin credentials" admin -username "${usernameTemp}" -password "${passwordTemp}"
            say "检测到全新安装，将为安全起见生成随机登录信息：\n" "this is a fresh installation,will generate random login info for security concerns:\n"
            echo -e "###############################################"
            say "${green}用户名：%s${plain}\n" "${green}username:%s${plain}\n" "$usernameTemp"
            say "${green}密码：%s${plain}\n" "${green}password:%s${plain}\n" "$passwordTemp"
            echo -e "###############################################"
            say "${red}如果忘记登录信息，可以输入 ${green}s-ui${red} 打开配置菜单。${plain}\n" "${red}if you forgot your login info,you can type ${green}s-ui${red} for configuration menu${plain}\n"
        else
            run_sui_required_silent "检查现有面板配置" "checking existing panel settings" setting -show
            say "${red}这是升级安装，将保留旧配置；如果忘记登录信息，可以输入 ${green}s-ui${red} 打开配置菜单。${plain}\n" "${red} this is your upgrade,will keep old settings,if you forgot your login info,you can type ${green}s-ui${red} for configuration menu${plain}\n"
        fi
    fi
}

prepare_services() {
    if [[ -f "/etc/systemd/system/sing-box.service" ]]; then
        say "${yellow}正在停止 sing-box 服务...${plain}\n" "${yellow}Stopping sing-box service... ${plain}\n"
        systemctl stop sing-box
        rm -f /usr/local/s-ui/bin/sing-box /usr/local/s-ui/bin/runSingbox.sh /usr/local/s-ui/bin/signal
    fi
    if [[ -e "/usr/local/s-ui/bin" ]]; then
        echo -e "###############################################################"
        say "${green}/usr/local/s-ui/bin${red} 目录已存在！\n" "${green}/usr/local/s-ui/bin${red} directory exists yet!\n"
        say "请检查内容，并在迁移后手动删除。${plain}\n" "Please check the content and delete it manually after migration ${plain}\n"
        echo -e "###############################################################"
    fi
    systemctl daemon-reload
}

install_s-ui() {
    cd /tmp/

    if [[ $# -eq 0 ]]; then
        last_version=$(curl -Ls "https://api.github.com/repos/yongwei9527-art/s-ui-go/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            say "${red}获取 s-ui 最新版本失败，可能是 Github API 限制，请稍后重试。${plain}\n" "${red}Failed to fetch s-ui version, it maybe due to Github API restrictions, please try it later${plain}\n"
            exit 1
        fi
        say "已获取 s-ui 最新版本：%s，开始安装...\n" "Got s-ui latest version: %s, beginning the installation...\n" "$last_version"
        wget -N --no-check-certificate -O /tmp/s-ui-linux-$(arch).zip https://github.com/yongwei9527-art/s-ui-go/releases/download/${last_version}/s-ui-linux-$(arch).zip
        if [[ $? -ne 0 ]]; then
            say "${red}下载 s-ui 失败，请确认服务器可以访问 Github。${plain}\n" "${red}Downloading s-ui failed, please be sure that your server can access Github ${plain}\n"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/yongwei9527-art/s-ui-go/releases/download/${last_version}/s-ui-linux-$(arch).zip"
        say "开始安装 s-ui %s\n" "Beginning the install s-ui %s\n" "$last_version"
        wget -N --no-check-certificate -O /tmp/s-ui-linux-$(arch).zip ${url}
        if [[ $? -ne 0 ]]; then
            say "${red}下载 s-ui %s 失败，请检查该版本是否存在。${plain}\n" "${red}download s-ui %s failed,please check the version exists${plain}\n" "$last_version"
            exit 1
        fi
    fi

    if [[ -e /usr/local/s-ui/ ]]; then
        systemctl stop s-ui
    fi

    rm -rf s-ui-linux-$(arch)
    unzip -oq s-ui-linux-$(arch).zip -d s-ui-linux-$(arch)
    rm s-ui-linux-$(arch).zip -f

    manager_script_tmp="/tmp/s-ui-manager.sh"
    manager_script_url="https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/s-ui.sh"
    if wget -q --no-check-certificate -O "$manager_script_tmp" "$manager_script_url"; then
        cp -f "$manager_script_tmp" s-ui-linux-$(arch)/s-ui.sh
        rm -f "$manager_script_tmp"
        say "已更新中文管理脚本。\n" "Updated localized management script.\n"
    else
        rm -f "$manager_script_tmp"
        say "警告：中文管理脚本下载失败，将使用发布包内置脚本。\n" "Warning: failed to download localized management script, using bundled script instead.\n"
    fi

    ensure_localized_manager_script "s-ui-linux-$(arch)/s-ui.sh"

    chmod +x s-ui-linux-$(arch)/sui s-ui-linux-$(arch)/s-ui.sh
    cp s-ui-linux-$(arch)/s-ui.sh /usr/bin/s-ui
    mkdir -p /usr/local/s-ui
    cp -rf s-ui-linux-$(arch)/* /usr/local/s-ui/
    cat >/etc/systemd/system/s-ui.service <<EOF_SERVICE
[Unit]
Description=S-UI Proxy Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/usr/local/s-ui
ExecStart=/usr/local/s-ui/sui
Restart=on-failure
RestartSec=10
Environment=SUI_DB_FOLDER=db
Environment=SUI_DEBUG=false

[Install]
WantedBy=multi-user.target
EOF_SERVICE
    rm -rf s-ui-linux-$(arch)

    config_after_install
    prepare_services

    if ! systemctl enable s-ui --now; then
        say "${red}s-ui 服务启动失败，请查看下面的 systemd 状态信息。${plain}\n" "${red}s-ui service failed to start. Please check the systemd status below.${plain}\n"
        systemctl status s-ui -l --no-pager || true
        exit 1
    fi
    sleep 2
    if ! systemctl is-active --quiet s-ui; then
        say "${red}s-ui 服务未处于运行状态，请查看下面的 systemd 状态信息。${plain}\n" "${red}s-ui service is not running. Please check the systemd status below.${plain}\n"
        systemctl status s-ui -l --no-pager || true
        exit 1
    fi

    say "${green}s-ui %s${plain} 安装完成，当前已启动运行。\n" "${green}s-ui %s${plain} installation finished, it is up and running now...\n" "$last_version"
    print_generated_urls
    say "可通过以下 URL 访问面板：${green}\n" "You may access the Panel with following URL(s):${green}\n"
    run_sui_required "获取面板访问地址" "getting panel URL" uri
    echo -e "${plain}"
    echo -e ""
    s-ui help
}

say "${green}正在执行...${plain}\n" "${green}Executing...${plain}\n"
install_base
install_s-ui "$@"
