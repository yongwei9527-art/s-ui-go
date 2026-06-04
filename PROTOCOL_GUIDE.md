# S-UI 协议组合建议方案

本文面向新手，说明在 S-UI 面板中常见代理协议如何组合，什么时候用，TLS / Reality / ECH / Transport / Mux / DNS 应该怎么配，以及常见错误。

> 简单结论：新手自建 VPS 优先选 `VLESS + Reality + TCP`；有 CDN 选 `VLESS/Trojan + TLS + WebSocket`；弱网和 UDP 选 `Hysteria2/TUIC`；本地代理入口选 `Mixed`；桌面全局代理选 `Tun`；软路由/网关透明代理选 `TProxy`。

## 1. 面板中的组合模型

S-UI 面板里的一个节点通常由下面几部分组成：

```text
入站协议
  + 监听地址 / 端口
  + 用户 / 密码 / UUID
  + TLS / Reality / ECH
  + Transport 传输层
  + Multiplex 多路复用
  + OutJson 客户端侧参数
  + Addr 多域名/多入口覆盖
  + DNS / 路由规则
```

保存后，后端会把面板配置转换成 sing-box JSON、订阅 JSON、Clash 配置或分享链接。

相关文件：

- 入站类型：[frontend/src/types/inbounds.ts](frontend/src/types/inbounds.ts)
- 传输类型：[frontend/src/types/transport.ts](frontend/src/types/transport.ts)
- 入站编辑弹窗：[frontend/src/layouts/modals/Inbound.vue](frontend/src/layouts/modals/Inbound.vue)
- 客户端侧参数：[frontend/src/components/OutJson.vue](frontend/src/components/OutJson.vue)
- 出站转换：[util/outJson.go](util/outJson.go)
- 订阅 JSON：[sub/jsonService.go](sub/jsonService.go)
- 分享链接生成：[util/genLink.go](util/genLink.go)
- 分享链接解析：[util/linkToJson.go](util/linkToJson.go)

## 2. 协议选择速查表

| 使用场景 | 推荐组合 | 难度 | 重点 |
| --- | --- | --- | --- |
| 新手自建 VPS，无 CDN | VLESS + Reality + TCP | 中 | 不需要证书，不适合 CDN |
| 有域名，想走 CDN | VLESS + TLS + WebSocket | 中 | Path / Host / SNI 必须一致 |
| 想简单稳定 | Trojan + TLS + TCP | 低 | 需要域名和证书 |
| 老客户端兼容 | VMess + TLS + WebSocket | 中 | 新建节点不优先推荐 |
| 弱网 / 高丢包 / 移动网络 | Hysteria2 + TLS | 中 | 必须开放 UDP |
| 游戏 / 低延迟 / UDP | TUIC + TLS | 中 | UUID、密码、UDP 端口要对 |
| 轻量代理 | Shadowsocks + AEAD/2022 | 低 | 伪装能力弱 |
| 高 HTTPS 伪装 | Naive + TLS | 中高 | 常配合 Caddy |
| 新 TLS 类方案 | AnyTLS + TLS | 中 | 需要新内核支持 |
| 本地软件代理 | Mixed | 低 | HTTP + SOCKS 混合入口 |
| Windows/macOS 全局代理 | Tun + DNS 接管 | 中 | 需要管理员权限，避免 DNS 泄漏 |
| 软路由 / 网关 | TProxy + DNS 劫持 | 高 | 需要 iptables/nftables 和策略路由 |
| 简单 TCP 透明代理 | Redirect | 中 | 主要处理 TCP，UDP 能力有限 |

## 3. 新手最推荐方案

### 3.1 自建 VPS，无 CDN：VLESS + Reality + TCP

适合：

- 个人 VPS
- 不想申请证书
- 不想配置 CDN
- 想要现代、稳定、性能好的方案

推荐配置：

```text
协议：VLESS
传输：TCP
安全：Reality
Flow：xtls-rprx-vision，按客户端支持情况选择
uTLS：Chrome 或 Firefox 指纹
SNI：选择稳定 HTTPS 网站域名
Short ID：服务端和客户端一致
Public Key / Private Key：由面板生成后正确填写
```

常见错误：

- Reality 的 SNI 和目标站不匹配。
- Public Key / Short ID 填错。
- 客户端没有启用 uTLS。
- Flow 服务端和客户端不一致。
- 把 Reality 放到 CDN 后面使用，通常不合适。

DNS 建议：

- 开启 DNS 防泄漏。
- 代理域名通过远程 DNS 或代理解析。
- 不要让系统 DNS 直接解析需要代理的域名。

