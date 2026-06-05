# s-ui-go 社区版

> 基于开源 S-UI / s-ui-go 项目整理的社区构建版本，提供 Windows、Linux、macOS 发布包和常用安装脚本。

[![Release](https://img.shields.io/badge/release-v1.4.2.3-blue)](https://github.com/yongwei9527-art/s-ui-go/releases/tag/v1.4.2.3)
[![License](https://img.shields.io/badge/license-GPL--3.0-green)](LICENSE)

## 目录

- [项目说明](#项目说明)
- [相比原始版本的改进点](#相比原始版本的改进点)
- [下载](#下载)
- [快速安装](#快速安装)
- [默认访问地址](#默认访问地址)
- [协议选择建议](#协议选择建议)
- [SNI 伪装域名参考](#sni-伪装域名参考)
- [从源码构建](#从源码构建)
- [验证与运行](#验证与运行)
- [脚本说明](#脚本说明)
- [安全提醒](#安全提醒)
- [English Summary](#english-summary)
- [致谢与许可](#致谢与许可)

## 项目说明

本仓库面向 `yongwei9527-art/s-ui-go` 发布，主要整理和增强以下内容：

- Windows / Linux / macOS 三端 amd64 发布压缩包
- 中文 / English Web 面板体验
- Windows、Linux、macOS 安装、卸载和服务管理脚本
- DNS 防泄漏基础检查能力
- TLS / ECH / WebSocket / TUIC / Hysteria2 订阅转换兼容性

> 说明：本项目是社区整理版本，非官方原版。生产环境使用前请自行评估风险。

## 相比原始版本的改进点

### 改进亮点速览

为了让新用户快速看懂本版本相对原始版本做了哪些增强，这里先给出简要总结：

- **更容易下载使用**：提供 Windows、Linux、macOS 三端 amd64 发布包。
- **更容易安装维护**：补齐安装、卸载、服务管理和 Windows 构建脚本。
- **更适合新手部署**：README 增加下载、安装、默认地址、构建、验证和安全提醒。
- **安装后显示更直观**：Linux 安装脚本和管理菜单默认中文显示，并在安装完成后自动读取服务器 IP 生成完整访问地址。
- **更方便选择协议**：新增协议组合指南，按 VPS、CDN、弱网、全局代理、软路由等场景给出建议。
- **配置细节更稳妥**：修正 ACME / TLS 默认 `server_name` 取值，并明确订阅兼容和 DNS 防泄漏方向。

如果以原始 S-UI / s-ui-go 开源版本或初始整理版本作为对比，我的改进重点不是单纯改名字，而是把项目整理成“下载后能安装、安装后能管理、出问题能按文档排查”的社区构建版本。主要改进点如下：

| 改进方向 | 我的改进内容 |
| --- | --- |
| 跨平台发布 | 整理 Windows、Linux、macOS 三端 amd64 发布包，并统一压缩包命名。 |
| 安装与卸载 | 补充和整理 Windows、Linux、macOS 的安装、卸载、服务管理脚本。 |
| 安装后汉化 | 将 Linux 安装流程、常用提示和 `s-ui` 管理菜单改为默认中文显示。 |
| 自动生成访问地址 | 安装完成后自动读取服务器公网 IP，并生成 `http://服务器IP:端口/路径/` 形式的完整面板地址和订阅地址。 |
| Windows 构建 | 完善 Windows PowerShell 构建脚本，支持指定系统/架构、打包、跳过前端构建、非交互构建和清理候选查看。 |
| README 文档 | 重新梳理 README 结构，加入目录、下载链接、快速安装、默认访问地址、源码构建、验证运行、脚本说明、安全提醒、许可说明等内容。 |
| Linux 安装说明 | 单独补充 Debian / Ubuntu、CentOS / Rocky / AlmaLinux / Oracle Linux 的安装步骤和防火墙端口提示。 |
| 协议选择指南 | 新增 [协议组合建议方案](PROTOCOL_GUIDE.md)，按自建 VPS、CDN、弱网、全局代理、软路由等场景给出推荐组合。 |
| ACME / TLS 配置 | 修正 ACME 默认 `server_name` 取值逻辑，改为优先使用第一个域名。 |
| 订阅与 DNS 说明 | 在项目说明和文档中明确 TLS / ECH / WebSocket / TUIC / Hysteria2 订阅兼容性，以及 DNS 防泄漏检查方向。 |
| 发布与合规提示 | 补充 Release 版本、发布说明、致谢、GPL-3.0 许可和“非官方社区版”风险提醒。 |

简单来说，本版本的核心改进可以概括为：**跨平台打包更完整、安装维护更方便、协议配置文档更清晰、TLS/ACME 细节更稳妥、对新手更友好**。

## 下载

最新版本：**v1.4.2.3**

Release 页面：<https://github.com/yongwei9527-art/s-ui-go/releases/tag/v1.4.2.3>

| 系统 | 架构 | 下载文件 |
| --- | --- | --- |
| Windows | amd64 | [`s-ui-windows-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/v1.4.2.3/s-ui-windows-amd64.zip) |
| Linux | amd64 | [`s-ui-linux-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/v1.4.2.3/s-ui-linux-amd64.zip) |
| macOS | amd64 | [`s-ui-macos-amd64.zip`](https://github.com/yongwei9527-art/s-ui-go/releases/download/v1.4.2.3/s-ui-macos-amd64.zip) |

## 快速安装

### Windows

1. 下载 `s-ui-windows-amd64.zip`。
2. 解压到目标目录。
3. 右键 `install-windows.bat`，选择 **以管理员身份运行**。
4. 按提示设置面板端口、面板路径、订阅端口、订阅路径、管理员账号和密码。
5. 安装完成后，可使用 `s-ui-windows.bat` 管理服务。

### Linux

以下只列出常用服务器系统的详细安装步骤。安装时建议使用 `root` 用户，或确保当前用户具备 `sudo` 权限。

#### Debian / Ubuntu

1. 更新软件源并安装基础依赖：

```bash
sudo apt update -y
sudo apt install -y curl wget tar unzip
```

2. 下载安装脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/install.sh -o install.sh
```

3. 执行安装脚本：

```bash
sudo bash install.sh
```

4. 按脚本提示设置以下内容：

- Web 面板端口
- Web 面板访问路径
- 订阅服务端口
- 订阅访问路径
- 管理员账号和密码

5. 安装完成后，脚本会自动读取服务器 IP，并输出可直接访问的完整面板地址，例如：

```text
公网完整地址：
http://服务器公网IP:2095/app/
```

如果安装完成后想再次查看地址，可以在服务器上执行：

```bash
s-ui
```

然后选择 **10. 查看面板设置和访问地址**。

也可以手动读取服务器公网 IP 并拼接默认访问地址：

```bash
SERVER_IP="$(curl -fsSL https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"
echo "http://${SERVER_IP}:2095/app/"
```

如果安装时保持默认端口和路径，则完整地址格式为：

```text
http://服务器公网IP:2095/app/
```

#### CentOS / Rocky / AlmaLinux / Oracle Linux

1. 安装基础依赖：

```bash
sudo yum install -y curl wget tar unzip
```

2. 如果系统启用了防火墙，放行面板和订阅端口。以下以默认端口 `2095`、`2096` 为例：

```bash
sudo firewall-cmd --permanent --add-port=2095/tcp
sudo firewall-cmd --permanent --add-port=2096/tcp
sudo firewall-cmd --reload
```

如果安装时自定义了端口，请把上面的 `2095`、`2096` 换成你的实际端口。

3. 下载安装脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/latest-project-upload/install.sh -o install.sh
```

4. 执行安装脚本：

```bash
sudo bash install.sh
```

5. 按脚本提示设置面板端口、访问路径、订阅端口、订阅路径、管理员账号和密码。

6. 安装完成后，脚本会自动读取服务器 IP，并输出可直接访问的完整面板地址，例如：

```text
公网完整地址：
http://服务器公网IP:2095/app/
```

如果安装完成后想再次查看地址，可以在服务器上执行：

```bash
s-ui
```

然后选择 **10. 查看面板设置和访问地址**。

也可以手动读取服务器公网 IP 并拼接默认访问地址：

```bash
SERVER_IP="$(curl -fsSL https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"
echo "http://${SERVER_IP}:2095/app/"
```

如果安装时保持默认端口和路径，则完整地址格式为：

```text
http://服务器公网IP:2095/app/
```

### macOS

1. 下载 `s-ui-macos-amd64.zip`。
2. 解压后进入目录。
3. 执行安装脚本：

```bash
chmod +x install-macos.sh uninstall-macos.sh
sudo ./install-macos.sh
```

## 默认访问地址

如果安装时没有修改默认配置：

| 服务 | 地址 |
| --- | --- |
| Web 面板 | `http://localhost:2095/app/` |
| 订阅服务 | `http://localhost:2096/sub/` |

## 协议选择建议

如果不确定应该使用哪种协议组合，可以先阅读：

- [S-UI 协议组合建议方案](PROTOCOL_GUIDE.md)

快速建议：

| 场景 | 推荐方案 |
| --- | --- |
| 自建 VPS，无 CDN | `VLESS + Reality + TCP` |
| 有域名，走 CDN | `VLESS/Trojan + TLS + WebSocket` |
| 弱网、移动网络、游戏或 UDP | `Hysteria2/TUIC + TLS` |
| 本地软件代理 | `Mixed` |
| Windows/macOS 全局代理 | `Tun + DNS 接管` |
| 软路由或网关 | `TProxy + DNS 劫持` |


## SNI 伪装域名参考

如果发现节点不能使用，或延迟显示 `-1`，可以随机挑选下方域名用于 SNI，直到节点正常使用为止；也可以参考视频教程中同一模式的域名填写，确保可用。

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
.\build-windows.ps1 -System linux -Architecture amd64 -NoCGO -SkipFrontend -Package -NonInteractive
.\build-windows.ps1 -System darwin -Architecture amd64 -NoCGO -SkipFrontend -Package -NonInteractive
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
| `-SkipFrontend` | 跳过前端构建，需要已有 `web/html/` |
| `-Package` | 生成 `dist/*.zip` 发布压缩包 |
| `-NonInteractive` | 构建完成后不等待输入 |
| `-ListCleanCandidates` | 只列出可清理候选，不删除文件 |

### Linux / macOS 手动构建

```bash
./build.sh
```

`build.sh` 会构建前端、复制到 `web/html/`，并生成后端二进制 `sui`。

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
- `web/html/`
- `db/`、`logs/`、`cert/`
- 本地二进制、临时包和备份文件
- `.env`、`.vscode/`、`.claude/` 等本地配置

## English Summary

This repository is a community build of the open-source S-UI / s-ui-go project.

It provides:

- Release packages for Windows, Linux, and macOS amd64
- Web panel experience in Chinese and English
- Install, uninstall, and service management scripts
- Basic DNS leak guard checks
- Subscription conversion compatibility for TLS, ECH, WebSocket, TUIC, and Hysteria2

Download the latest release from:

<https://github.com/yongwei9527-art/s-ui-go/releases/tag/v1.4.2.3>

## 致谢与许可

本项目基于开源 S-UI / s-ui-go 项目整理和构建，感谢原项目作者、历任维护者、社区贡献者以及所有提交问题反馈、测试结果和改进建议的用户。

同时感谢以下开源项目和生态提供的重要能力与基础设施：

- sing-box、Xray-core 等核心网络组件及其贡献者
- Go、Vue、Vite、TypeScript 等语言、框架和构建工具生态
- 各类协议、订阅转换、TLS / ECH / DNS 相关开源实现和文档

本仓库作为社区整理版本，保留并遵循原项目的开源许可要求。仓库根目录提供了 [GPL-3.0 License](LICENSE) 文本；如果你修改、再发布或基于本项目继续分发，请务必同时遵守 GPL-3.0、上游项目以及相关第三方依赖各自的许可证条款。

请注意：本项目不是官方原版发布。使用者应自行确认代码来源、依赖许可证、二进制分发方式和生产环境风险；本项目按开源许可证约定不提供任何形式的担保。
