#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

readonly REPOSITORY="yongwei9527-art/s-ui-go"
readonly DEFAULT_INSTALL_DIR="/usr/local/s-ui"
INSTALL_LANG=""
REQUESTED_VERSION=""
INSTALL_DIR="$DEFAULT_INSTALL_DIR"
INSTALL_DIR_EXPLICIT=0
UPGRADE_REQUESTED=0
TEMP_DIR=""

red='\033[0;31m'
green='\033[0;32m'
plain='\033[0m'

cleanup() {
    if [[ -n "${TEMP_DIR:-}" && -d "$TEMP_DIR" ]]; then
        rm -rf -- "$TEMP_DIR"
    fi
}
trap cleanup EXIT INT TERM HUP

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
    if [[ "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" != zh* ]]; then
        default_choice="2"
    fi
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

say() {
    local zh_text="$1" en_text="$2"
    shift 2
    if [[ "$INSTALL_LANG" == "zh" ]]; then
        printf "$zh_text" "$@"
    else
        printf "$en_text" "$@"
    fi
}

die() {
    say "${red}错误：%s${plain}\n" "${red}Error: %s${plain}\n" "$1" >&2
    exit 1
}

usage() {
    cat <<'EOF'
Usage / 用法:
  bash install.sh [--lang zh|en] [--install-dir /usr/local/name|/opt/name|/srv/name] [--upgrade] [--version TAG]

The installer downloads one release only, verifies its matching ZIP and SHA-256
asset, then delegates to the verified packaged installer.
安装器只使用一个发布版本，校验同一发布中的 ZIP 与 SHA-256 后，再委派给已校验包内安装器。
EOF
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

validate_install_dir() {
    local raw="$1" normalized wrapped
    command -v realpath >/dev/null 2>&1 || die '需要 GNU coreutils 的 realpath 命令 / GNU realpath from coreutils is required.'
    if ! realpath -m -- / >/dev/null 2>&1; then
        die '需要支持 -m 的 GNU realpath / GNU realpath with -m support is required.'
    fi
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

while [[ $# -gt 0 ]]; do
    case "$1" in
        --lang=*) SUI_LANG="${1#*=}" ;;
        --lang|-l)
            [[ $# -ge 2 ]] || { printf 'Missing language value / 缺少语言参数\n' >&2; exit 1; }
            SUI_LANG="$2"
            shift
            ;;
        --version=*) REQUESTED_VERSION="${1#*=}" ;;
        --version)
            [[ $# -ge 2 ]] || { printf 'Missing version / 缺少版本参数\n' >&2; exit 1; }
            REQUESTED_VERSION="$2"
            shift
            ;;
        --install-dir=*)
            INSTALL_DIR="${1#*=}"
            INSTALL_DIR_EXPLICIT=1
            ;;
        --install-dir)
            [[ $# -ge 2 ]] || { printf 'Missing installation directory / 缺少安装目录参数\n' >&2; exit 1; }
            INSTALL_DIR="$2"
            INSTALL_DIR_EXPLICIT=1
            shift
            ;;
        --upgrade) UPGRADE_REQUESTED=1 ;;
        --help|-h) usage; exit 0 ;;
        --*)
            printf 'Unsupported option / 不支持的参数: %s\n' "$1" >&2
            exit 1
            ;;
        *)
            [[ -z "$REQUESTED_VERSION" ]] || { printf 'Only one release version may be specified / 只能指定一个发布版本\n' >&2; exit 1; }
            REQUESTED_VERSION="$1"
            ;;
    esac
    shift
done

select_install_language

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
    die '请使用 root 权限运行此安装器 / run this installer as root.'
fi

if [[ -f /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
else
    die '无法识别 Linux 发行版 / unable to identify the Linux distribution.'
fi

platform_arch() {
    case "$(uname -m)" in
        x86_64|x64|amd64) printf 'amd64' ;;
        arm64|aarch64) printf 'arm64' ;;
        armv7*|armv6*|arm) printf 'arm' ;;
        *) die "不支持的 CPU 架构：$(uname -m) / unsupported CPU architecture." ;;
    esac
}