### 3.2 有域名和 CDN：VLESS/Trojan + TLS + WebSocket

适合：

- Cloudflare 等 CDN 中转
- 想伪装成普通 HTTPS 网站
- 有域名和证书

推荐配置：

```text
协议：VLESS 或 Trojan
传输：WebSocket
TLS：开启
Path：例如 /vless 或 /trojan
Host：证书域名
SNI：证书域名
CDN：开启 WebSocket
```

常见错误：

- Path 客户端和服务端不一致。
- Host / SNI / 证书域名不一致。
- CDN 没开启 WebSocket。
- 使用了 CDN 不支持的端口。
- 后端和反代重复终止 TLS，配置混乱。

### 3.3 想简单稳定：Trojan + TLS + TCP

适合：

- 新手
- 有域名和证书
- 不想研究太多参数

推荐配置：

```text
协议：Trojan
传输：TCP
TLS：开启
密码：客户端和服务端一致
SNI：证书域名
端口：通常 443
```

常见错误：

- 忘记开启 TLS。
- 密码不一致。
- 使用 IP 直连但证书是域名证书。
- 证书过期或链不完整。

### 3.4 弱网、UDP、游戏：Hysteria2 或 TUIC + TLS

适合：

- 移动网络
- 跨境高丢包线路
- 游戏、视频、语音
- 需要 UDP 的应用

推荐配置：

```text
协议：Hysteria2 或 TUIC
传输：QUIC/UDP
TLS：开启
UDP 端口：服务器防火墙和云安全组必须放行
带宽参数：按实际带宽设置，不要乱填过高
```

Hysteria2 额外建议：

```text
如启用 mport/server_ports，建议同时设置 hop_interval。
如启用 salamander obfs，客户端和服务端密码必须一致。
```

TUIC 额外建议：

```text
uuid 和 password 必须同时存在。
congestion_control 可先用 cubic。
udp_relay_mode 不懂就保持默认或留空。
```

常见错误：

- 只开放 TCP，没开放 UDP。
- 云安全组漏放 UDP。
- 网络环境屏蔽 UDP。
- TLS 证书或 SNI 错误。
- 带宽参数过大导致体验变差。

## 4. 各协议详细建议

### VLESS

推荐组合：

```text
VLESS + Reality + TCP
VLESS + TLS + WebSocket
VLESS + TLS + gRPC
VLESS + TLS + HTTPUpgrade
```

注意：

- 配置 `flow` 时必须有 TLS/Reality。
- WebSocket 要设置 Host/header。
- gRPC 要设置 service_name。
- Reality 不建议配 CDN。

### VMess

推荐组合：

```text
VMess + TLS + WebSocket
VMess + TLS + gRPC
VMess + HTTP transport
```

注意：

- `alter_id` 通常为 0。
- 新建节点不优先推荐 VMess，更多用于兼容旧客户端。
- UUID、Path、Host、SNI 必须一致。

### Trojan

推荐组合：

```text
Trojan + TLS + TCP
Trojan + TLS + WebSocket
Trojan + TLS + gRPC
Trojan + TLS + HTTPUpgrade
```

注意：

- Trojan 通常应该配 TLS。
- 密码必须一致。
- SNI 必须匹配证书域名。

### Shadowsocks

推荐加密：

```text
2022-blake3-aes-128-gcm
2022-blake3-aes-256-gcm
2022-blake3-chacha20-poly1305
aes-128-gcm
aes-256-gcm
chacha20-ietf-poly1305
```

注意：

- 不建议使用老旧非 AEAD 算法。
- 2022 方法需要正确的服务端 password 和用户 password。
- Shadowsocks 轻量快速，但伪装能力弱。

### Hysteria2

推荐组合：

```text
Hysteria2 + TLS
Hysteria2 + TLS + salamander obfs
Hysteria2 + TLS + mport + hop_interval
```

注意：

- 必须开放 UDP。
- 必须 TLS。
- `server_ports` 存在时建议同时配置 `hop_interval`。
- 不建议 CDN 中转。

### TUIC

推荐组合：

```text
TUIC + TLS + uuid/password
TUIC + TLS + congestion_control=cubic/bbr/new_reno
```

注意：

- 必须开放 UDP。
- 必须 TLS。
- uuid 和 password 都要有。
- 不要强制 network=tcp。

### Naive

推荐组合：

```text
Naive + TLS
Naive + QUIC，可选
```

注意：

- 需要真实 TLS 证书。
- 常配合 Caddy 或 HTTPS 服务。
- 客户端必须支持 Naive。

### AnyTLS

推荐组合：

```text
AnyTLS + TLS
```

