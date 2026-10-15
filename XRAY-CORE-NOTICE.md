# s-ui-go 0.1：内置 Xray-core 源码依赖声明

> **用户可见版本：** s-ui-go 0.1
> **发布标签：** `s-ui-go-v0.1.4`

s-ui-go 将来自 [XTLS/Xray-core](https://github.com/XTLS/Xray-core) 的所需 Go 源码包直接编译进 `sui`，用于提供 VLESS Reality TCP Vision 与 XHTTP 能力。

- 当前上游源码版本：**Xray-core `v26.3.27`**。
- 当前 Go 模块版本：**`github.com/xtls/xray-core v1.260327.0`**；精确依赖由 `go.mod` 和 `go.sum` 记录。
- Linux Release ZIP 中没有、运行时也不会启动独立 `xray` 可执行文件；不需要 `SUI_XRAY_BINARY`。
- 上游项目、源码、发布与安全公告：<https://github.com/XTLS/Xray-core>
- 上游许可：Mozilla Public License 2.0（[MPL-2.0](https://github.com/XTLS/Xray-core/blob/main/LICENSE)）。

Xray-core 是独立维护的上游项目，其许可证、NOTICE、发布节奏和安全建议仍然适用。升级时必须选择并审核明确的上游版本，显式更新 Go 依赖、运行完整测试并重建所有发布目标；不要让发布构建依赖浮动的 `latest` 下载。

本声明不改变 Xray-core 的 MPL-2.0 许可义务。有关 Reality 配置及非 443 端口风险，参见 Xray 官方文档：<https://xtls.github.io/en/config/transport.html>。