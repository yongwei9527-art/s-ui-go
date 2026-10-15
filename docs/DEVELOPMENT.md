# 源码构建与客户端兼容性

[返回项目首页](../README.md)

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

`VLESS + Reality + TCP Vision/XHTTP` 所需的 Xray-core Go 源码已编译进 `sui`。当前上游源码版本为 **Xray-core `v26.3.27`**，对应 Go 模块 **`github.com/xtls/xray-core v1.260327.0`**（由 `go.mod`/`go.sum` 固定）。Linux 发布 ZIP 不包含、也不会启动独立 `xray` 可执行文件；运行时不需要 `SUI_XRAY_BINARY`。Xray-core 使用 MPL-2.0，详见 [XRAY-CORE-NOTICE.md](../XRAY-CORE-NOTICE.md)。

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
