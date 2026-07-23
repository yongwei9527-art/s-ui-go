# s-ui-go 0.1

> 基于开源 S-UI / s-ui-go 项目的社区维护构建，提供 Windows、Linux、macOS 发布包、安装脚本、服务管理脚本及相关配置说明。
>
> **版本约定：** 用户可见名称为 **s-ui-go 0.1**；本次发布标签为 [`s-ui-go-v0.1.0`](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0)。文档中的 `sui` 是程序文件名，`s-ui` 是 Linux 服务/管理命令，不代表另一套版本。

[![Release](https://img.shields.io/badge/release-s--ui--go%200.1-blue)](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0)
[![License](https://img.shields.io/badge/license-GPL--3.0-green)](LICENSE)
## 目录

- [项目说明](#项目说明)
- [维护内容与差异](#维护内容与差异)
- [下载](#下载)
- [快速安装](#快速安装)
- [默认访问地址](#默认访问地址)
- [协议选择建议](#协议选择建议)
- [SNI 域名参考](#sni-域名参考)
- [从源码构建](#从源码构建)
- [验证与运行](#验证与运行)
- [脚本说明](#脚本说明)
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

## 下载

当前用户可见版本：**s-ui-go 0.1**
发布标签：**`s-ui-go-v0.1.0`**
Release 页面：<https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0>

| 系统 | 架构 | 发布资产 |
| --- | --- | --- |
| Windows | amd64 | [`s-ui-windows-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.0/s-ui-windows-amd64.zip) |
| Linux | amd64 | [`s-ui-linux-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.0/s-ui-linux-amd64.zip) + [`s-ui-linux-amd64.zip.sha256`](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.0/s-ui-linux-amd64.zip.sha256) |
| macOS | amd64 | [`s-ui-macos-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.0/s-ui-macos-amd64.zip) |

Linux 的发布单元是 **ZIP 与同名 ZIP 的 `.sha256` 文件**。先校验 ZIP，再解压并使用包内的 `sui` 或 `install-linux.sh`；不要把一个不存在的“裸二进制校验文件”当作发布契约。
## 快速安装

### Windows

1. 下载 `s-ui-windows-amd64.zip`。
2. 解压到目标目录。
3. 右键 `install-windows.bat`，选择 **以管理员身份运行**。
4. 按提示设置面板端口、面板路径、订阅端口、订阅路径、管理员账号和密码。
5. 安装完成后，可使用 `s-ui-windows.bat` 管理服务。

### Linux（Ubuntu / systemd）

Linux 的包内安装器要求 **systemd**。推荐 Ubuntu 22.04/24.04 等带 systemd 的 VPS，`uname -m` 应为 `x86_64` 才能使用本节的 amd64 资产。安装后的默认目录是 `/usr/local/s-ui`。

1. 安装校验和运行依赖：

   ```bash
   sudo apt update
   sudo apt install -y ca-certificates curl unzip openssl iproute2 coreutils
   systemctl --version
   ```

2. **只从固定 Release 资产建立信任链**。下载 ZIP 及其同名校验文件、校验 ZIP、测试压缩包、再解压：

   ```bash
   TAG='s-ui-go-v0.1.0'
   ASSET='s-ui-linux-amd64.zip'
   BASE="https://github.com/yongwei9527-art/s-ui-go/releases/download/${TAG}"

   mkdir -p ~/s-ui-go-install && cd ~/s-ui-go-install
   curl -fL -O "${BASE}/${ASSET}"
   curl -fL -O "${BASE}/${ASSET}.sha256"
   sha256sum --check "${ASSET}.sha256"
   unzip -t "${ASSET}"
   rm -rf package && unzip -q "${ASSET}" -d package
   cd package
   ```

3. 已校验的 ZIP 内含 `sui` 与包内安装器；通常使用安装器：

   ```bash
   sudo bash ./install-linux.sh
   ```

   这是“校验 ZIP → 解压 → 使用包内 `sui` / `install-linux.sh`”的完整流程。不要使用 `curl | bash`，也不要从任何可变分支下载 bootstrap；一键安装的可审计信任边界只能是固定 Release 的 ZIP 与其同名 `.sha256`。

4. 全新安装会严格检查 Reality 目标、所需端口和双核心启动状态；通过后启用下列默认入站：

   | 核心 | 默认入站 | 端口 |
   | --- | --- | --- |
   | sing-box | VLESS Reality | `443/tcp` |
   | 内置 Xray-core | VLESS Reality TCP Vision | `10443/tcp` |
   | 内置 Xray-core | VLESS Reality XHTTP | `2088/tcp` |

   若严格检查、安装或启动失败，安装器会停止并回滚本次变更；已有数据库升级会保留既有面板、管理员和协议配置，并使用安装前备份恢复失败状态。只有明确接受 Xray 默认模板保持关闭的高级场景才应使用 `--allow-manual-xray`。

5. 安装后核对本机服务：

   ```bash
   sudo systemctl status s-ui --no-pager
   sudo ss -ltnp | grep -E ':(443|10443|2088)\b'
   sudo /usr/local/s-ui/sui -v
   ```

   安装器**不会承诺自动开放**管理面板/订阅端口 `2095/2096`；推荐将它们限制为本机并经 HTTPS 反代或 SSH 转发访问。无论本机 UFW 状态如何，云厂商安全组/云防火墙都需要你自行确认 `443/tcp`、`10443/tcp`、`2088/tcp` 是否按实际需求放行。完整的 Ubuntu 部署与恢复说明见 [Ubuntu 部署指南](UBUNTU_DEPLOYMENT.md)。
### macOS

1. 下载 `s-ui-macos-amd64.zip`。
2. 解压后进入目录。
3. 执行安装脚本：

```bash
chmod +x install-macos.sh uninstall-macos.sh
sudo ./install-macos.sh
```

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

常用参数：

| 参数 | 说明 |
| --- | --- |
| `-System windows\|linux\|darwin` | 目标系统 |
| `-Architecture amd64\|arm64\|386\|arm` | 目标架构 |
| `-NoCGO` | 关闭 CGO，适合从 Windows 交叉构建 Linux/macOS 包 |
| `-SkipFrontend` | 跳过前端构建，需要已有 `frontend/dist/` |
| `-Package` | 生成 `dist/*.zip` 发布压缩包 |
| Linux amd64 `-Package` | Xray-core Go 源码依赖已编译进 `sui`，压缩包不附带独立 `xray` 文件 |
| `-NonInteractive` | 构建完成后不等待输入 |
| `-ListCleanCandidates` | 只列出可清理候选，不删除文件 |

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
## 验证与运行

后端测试：

```bash
go test ./...
```

前端检查和构建：

```bash
cd frontend
npm install
npm run lint
npm run build
```

本地运行时，建议指定独立数据库目录：

```bash
SUI_DB_FOLDER=db SUI_DEBUG=true ./sui
```

Windows PowerShell 示例：

```powershell
$env:SUI_DB_FOLDER="db"
$env:SUI_DEBUG="true"
.\sui.exe
```

基础检查项：

- Web 面板可以打开
- 可以登录后台
- 可以创建入站 / 节点配置
- 可以生成订阅链接
- 重启服务后配置仍然生效

## 脚本说明

根目录脚本是主要维护版本：

| 文件 | 用途 |
| --- | --- |
| `build-windows.ps1` | Windows PowerShell 多系统构建和打包脚本 |
| `build-windows.bat` | Windows CMD 构建入口 |
| `install-windows.bat` | Windows 安装脚本 |
| `uninstall-windows.bat` | Windows 卸载脚本 |
| `s-ui-windows.bat` | Windows 服务管理脚本 |
| `install.sh` | Linux 通用安装入口 |
| `install-linux.sh` | Linux 包内安装脚本 |
| `uninstall-linux.sh` | Linux 卸载脚本 |
| `s-ui.sh` | Linux 服务管理脚本 |
| `s-ui.service` | Linux systemd 服务文件 |
| `install-macos.sh` | macOS 安装脚本 |
| `uninstall-macos.sh` | macOS 卸载脚本 |
| `com.s-ui.plist` | macOS launchd 配置 |

`windows/` 目录中的同名文件仅用于旧路径兼容或参考，优先使用根目录脚本。

## 安全提醒

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

**User-facing version:** **s-ui-go 0.1**. **Release tag:** [`s-ui-go-v0.1.0`](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0).

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
