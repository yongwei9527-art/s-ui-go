# s-ui-go 0.1

> 基于开源 S-UI / s-ui-go 项目的社区维护构建，提供 Windows、Linux、macOS 发布包、安装脚本、服务管理脚本及相关配置说明。
>
> **版本约定：** 用户可见名称为 **s-ui-go 0.1**；当前更新包包含 sing-box `v1.14.3` 和 Xray-core `v26.3.27`，包含默认节点 TLS 与订阅兼容性修复。旧 Release `s-ui-go-v0.1.1` 尚未替换，请使用下方更新包。文档中的 `sui` 是程序文件名，`s-ui` 是 Linux 服务/管理命令。

[![Release](https://img.shields.io/badge/release-s--ui--go%200.1-blue)](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.1)
[![License](https://img.shields.io/badge/license-GPL--3.0-green)](LICENSE)
## 目录

- [项目说明](#项目说明)
- [维护内容与差异](#维护内容与差异)
- [项目压缩包](#项目压缩包)
- [下载](#下载)
- [快速安装](#快速安装)
- [默认访问地址](#默认访问地址)
- [协议选择建议](#协议选择建议)
- [SNI 域名参考](#sni-域名参考)
- [从源码构建](#从源码构建)
- [安全提醒](#安全提醒)
- [English Summary](#english-summary)
- [致谢与许可](#致谢与许可)

## 项目说明

本仓库面向 `yongwei9527-art/s-ui-go` 发布，主要维护以下内容：

- Windows / Linux / macOS 三端 amd64 发布压缩包
- 中文 / English Web 面板体验
- Windows、Linux、macOS 安装、卸载和服务管理脚本
- TLS / ECH / WebSocket / TUIC / Hysteria2 订阅转换兼容性

> 说明：本项目是社区整理版本，非官方原版。生产环境使用前请自行评估风险。

## 维护内容与差异

主要维护内容如下：

| 维护方向 | 具体内容 |
| --- | --- |
| 跨平台发布 | 整理 Windows、Linux、macOS 三端 amd64 发布包，并统一压缩包命名。 |
| 安装与卸载 | 补充和整理 Windows、Linux、macOS 的安装、卸载、服务管理脚本。 |
| 安装后汉化 | 将 Linux 安装流程、常用提示和 `s-ui` 管理菜单改为默认中文显示。 |
| 自动生成访问地址 | 安装完成后自动读取服务器公网 IP，并生成 `http://服务器IP:端口/路径/` 形式的完整面板地址和订阅地址。 |
| Windows 构建 | 完善 Windows PowerShell 构建脚本，支持指定系统/架构、打包、跳过前端构建、非交互构建和清理候选查看。 |
| README 文档 | 梳理 README 结构，加入目录、下载链接、快速安装、默认访问地址、源码构建、验证运行、脚本说明、安全提醒、许可说明等内容。 |
| Linux 安装说明 | 单独补充 Debian / Ubuntu、CentOS / Rocky / AlmaLinux / Oracle Linux 的安装步骤和防火墙端口提示。 |
| 协议选择指南 | 新增 [协议组合建议方案](PROTOCOL_GUIDE.md)，按自建 VPS、CDN、弱网、全局代理、软路由等场景给出推荐组合。 |
| ACME / TLS 配置 | 修正 ACME 默认 `server_name` 取值逻辑，改为优先使用第一个域名。 |
| 首次初始化默认模板 | 全新安装后自动生成 Reality/TLS 模板、VLESS Reality、Hysteria2、TUIC、Trojan TLS、VMess WS TLS、Shadowsocks 2022、Naive TLS、AnyTLS、Hysteria TLS 默认入站，以及绑定这些入站的默认客户端；导出地址会在 VPS 初始化时自动检测服务器地址，避免写死 IP。Mixed/SOCKS/HTTP 不默认公网创建，避免开放代理风险。 |
| 运行稳定性加固 | 加强 sing-box core start/stop 并发保护、corePtr 空值保护、cron panic recover、订阅服务异常日志和默认端口预检查，降低 VPS 运行时面板或 core 崩溃风险。 |
| Go 静态检查清理 | 根据 IDE 提示处理 `database/backup.go` 与 `service/dns_leak_guard.go` 中的 QF1003 建议，保持逻辑不变并提升可读性。 |
| 发布与合规提示 | 补充 Release 版本、发布说明、致谢、GPL-3.0 许可和“非官方社区版”风险提醒。 |

上述调整的目标是提升跨平台交付、安装维护、协议配置说明、TLS/ACME 默认配置、首次启动可用性和 VPS 运行稳定性。

## 项目压缩包

每个 Release 提供三种 **amd64（x86_64）** 发布压缩包；请按操作系统和 CPU 架构选择。Apple Silicon、ARM Linux/Windows 不应使用这些 amd64 包，应改用对应架构的资产或自行构建。

| 压缩包 | 适用平台 | 解压后使用 |
| --- | --- | --- |
| `s-ui-windows-amd64.zip` | 64 位 Windows 10/11、Windows Server（x86_64） | 使用资源管理器或 7-Zip 解压；在解压目录中右键 **以管理员身份运行** `install-windows.bat`。 |
| `s-ui-linux-amd64.zip` | 使用 systemd 的 64 位 Linux 服务器（`uname -m` 为 `x86_64`） | 先下载同名 `.sha256` 校验 ZIP；执行 `unzip -q s-ui-linux-amd64.zip -d package`，再进入 `package` 并运行 `sudo bash ./install-linux.sh`。 |
| `s-ui-macos-amd64.zip` | Intel Mac（x86_64，非 Apple Silicon） | 执行 `unzip -q s-ui-macos-amd64.zip -d package`，进入 `package` 后运行 `sudo bash ./install-macos.sh`。 |

> 压缩包内包含对应平台的程序、安装/卸载脚本以及发布说明。下载位置见下方「下载」表中的固定 GitHub Release 资产链接；不要混用其他平台包或从未校验的第三方镜像下载。

Linux 下载 ZIP 与同名校验文件后，可在下载目录执行以下 SHA-256 校验（校验值由同名文件提供，无需手工填写或臆造哈希）：

```bash
sha256sum --check s-ui-linux-amd64.zip.sha256
```

## 下载

当前用户可见版本：**s-ui-go 0.1**
更新包分支：**`latest-project-upload`**（sing-box `v1.14.3` / Xray-core `v26.3.27`）
以下链接下载本分支三平台安装包；旧 Release `s-ui-go-v0.1.1` 的资产不包含此次更新。

| 系统 | 架构 | 发布资产 |
| --- | --- | --- |
| Windows | amd64 | [`s-ui-windows-amd64.zip`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-windows-amd64.zip) + [`SHA256`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-windows-amd64.zip.sha256) |
| Linux | amd64 | [`s-ui-linux-amd64.zip`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-linux-amd64.zip) + [`SHA256`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-linux-amd64.zip.sha256) |
| macOS | amd64 | [`s-ui-macos-amd64.zip`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-macos-amd64.zip) + [`SHA256`](https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist/s-ui-macos-amd64.zip.sha256) |

每个平台的发布单元均为 **ZIP 与同名 ZIP 的 `.sha256` 文件**。先校验 ZIP，再解压并使用包内的安装脚本或 `sui`；不要把一个不存在的“裸二进制校验文件”当作发布契约。
## 快速安装

### Windows

1. 下载并解压 `s-ui-windows-amd64.zip`。
2. 右键 `install-windows.bat`，选择 **以管理员身份运行**。
3. 按提示完成初始化；以后双击 `s-ui-windows.bat` 管理服务。

### Linux（Ubuntu / systemd）

Linux 的安装器要求 **systemd**。推荐 Ubuntu 22.04/24.04 等带 systemd 的 VPS，且 `uname -m` 为 `x86_64`；安装后的默认目录是 `/usr/local/s-ui`。

已有安装更新：需要 `curl`、`unzip` 和 `sha256sum`，复制整个代码块执行。默认升级目录为 `/usr/local/s-ui`：

```bash
(
  set -eu
  workdir="$(mktemp -d)"
  cd "$workdir"
  base='https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/dist'
  curl -fL --retry 3 -O "$base/s-ui-linux-amd64.zip"
  curl -fL --retry 3 -O "$base/s-ui-linux-amd64.zip.sha256"
  sha256sum --check s-ui-linux-amd64.zip.sha256
  unzip -q s-ui-linux-amd64.zip
  sudo bash ./install-linux.sh --lang zh --upgrade --install-dir /usr/local/s-ui
)
```

这些命令下载本分支最新安装包，校验成功后才执行包内安装器。升级保留数据库和配置；如果原安装目录是 `/opt/s-ui`，请把最后一行的目录改为 `/opt/s-ui`。全新安装时去掉 `--upgrade`，其余参数不变。下载临时目录保留用于排查。完成后，使用以下命令查看服务状态：

```bash
sudo systemctl status s-ui --no-pager
```

全新安装通过严格检查后，会启用下列默认入站：

   | 核心 | 默认入站 | 端口 |
   | --- | --- | --- |
   | sing-box | VLESS Reality | `443/tcp` |
   | 内置 Xray-core | VLESS Reality TCP Vision | `10443/tcp` |
   | 内置 Xray-core | VLESS Reality XHTTP | `2088/tcp` |

   若严格检查、安装或启动失败，安装器会停止并回滚本次变更；已有数据库升级会保留既有面板、管理员和协议配置，并使用安装前备份恢复失败状态。只有明确接受 Xray 默认模板保持关闭的高级场景才应使用 `--allow-manual-xray`。

安装器**不会承诺自动开放**管理面板/订阅端口 `2095/2096`；推荐将它们限制为本机并经 HTTPS 反代或 SSH 转发访问。无论本机 UFW 状态如何，云厂商安全组/云防火墙都需要你自行确认 `443/tcp`、`10443/tcp`、`2088/tcp` 是否按实际需求放行。完整的 Ubuntu 部署与恢复说明见 [Ubuntu 部署指南](UBUNTU_DEPLOYMENT.md)。
### macOS

下载 `s-ui-macos-amd64.zip` 并解压后，在解压目录执行：

```bash
sudo bash ./install-macos.sh
```

## 默认节点 TLS 与升级

默认入站包含 VLESS Reality、Hysteria2、TUIC、Trojan、VMess WS、Shadowsocks 2022、Naive、AnyTLS 和 Hysteria。Reality 与 Shadowsocks 不使用普通 TLS 域名证书。

其他默认 TLS 模板未提供域名证书时仍是**初始化专用自签证书**，不等于受信任证书；客户端必须支持导出的对应验证设置。生产部署推荐使用自己的域名及完整证书链，保持证书验证开启。UDP 节点还必须放行对应 UDP 端口，更新程序不会自动解决云防火墙或客户端不支持的问题。

可以显式设置以下三个环境变量，让程序初始化或迁移未修改的默认 TLS 模板：

```text
S_UI_DEFAULT_TLS_SERVER_NAME=你的证书域名
S_UI_DEFAULT_TLS_CERTIFICATE_PATH=完整证书链的绝对路径
S_UI_DEFAULT_TLS_KEY_PATH=对应私钥的绝对路径
```

三项必须完整提供；已有安装还须显式设置 `S_UI_DEFAULT_TLS_MIGRATE_BOOTSTRAP=true` 才执行迁移；证书须与私钥匹配、未过期且包含该域名。已有自定义 TLS 配置不会被覆盖，入站 ID、端口和用户保留。迁移完成后需重新更新客户端订阅。不要把自己的私钥提交到 GitHub。

systemd 不会自动继承终端环境变量，须在 `sudo systemctl edit s-ui` 中添加 `[Service]` 下的 `Environment="变量名=值"`，再执行 `sudo systemctl daemon-reload && sudo systemctl restart s-ui`。证书及私钥须位于服务用户可读取、systemd 沙箱可访问的位置（推荐安装目录的 `cert/`）；不要开放 `/root` 权限，也不要使用 `chmod 777`。导入时证书和私钥内容保存在本机数据库中；续期后需通过面板替换证书并保存，单纯重启不会更新已导入内容。迁移成功后可移除这些一次性环境变量。

## 默认访问地址

安装时保留默认配置时：

| 服务 | 地址 |
| --- | --- |
| Web 面板 | 端口 `2095`，默认路径 `/app/`（地址取决于监听设置与防火墙） |
| 订阅服务 | 端口 `2096`，默认路径 `/sub/`（地址取决于监听设置与防火墙） |

## 协议选择建议

如果不确定应该使用哪种协议组合，可以先阅读：

- [s-ui-go 0.1 协议组合建议方案](PROTOCOL_GUIDE.md)

快速建议：

| 场景 | 推荐方案 |
| --- | --- |
| 自建 VPS，无 CDN | `VLESS + Reality + TCP` |
| 有域名，走 CDN | `VLESS/Trojan + TLS + WebSocket` |
| 弱网、移动网络、游戏或 UDP | `Hysteria2/TUIC + TLS` |
| 本地软件代理 | `Mixed` |
| Windows/macOS 全局代理 | `Tun + DNS 接管` |
| 软路由或网关 | `TProxy + DNS 劫持` |





模式概览：

| 模式 | 适用场景 | 说明 |
| --- | --- | --- |
| 推荐 | 通用部署场景 | 使用加密远程 DNS、DNS 劫持和本地域名规则，兼顾兼容性。 |
| 严格 | 对 DNS 出口一致性要求较高的场景 | 在推荐模式基础上强制默认域名解析器使用 `remote-dns`。 |

## SNI 域名参考

以下域名仅作为 TLS / Reality 场景下的 SNI 候选参考。实际配置应优先使用与证书、目标站点或部署方案一致且可稳定访问的 HTTPS 域名，并在客户端连接前完成可用性验证。

```text
aws.com
bing.com
snap.licdn.com
devblogs.microsoft.com
cdn.bizibly.com
www.apple.com
ts1.tc.mm.bing.net
fpinit.itunes.apple.com
go.microsoft.com
catalog.gamepass.com
gray-config-prod.api.arc-cdn.net
apps.mzstatic.com
tag.demandbase.com
r.bing.com
tag-logger.demandbase.com
cdn-dynmedia-1.microsoft.com
services.digitaleast.mobi
gray.video-player.arcpublishing.com
azure.microsoft.com
beacon.gtv-pub.com
amd.com
www.microsoft.com
www.cloudflare.com
www.amazon.com
www.google.com
www.youtube.com
www.github.com
www.linkedin.com
www.office.com
www.live.com
www.netflix.com
www.paypal.com
www.spotify.com
www.adobe.com
www.oracle.com
www.ibm.com
www.intel.com
www.nvidia.com
www.dropbox.com
www.slack.com
www.zoom.us
cdn.jsdelivr.net
cdnjs.cloudflare.com
ajax.googleapis.com
fonts.googleapis.com
storage.googleapis.com
api.github.com
assets.msn.com
login.microsoftonline.com
```

## 从源码构建

### Windows PowerShell 推荐命令

在仓库根目录运行：

```powershell
.\build-windows.ps1 -System windows -Architecture amd64 -Package -NonInteractive
# Linux/macOS 包可以在 Windows 上使用纯 Go SQLite 驱动关闭 CGO 交叉构建。
# 以下命令不使用 -SkipFrontend，因此可从干净检出构建 frontend/dist。
.\build-windows.ps1 -System linux -Architecture amd64 -NoCGO -Package -NonInteractive
.\build-windows.ps1 -System darwin -Architecture amd64 -NoCGO -Package -NonInteractive
```

生成文件：

| 目标系统 | 输出路径 |
| --- | --- |
| Windows amd64 | `dist/s-ui-windows-amd64.zip` |
| Linux amd64 | `dist/s-ui-linux-amd64.zip` |
| macOS amd64 | `dist/s-ui-macos-amd64.zip` |

### Linux XHTTP / Reality 内置核心与兼容性

`VLESS + Reality + TCP Vision/XHTTP` 所需的 Xray-core Go 源码已编译进 `sui`。当前上游源码版本为 **Xray-core `v26.3.27`**，对应 Go 模块 **`github.com/xtls/xray-core v1.260327.0`**（由 `go.mod`/`go.sum` 固定）。Linux 发布 ZIP 不包含、也不会启动独立 `xray` 可执行文件；运行时不需要 `SUI_XRAY_BINARY`。Xray-core 使用 MPL-2.0，详见 [XRAY-CORE-NOTICE.md](XRAY-CORE-NOTICE.md)。

客户端兼容性是**能力边界**，不是“每个客户端、每个版本都已真实实测”的承诺：

| 客户端/核心 | Reality TCP Vision | XHTTP | 使用建议 |
| --- | --- | --- | --- |
| v2rayNG / v2rayN（选择 Xray-core） | 支持所需 Xray 扩展 | 支持取决于客户端与所选 Xray-core 版本 | 导入标准 VLESS URI；确认应用实际选中了 Xray-core。 |
| Mihomo **≥ 1.19.22** | 支持 Reality TCP Vision 配置/订阅输出 | 支持 XHTTP 配置/订阅输出 | 使用当前 Mihomo/Clash Meta，并在目标设备上自行连通性验证。 |
| 旧 Clash、旧 Clash Meta | 不保证 | 不保证 | 不要把它们当作此功能的兼容目标。 |
| v2fly / V2Ray-core | 不支持这些 Xray 专有扩展 | 不支持 | 改用 Xray-core。 |

XHTTP 的客户端覆盖范围比 Reality TCP Vision 更窄；无法确认 `type=xhttp` 时，优先使用 TCP Vision 的标准 URI。

> **非 443 Reality 风险：** 默认 Xray 入站 `10443` 和 `2088` 不是 443。Xray 官方 Reality 配置说明提示，非 443 端口可能使 Reality 目标站对来源产生封锁风险。可评估将 Reality 服务与目标访问路径使用不同 IP、做端口映射，或明确接受该风险；不能接受时不要启用这些默认非 443 入站。请先在自己的网络与目标条件下测试。

官方参考：

- Xray-core 发布与版本：<https://github.com/XTLS/Xray-core/releases/tag/v26.3.27>
- Xray Reality/传输配置与非 443 风险：<https://xtls.github.io/en/config/transport.html>
- Mihomo VLESS 配置：<https://wiki.metacubex.one/en/config/proxies/vless/>
- Mihomo 传输配置：<https://wiki.metacubex.one/en/config/proxies/transport/>
- Mihomo 官方发布记录（确认 `v1.19.22` 或更高）：<https://github.com/MetaCubeX/mihomo/releases>
## 安全提醒

### 反向代理信任边界

仅在面板位于实际受控的反向代理之后时，才设置 `S_UI_TRUSTED_PROXIES`。该环境变量接受以逗号分隔的代理 IP 或 CIDR，例如：

```bash
S_UI_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/24
```

- 只填写真实反向代理的地址范围；反向代理必须安全地覆写或追加 `X-Forwarded-For` 与 `X-Forwarded-Proto`。
- 不要配置 `0.0.0.0/0` 或 `::/0`，也不要在信任宽泛网段时将面板监听端口直接暴露到公网。
- 未配置该变量时，来自客户端的伪造转发头不会影响登录限流、审计 IP 或 Secure Cookie 判定。

生产环境使用前请务必：

- 修改默认管理员账号和密码
- 修改默认端口和访问路径
- 限制面板访问来源或增加反向代理鉴权
- 妥善保管数据库、证书和配置文件
- 定期检查 Release 更新和依赖安全风险

请勿提交以下本机数据：

- `frontend/node_modules/`
- `frontend/dist/`
- `db/`、`logs/`、`cert/`
- 本地二进制、临时包和备份文件
- `.env`、`.vscode/`、`.claude/` 等本地配置

## English Summary

**User-facing version:** **s-ui-go 0.1**. Current update packages are pinned to commit `3768f56` (sing-box `v1.14.3`, Xray-core `v26.3.27`); the older `s-ui-go-v0.1.1` Release assets have not been replaced.

For Ubuntu/systemd deployments, download the immutable Linux release pair `s-ui-linux-amd64.zip` and `s-ui-linux-amd64.zip.sha256`, verify the ZIP, extract it, then use the packaged `sui` or `install-linux.sh`. Do not bootstrap from a mutable branch. The default install directory is `/usr/local/s-ui`.

The program embeds Xray-core `v26.3.27` (Go module `v1.260327.0`) into `sui`; no standalone `xray` executable is shipped. Client support is limited to Xray-core based v2rayNG/v2rayN and Mihomo >= 1.19.22 where applicable; XHTTP support is narrower than TCP Vision. See the Chinese sections and [XRAY-CORE-NOTICE.md](XRAY-CORE-NOTICE.md) for details.
## 致谢与许可

本项目基于开源 S-UI / s-ui-go 项目整理和构建，感谢原项目作者、历任维护者、社区贡献者以及所有提交问题反馈、测试结果和改进建议的用户。

同时感谢以下开源项目和生态提供的重要能力与基础设施：

- sing-box、Xray-core 等核心网络组件及其贡献者
- Go、Vue、Vite、TypeScript 等语言、框架和构建工具生态
- 各类协议、订阅转换、TLS / ECH / DNS 相关开源实现和文档

本仓库作为社区整理版本，保留并遵循原项目的开源许可要求。仓库根目录提供了 [GPL-3.0 License](LICENSE) 文本；如果你修改、再发布或基于本项目继续分发，请务必同时遵守 GPL-3.0、上游项目以及相关第三方依赖各自的许可证条款。

请注意：本项目不是官方原版发布。使用者应自行确认代码来源、依赖许可证、二进制分发方式和生产环境风险；本项目按开源许可证约定不提供任何形式的担保。