install_dependencies() {
    local command_name
    say '正在安装校验与部署依赖...\n' 'Installing verification and deployment dependencies...\n'
    case "${ID:-}" in
        ubuntu|debian|linuxmint)
            apt-get update
            DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
                ca-certificates curl unzip openssl iproute2 coreutils jq
            ;;
        centos|rhel|almalinux|rocky|ol)
            yum install -y ca-certificates curl unzip openssl iproute coreutils jq
            ;;
        fedora)
            dnf install -y ca-certificates curl unzip openssl iproute coreutils jq
            ;;
        arch|manjaro)
            pacman -Sy --noconfirm ca-certificates curl unzip openssl iproute2 coreutils jq
            ;;
    esac
    for command_name in curl unzip sha256sum realpath openssl timeout jq; do
        command -v "$command_name" >/dev/null 2>&1 || die "缺少命令 $command_name；请使用系统包管理器安装。 / Missing command $command_name."
    done
}

install_dependencies
validate_install_dir "$INSTALL_DIR"

# Preserve a legacy /opt/s-ui installation only when the caller did not select
# a directory. The candidate is normalized and validated before it is inspected.
if [[ "$INSTALL_DIR_EXPLICIT" -eq 0 && ! -f "$INSTALL_DIR/db/s-ui.db" && -f /opt/s-ui/db/s-ui.db ]]; then
    validate_install_dir /opt/s-ui
fi

ARCH="$(platform_arch)"
ASSET_NAME="s-ui-linux-${ARCH}.zip"
# GitHub's /releases/latest excludes prereleases. Always resolve both the
# default and --version paths from the release list. The newest non-draft
# release is authoritative and must itself contain the selected ZIP and its
# checksum; the installer never falls back to an older release.
RELEASE_API_URL="https://api.github.com/repos/${REPOSITORY}/releases?per_page=100"
RELEASE_JSON="$(curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL --retry 3 --connect-timeout 15 --max-time 120 "$RELEASE_API_URL")" || die 'Unable to fetch the release list.'
if [[ -n "$REQUESTED_VERSION" ]]; then
    [[ "$REQUESTED_VERSION" =~ ^[A-Za-z0-9._-]+$ ]] || die 'Version tag contains unsafe characters.'
    if ! CANDIDATE_RELEASE="$(jq -ce --arg tag "$REQUESTED_VERSION" '[.[] | select(.draft != true) | select(.tag_name == $tag)] | sort_by(.published_at // .created_at // "") | reverse | .[0] // empty' <<< "$RELEASE_JSON")"; then
        die 'The requested release is missing, invalid, or still a draft.'
    fi
else
    if ! CANDIDATE_RELEASE="$(jq -ce '[.[] | select(.draft != true)] | sort_by(.published_at // .created_at // "") | reverse | .[0] // empty' <<< "$RELEASE_JSON")"; then
        die 'The release list has no installable non-draft release.'
    fi
fi

RELEASE_VERSION="$(jq -er '.tag_name // empty' <<< "$CANDIDATE_RELEASE")" || die '发布元数据缺少 tag_name。 / Release metadata is missing tag_name.'
[[ "$RELEASE_VERSION" =~ ^[A-Za-z0-9._-]+$ ]] || die 'Selected release tag contains unsupported characters.'
mapfile -t ASSET_URLS < <(jq -r --arg name "$ASSET_NAME" '.assets[]? | select(.name == $name) | .browser_download_url // empty' <<< "$CANDIDATE_RELEASE")
mapfile -t CHECKSUM_URLS < <(jq -r --arg name "${ASSET_NAME}.sha256" '.assets[]? | select(.name == $name) | .browser_download_url // empty' <<< "$CANDIDATE_RELEASE")
if [[ "${#ASSET_URLS[@]}" -ne 1 || "${#CHECKSUM_URLS[@]}" -ne 1 ]]; then
    die "Newest selected release ${RELEASE_VERSION} is incomplete: it must publish exactly one ${ASSET_NAME} and one ${ASSET_NAME}.sha256; refusing to fall back to an older release."
fi
ASSET_URL="${ASSET_URLS[0]}"
CHECKSUM_URL="${CHECKSUM_URLS[0]}"
EXPECTED_ASSET_URL="https://github.com/${REPOSITORY}/releases/download/${RELEASE_VERSION}/${ASSET_NAME}"
EXPECTED_CHECKSUM_URL="${EXPECTED_ASSET_URL}.sha256"
if [[ "$ASSET_URL" != "$EXPECTED_ASSET_URL" || "$CHECKSUM_URL" != "$EXPECTED_CHECKSUM_URL" ]]; then
    die "Selected release ${RELEASE_VERSION} has unexpected asset URLs; refusing the download."
