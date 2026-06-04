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
        echo "en"
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
    /usr/local/s-ui/sui migrate

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
        /usr/local/s-ui/sui setting "${params[@]}"

        read -r -p "$(prompt_text "是否修改管理员账号密码 [y/n]? " "Do you want to change admin credentials [y/n]? ")" admin_confirm
        if is_yes "${admin_confirm}"; then
            # First admin credentials
            read -r -p "$(prompt_text "请设置管理员用户名：" "Please set up your username:")" config_account
            read -r -p "$(prompt_text "请设置管理员密码：" "Please set up your password:")" config_password

            # Set credentials
            say "${yellow}正在初始化，请稍候...${plain}\n" "${yellow}Initializing, please wait...${plain}\n"
            /usr/local/s-ui/sui admin -username "${config_account}" -password "${config_password}"
        else
            say "${yellow}当前管理员账号信息：${plain}\n" "${yellow}Your current admin credentials: ${plain}\n"
            /usr/local/s-ui/sui admin -show
        fi
    else
        say "${red}已取消...${plain}\n" "${red}cancel...${plain}\n"
        if [[ ! -f "/usr/local/s-ui/db/s-ui.db" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head -c 6 /dev/urandom | base64)
            say "检测到全新安装，将为安全起见生成随机登录信息：\n" "this is a fresh installation,will generate random login info for security concerns:\n"
            echo -e "###############################################"
            say "${green}用户名：%s${plain}\n" "${green}username:%s${plain}\n" "$usernameTemp"
            say "${green}密码：%s${plain}\n" "${green}password:%s${plain}\n" "$passwordTemp"
            echo -e "###############################################"
            say "${red}如果忘记登录信息，可以输入 ${green}s-ui${red} 打开配置菜单。${plain}\n" "${red}if you forgot your login info,you can type ${green}s-ui${red} for configuration menu${plain}\n"
            /usr/local/s-ui/sui admin -username "${usernameTemp}" -password "${passwordTemp}"
        else
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

    chmod +x s-ui-linux-$(arch)/sui s-ui-linux-$(arch)/s-ui.sh
    cp s-ui-linux-$(arch)/s-ui.sh /usr/bin/s-ui
    mkdir -p /usr/local/s-ui
    cp -rf s-ui-linux-$(arch)/* /usr/local/s-ui/
    cp -f s-ui-linux-$(arch)/*.service /etc/systemd/system/
    rm -rf s-ui-linux-$(arch)

    config_after_install
    prepare_services

    systemctl enable s-ui --now

    say "${green}s-ui %s${plain} 安装完成，当前已启动运行。\n" "${green}s-ui %s${plain} installation finished, it is up and running now...\n" "$last_version"
    say "可通过以下 URL 访问面板：${green}\n" "You may access the Panel with following URL(s):${green}\n"
    /usr/local/s-ui/sui uri
    echo -e "${plain}"
    echo -e ""
    s-ui help
}

say "${green}正在执行...${plain}\n" "${green}Executing...${plain}\n"
install_base
install_s-ui "$@"
