# s-ui-go Community Build Release Notes

## v1.0.0

### 中文

#### 发布包

本版本提供以下 amd64 压缩包：

- `s-ui-windows-amd64.zip`
- `s-ui-linux-amd64.zip`
- `s-ui-macos-amd64.zip`

#### 主要更新

- Web 面板语言选择整理为 `中文` / `English` 两个主要选项。
- 默认语言根据浏览器语言自动选择。
- Linux 一键安装脚本的 Release 下载源切换到当前仓库：`yongwei9527-art/s-ui-go`。
- 构建脚本增加 `-NonInteractive` 参数，适合自动化生成发布包。
- macOS 发布包统一命名为 `s-ui-macos-*.zip`，便于用户识别。
- 发布包内包含 `RELEASE_NOTES.md`，方便离线查看说明。

#### DNS 防泄漏

- 增加 DNS 防泄漏基础逻辑与校验能力。
- 增加 DNS hijack / resolver 相关检查，降低订阅配置中的 DNS 泄漏风险。
- 增加 DNS 防泄漏相关单元测试。

#### 订阅转换修复

- 修复 TLS / ECH / WebSocket / TUIC / Hysteria2 订阅转换缺口。
- 增加安全访问器，降低缺失字段、空值和 IPv6 host 解析导致的异常风险。
- HY2 支持 `mport` / `hop_interval` 转换。
- TUIC 订阅转换会省略空可选字段，并支持常见 TLS alias。
- ECH 支持多 config 和相关布尔选项。
- WebSocket early-data 缺失 header 时会生成 warning。
- 增加 focused unit tests 覆盖订阅转换和 warning 行为。

#### 安装提示

- Windows：下载 `s-ui-windows-amd64.zip`，解压后以管理员身份运行 `install-windows.bat`。
- Linux：下载 `s-ui-linux-amd64.zip`，或使用仓库根目录 `install.sh` 一键安装。
- macOS：下载 `s-ui-macos-amd64.zip`，解压后按包内 `install-macos.sh` 和 `com.s-ui.plist` 安装。

### English

#### Release assets

This release provides the following amd64 packages:

- `s-ui-windows-amd64.zip`
- `s-ui-linux-amd64.zip`
- `s-ui-macos-amd64.zip`

#### Highlights

- Web panel language options are simplified to `中文` / `English`.
- Default language follows the browser language.
- Linux one-click installer now downloads release assets from `yongwei9527-art/s-ui-go`.
- Build script now supports `-NonInteractive` for automated packaging.
- macOS packages are named as `s-ui-macos-*.zip` for clearer platform recognition.
- Release packages include `RELEASE_NOTES.md` for offline reference.

#### DNS leak guard

- Added DNS leak guard groundwork and validation support.
- Added DNS hijack / resolver checks to reduce DNS leak risks in generated subscription configs.
- Added DNS leak guard unit tests.

#### Subscription conversion fixes

- Fixed subscription conversion gaps for TLS, ECH, WebSocket, TUIC, and Hysteria2.
- Added safe accessors to reduce missing-field, empty-value, and IPv6 host parsing risks.
- HY2 now supports `mport` / `hop_interval` conversion.
- TUIC conversion omits empty optional fields and supports common TLS aliases.
- ECH supports multiple config values and related boolean options.
- WebSocket early-data gaps now produce warnings when the header name is missing.
- Added focused unit tests for conversion and warning behavior.

#### Install notes

- Windows: download `s-ui-windows-amd64.zip`, extract it, and run `install-windows.bat` as Administrator.
- Linux: download `s-ui-linux-amd64.zip`, or use the root `install.sh` one-click installer.
- macOS: download `s-ui-macos-amd64.zip`, extract it, and follow `install-macos.sh` / `com.s-ui.plist`.

## Credits / 致谢

本项目基于 S-UI / s-ui-go 的开源成果继续整理和改进。感谢原作者与所有上游贡献者提供的核心功能、前后端架构、协议能力和社区基础。

This project continues from the open-source S-UI / s-ui-go work. Thanks to the original author and upstream contributors for the core features, frontend/backend architecture, protocol support, and community foundation.