注意：

- 需要较新的 sing-box / 客户端内核。
- SNI、证书、密码必须一致。
- 不要当作普通 Trojan/VLESS 使用。

### HTTP / SOCKS / Mixed

这些主要用于本地入站，不建议直接暴露公网。

推荐：

```text
Mixed 监听 127.0.0.1:7890
```

适合：

- 浏览器
- Telegram
- 开发工具
- 本地软件代理

风险：

- 监听 `0.0.0.0` 且无认证，会被局域网或公网滥用。

### Tun

适合：

- Windows / macOS / Linux 桌面全局代理
- 不支持代理的软件
- 游戏或 UWP 应用

必须注意：

- 需要管理员权限。
- 需要 DNS 接管，否则容易 DNS 泄漏。
- 不要同时运行多个 Tun 类代理。

### TProxy / Redirect

适合：

- Linux 网关
- OpenWrt / 软路由
- 透明代理

TProxy：

```text
适合 TCP + UDP 透明代理，需要策略路由和 iptables/nftables。
```

Redirect：

```text
更适合简单 TCP 透明代理，UDP 能力有限。
```

必须排除：

```text
127.0.0.0/8
局域网网段
服务器 IP
面板管理地址
DNS 服务器地址，按实际配置决定
```

## 5. TLS / Reality / ECH 建议

### TLS

必须确认：

- 证书没过期。
- SNI 和证书域名一致。
- 证书链完整。
- 客户端信任证书。

### Reality

适合：

- VLESS
- 无证书 VPS 自建
- 不走 CDN

必须确认：

- Public Key / Private Key 正确。
- Short ID 一致。
- SNI 和目标站合理。
- uTLS 指纹开启。

### ECH

适合：

- 客户端和服务端都支持 ECH 的新环境。

必须确认：

- `ech.enabled = true` 时必须有 `ech.config`。
- 客户端支持 ECH。
- DNS / TLS 配置与 ECH 配套。

## 6. DNS 建议

推荐开启 DNS 防泄漏功能。

### 推荐模式

```text
recommended：适合大多数用户，自动补齐 DNS 和 hijack。
strict：生产环境更安全，会强制 default_domain_resolver 使用 remote-dns。
off：不建议普通用户关闭。
```

### 基本原则

```text
国内域名：国内 DNS
国外域名：可信远程 DNS / 代理 DNS
代理域名：避免本地运营商 DNS 解析
Tun/TProxy：必须接管 DNS
```

### 常见错误

- 只代理 TCP，不处理 DNS。
- Tun 开了但 DNS 没接管。
- 浏览器开启自己的 DoH 绕过代理。
- 路由器 DHCP 下发了错误 DNS。
- 代理服务器域名被错误地通过代理解析，导致回环。

## 7. 发布前协议检查清单

发布或交付给用户前，建议至少检查：

```text
[ ] VLESS Reality 能生成可用分享链接/订阅 JSON
[ ] VLESS / VMess / Trojan 的 WS/gRPC/HTTPUpgrade 参数能正确转换
[ ] Shadowsocks 2022 密码组合正确
[ ] Hysteria2 的 mport/server_ports 与 hop_interval 正确
[ ] TUIC 不输出空的 udp_relay_mode / congestion_control
[ ] AnyTLS 的 TLS 配置能保留
[ ] ECH enabled 但 config 空时能 warning
[ ] TLS enabled 但 server_name 空时能 warning
[ ] Reality enabled 但 public_key 空时能 warning
[ ] DNS-sensitive outbound 没有 DNS hijack 时能 warning
[ ] Clash 导出没有丢失关键字段
```

## 8. 依赖模块是否要更新

当前建议：**不要在本次发布前大范围升级协议依赖**。

原因：

- 当前依赖围绕 `sing-box v1.13.12`，协议字段和前后端类型已经有耦合。
- 大范围升级可能导致 JSON 字段变化，破坏订阅、Clash 导出、保存配置和前端默认值。
- QUIC/TUIC/Hysteria2/Naive 相关依赖变动风险较高。

需要重点验证：

- `go.mod` 中 `github.com/quic-go/quic-go` 依赖图是 `v0.59.0`，但 replace 固定到 `v0.57.1`。
- 这可能影响 TUIC、Hysteria/Hysteria2、Naive QUIC。
- 不建议盲目删除 replace；建议先跑 smoke tests。

建议策略：

```text
本次发布：不大升级依赖，只修明确 bug。
发布前：重点测试 QUIC 相关协议。
下个版本：单独开分支升级 sing-box / QUIC / uTLS，并逐项回归协议转换。
```