fi
say '??????? s-ui-go %s...\n' 'Downloading and verifying s-ui-go %s...\n' "$RELEASE_VERSION"
TEMP_DIR="$(mktemp -d /tmp/s-ui-go-install.XXXXXX)"
ARCHIVE="$TEMP_DIR/$ASSET_NAME"
CHECKSUM_FILE="$TEMP_DIR/${ASSET_NAME}.sha256"
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fL --retry 3 --connect-timeout 15 --max-time 600 -o "$ARCHIVE" "$ASSET_URL" || die 'Unable to download the release ZIP.'
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fL --retry 3 --connect-timeout 15 --max-time 120 -o "$CHECKSUM_FILE" "$CHECKSUM_URL" || die 'Unable to download the release checksum.'

mapfile -t EXPECTED_HASHES < <(LC_ALL=C awk -v name="$ASSET_NAME" 'NF == 2 { file=$2; sub(/^\*/, "", file); if (file == name) print $1 }' "$CHECKSUM_FILE")
if [[ "${#EXPECTED_HASHES[@]}" -ne 1 || ! "${EXPECTED_HASHES[0]}" =~ ^[0-9A-Fa-f]{64}$ ]]; then
    die 'The checksum file must contain exactly one valid SHA-256 entry for the release ZIP.'
fi
EXPECTED_HASH="${EXPECTED_HASHES[0]}"
ACTUAL_HASH="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
if [[ "${ACTUAL_HASH,,}" != "${EXPECTED_HASH,,}" ]]; then
    die 'SHA-256 ??????????? / SHA-256 verification failed; installation was aborted.'
fi

unzip -tq "$ARCHIVE" >/dev/null || die '?? ZIP ?? / release ZIP is corrupt.'
ZIP_ENTRIES="$(unzip -Z1 "$ARCHIVE")" || die 'Unable to inspect release ZIP entries.'
if [[ -z "$ZIP_ENTRIES" ]] || grep -Eq '(^/|(^|/)\.\.(/|$)|(^|/)\./|\\)' <<< "$ZIP_ENTRIES"; then
    die '????????????????? / release archive contains an unsafe path.'
fi
DUPLICATE_ENTRY="$(printf '%s\n' "$ZIP_ENTRIES" | LC_ALL=C sort | uniq -d | sed -n '1p')"
[[ -z "$DUPLICATE_ENTRY" ]] || die 'Release archive contains duplicate paths.'
TOPLEVEL_INSTALLER_COUNT="$(awk '$0 == "install-linux.sh" { count++ } END { print count + 0 }' <<< "$ZIP_ENTRIES")"
[[ "$TOPLEVEL_INSTALLER_COUNT" == 1 ]] || die 'Release archive must contain exactly one top-level install-linux.sh.'
unzip -q "$ARCHIVE" -d "$TEMP_DIR/package" || die 'Unable to extract the verified release ZIP.'
INSTALLER_PATH="$TEMP_DIR/package/install-linux.sh"
if [[ ! -d "$TEMP_DIR/package" || -L "$TEMP_DIR/package" || ! -f "$INSTALLER_PATH" || -L "$INSTALLER_PATH" ]]; then
    die '???????? install-linux.sh? / release archive is missing a safe install-linux.sh.'
fi

INSTALLER_ARGS=("--lang=$INSTALL_LANG" "--install-dir=$INSTALL_DIR")
if [[ "$UPGRADE_REQUESTED" -eq 1 || -f "$INSTALL_DIR/db/s-ui.db" ]]; then
    INSTALLER_ARGS+=("--upgrade")
fi
bash "$INSTALLER_PATH" "${INSTALLER_ARGS[@]}"
say "${green}s-ui-go %s 安装器已成功完成；已验证发布 ZIP 与 SHA-256。请以包内安装器的实际健康检查结果为准。${plain}\n" "${green}s-ui-go %s installer completed successfully; the release ZIP and SHA-256 were verified. Refer to the packaged installer's actual health-check result.${plain}\n" "$RELEASE_VERSION"
