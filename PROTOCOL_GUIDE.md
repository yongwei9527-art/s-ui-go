# S-UI 协议组合建议方案

本文面向新手，说明在 S-UI 面板中常见代理协议如何组合，什么时候用，TLS / Reality / ECH / Transport / Mux / DNS 应该怎么配，以及常见错误。

> 简单结论：新手自建 VPS 优先选 `VLESS + Reality + TCP`；有 CDN 选 `VLESS/Trojan + TLS + WebSocket`；弱网和 UDP 选 `Hysteria2/TUIC`；本地代理入口选 `Mixed`；桌面全局代理选 `Tun`；软路由/网关透明代理选 `TProxy`。

## 0. 新手实操教程：按场景配置协议组合

这一节按“你现在有什么条件”来选协议，并给出面板中的配置步骤。后面的章节保留为参数解释、速查表和排错清单。

### 0.1 配置前先确认 5 件事

无论选择哪种组合，先确认：

1. **服务器系统**：建议优先使用 Debian / Ubuntu 或 CentOS 系列。
2. **端口是否放行**：云服务器安全组和系统防火墙都要放行对应端口。
3. **是否有域名**：没有域名优先选 Reality；有域名和证书可以选 TLS。
4. **是否走 CDN**：走 CDN 优先 WebSocket / gRPC；Reality 通常不放 CDN 后面。
5. **客户端是否支持**：客户端内核太旧时，Reality、ECH、AnyTLS、TUIC 可能不可用。

### 0.2 推荐选择路线

```text
没有域名 / 不想申请证书
  → VLESS + Reality + TCP

有域名，想走 CDN
  → VLESS + TLS + WebSocket
  → Trojan + TLS + WebSocket

有域名，不走 CDN，想简单稳定
  → Trojan + TLS + TCP

移动网络、弱网、游戏、UDP
  → Hysteria2 + TLS
  → TUIC + TLS

只给本机软件提供 HTTP/SOCKS 代理
  → Mixed

桌面全局代理
  → Tun + DNS 接管

软路由或网关透明代理
  → TProxy + DNS 劫持
```

### 0.2.1 面板通用设置顺序

不管选择哪种协议，都建议按下面顺序配置，避免“协议填对了但端口、证书或客户端参数不一致”：

1. **放行端口**：先在云服务器安全组放行端口，再在系统防火墙放行同一个端口；Hysteria2 / TUIC 必须放行 UDP。
2. **准备安全配置**：需要证书的组合先配置 TLS / ACME；Reality 先生成 `private_key`、`public_key`、`short_id`；ECH 先准备 `ech.config`。
3. **新建入站**：选择协议，填写监听地址、端口、用户 UUID / 密码 / Password。
4. **选择传输层**：TCP、WebSocket、gRPC、HTTPUpgrade、QUIC 等按推荐组合选择，Path / Host / service_name 必须和客户端一致。
5. **绑定安全配置**：TLS / Reality / ECH 与入站绑定后，确认 `server_name` / `SNI` 不为空且和证书或 Reality 目标一致。
6. **保存并重启核心**：保存入站后重启 sing-box / S-UI 服务，再生成订阅或分享链接。
7. **客户端核对**：客户端地址、端口、协议、UUID/密码、SNI、Path、Host、Public Key、Short ID 必须与服务端一致。

### 0.2.2 更新协议速填表

本项目当前协议类型和传输类型来自面板类型定义与 sing-box 依赖组合。新手可以先按下表填写，能跑通后再调整高级参数。

| 目标 | 入站协议 | 安全配置 | 传输层 | 关键字段 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 无域名 VPS | `VLESS` | `Reality` | `TCP` | UUID、`server_name/SNI`、`public_key`、`short_id`、`flow=xtls-rprx-vision` | 不建议放 CDN 后面。 |
| CDN HTTPS | `VLESS` / `Trojan` | `TLS` | `WebSocket` | 域名证书、`SNI`、`Host`、`Path` | Host / SNI / 证书域名尽量一致。 |
| CDN gRPC | `VLESS` / `Trojan` | `TLS` | `gRPC` | 域名证书、`SNI`、`service_name` | 需要 CDN 和客户端都支持 gRPC。 |
| 简单稳定 | `Trojan` | `TLS` | `TCP` | Trojan 密码、证书域名、`SNI` | 配置少，适合有域名用户。 |
| 兼容旧客户端 | `VMess` | `TLS` | `WebSocket` / `gRPC` | UUID、`alter_id=0`、Path/Host 或 service_name | 新建节点不优先推荐。 |
| 弱网/UDP | `Hysteria2` | `TLS` | UDP 内置 | 密码、UDP 端口、SNI、可选 `obfs`、`mport/server_ports` | 只放行 TCP 会不可用。 |
| 低延迟 UDP | `TUIC` | `TLS` | UDP 内置 | UUID、Password、`congestion_control=cubic`、SNI | UUID 和 Password 都要填。 |
| 新 TLS 类方案 | `AnyTLS` | `TLS` | TCP | 密码、证书、SNI、`padding_scheme` | 要求客户端内核较新。 |
| TLS 握手中转 | `ShadowTLS` | 外层握手 | TCP | version、password、handshake server/port | 通常与其它代理链路配合，非新手首选。 |
| 本机代理入口 | `Mixed` | 不需要 | 本地监听 | `127.0.0.1:7890` | 不要无认证暴露公网。 |
| 桌面全局代理 | `Tun` | 按出站决定 | 虚拟网卡 | 管理员权限、DNS 接管、路由排除 | 避免多个 Tun 同时运行。 |
| 软路由透明代理 | `TProxy` | 按出站决定 | 透明代理 | iptables/nftables、策略路由、DNS 劫持 | 需要排除局域网和服务器 IP。 |

### 0.2.3 最容易配错的字段

| 字段 | 应该怎么填 | 常见错误 |
| --- | --- | --- |
| `listen` | 服务端监听地址，新手通常填 `0.0.0.0` 或留默认 | 只监听 `127.0.0.1`，外部客户端连不上。 |
| `listen_port` | 服务端实际端口，必须和防火墙一致 | 面板端口、订阅端口、节点端口混用。 |
| `server_name` / `SNI` | TLS 填证书域名；Reality 填稳定 HTTPS 目标域名 | 客户端 SNI 与服务端/证书不一致。 |
| WebSocket `path` | 以 `/` 开头，客户端完全一致 | 大小写不一致或少写 `/`。 |
| WebSocket `Host` | CDN/证书域名 | Host 填 IP，导致 CDN 或证书校验失败。 |
| gRPC `service_name` | 服务端和客户端完全一致 | 客户端留空或拼写不同。 |
| Reality `public_key` | 客户端填写服务端生成的 public key | 把 private key 填进客户端。 |
| Reality `short_id` | 从服务端 short_id 列表中选择一个 | 服务端和客户端不一致。 |
| ECH `config` | 有 ECH 配置才启用 | 只开 enabled 但 config 为空。 |
| TUIC `congestion_control` | 不确定先用 `cubic` | 填客户端不支持的值。 |
| Hysteria2 / TUIC 端口 | 云安全组和系统防火墙都放行 UDP | 只放行 TCP。 |

---

## 0.3 方案一：VLESS + Reality + TCP（无域名/无证书首选）

### 适合谁

- 新手自建 VPS。
- 没有域名，或不想配置证书。
- 不准备使用 CDN。
- 想要现代、稳定、性能较好的节点。

### 服务端配置步骤

1. 进入面板，打开 **TLS / 安全配置**。
2. 新建 Reality 配置，生成或填写：
   - `private_key`
   - `public_key`
   - `short_id`
   - `server_name` / `SNI`
3. Reality 的 `SNI` 建议填写一个真实、稳定的 HTTPS 域名。
4. 新建入站，协议选择 `VLESS`。
5. 监听端口建议使用 `443` 或其它已放行 TCP 端口。
6. 传输层选择 `TCP`。
7. 安全类型选择 `Reality`，绑定刚才的 Reality/TLS 配置。
8. 用户 UUID 使用面板生成值，不要手动乱改。
9. 如果客户端支持，`flow` 可使用：

```text
xtls-rprx-vision
```

10. 保存配置，重启核心或服务。

### 客户端配置要点

客户端侧必须和服务端一致：

```text
协议：VLESS
地址：服务器 IP 或域名
端口：服务端监听端口
UUID：面板用户 UUID
传输：TCP
安全：Reality
SNI：服务端 Reality server_name
Public Key：服务端生成的 public_key
Short ID：服务端 short_id
Fingerprint/uTLS：chrome 或 firefox
Flow：与服务端一致，例如 xtls-rprx-vision
```

### 常见错误

- 把 `private_key` 填到客户端；客户端应填 `public_key`。
- `short_id` 服务端和客户端不一致。
- `SNI` 填了不存在或不稳定的站点。
- 端口只在系统防火墙放行，云安全组没放行。
- Reality 放到 CDN 后面，导致握手异常。

---

## 0.4 方案二：VLESS/Trojan + TLS + WebSocket（有域名/CDN 推荐）

### 适合谁

- 有域名。
- 想使用 Cloudflare 等 CDN。
- 想让流量表现得像普通 HTTPS 网站。

### 域名和 CDN 准备

1. 域名解析到服务器 IP，或开启 CDN 代理。
2. CDN 中确认开启 WebSocket 支持。
3. 选择 CDN 支持的 HTTPS 端口，例如常见的 `443`。
4. 确保证书域名、SNI、Host 是同一个域名。

### 服务端配置步骤

1. 在面板中新建 TLS 配置。
2. 证书可使用 ACME 申请，或手动填写证书路径。
3. 新建入站，协议选择：
   - `VLESS`：更现代，常用。
   - `Trojan`：简单，兼容性好。
4. 传输层选择 `WebSocket`。
5. 设置 WebSocket Path，例如：

```text
/vless
/trojan
/ws
```

6. 设置 Host 为你的证书域名，例如：

```text
example.com
```

7. 开启 TLS，并设置 SNI 为同一个域名。
8. 保存配置，重启核心或服务。

### 客户端配置要点

```text
协议：VLESS 或 Trojan
地址：域名，不建议直接填 IP
端口：443 或你实际开放的 HTTPS 端口
TLS：开启
SNI：证书域名
传输：WebSocket
Path：服务端 Path，必须完全一致
Host：证书域名/CDN 域名
```

### 常见错误

- 服务端 Path 是 `/vless`，客户端写成 `vless` 或 `/VLESS`。
- Host、SNI、证书域名不一致。
- CDN 没开启 WebSocket。
- CDN 使用了不支持的端口。
- 同时在反向代理和面板里重复终止 TLS，导致配置混乱。

---

## 0.5 方案三：Trojan + TLS + TCP（有域名、想简单稳定）

### 适合谁

- 有域名和证书。
- 不想折腾 WebSocket / gRPC / CDN。
- 希望配置简单、客户端兼容性好。

### 服务端配置步骤

1. 准备域名并解析到服务器。
2. 在面板中配置 TLS 证书。
3. 新建入站，协议选择 `Trojan`。
4. 监听端口推荐 `443`。
5. 传输层保持 `TCP`。
6. 开启 TLS，SNI 填证书域名。
7. 设置 Trojan 密码。
8. 保存并重启服务。

### 客户端配置要点

```text
协议：Trojan
地址：证书域名
端口：443
密码：服务端 Trojan 密码
TLS：开启
SNI：证书域名
传输：TCP
```

### 常见错误

- 用 IP 连接，但证书是域名证书。
- 忘记开启 TLS。
- 密码不一致。
- 证书过期或证书链不完整。

---

## 0.6 方案四：Hysteria2 + TLS（弱网、UDP、移动网络）

### 适合谁

- 移动网络、跨境高丢包线路。
- 游戏、语音、视频等需要 UDP 的场景。
- 服务器和客户端都允许 UDP。

### 服务端配置步骤

1. 准备域名和 TLS 证书。
2. 云安全组放行 UDP 端口，例如 `443/udp` 或自定义 UDP 端口。
3. 系统防火墙放行同一个 UDP 端口。
4. 新建入站，协议选择 `Hysteria2`。
5. 开启 TLS，SNI 填证书域名。
6. 设置用户密码。
7. 如果启用端口跳跃 `mport/server_ports`，建议同时设置 `hop_interval`。
8. 如启用 `salamander obfs`，客户端和服务端混淆密码必须一致。

### 客户端配置要点

```text
协议：Hysteria2
地址：域名或服务器 IP
端口：服务端 UDP 端口
密码：服务端用户密码
TLS：开启
SNI：证书域名
Obfs：如服务端开启，客户端必须一致
```

### 常见错误

- 只放行了 TCP，没放行 UDP。
- 云安全组放行了，系统防火墙没放行。
- 运营商或网络环境屏蔽 UDP。
- 带宽参数乱填过大，反而变慢。

---

## 0.7 方案五：TUIC + TLS（低延迟、UDP、游戏）

### 适合谁

- 需要低延迟 UDP。
- 客户端支持 TUIC。
- 想在弱网环境尝试 QUIC 类协议。

### 服务端配置步骤

1. 准备域名和 TLS 证书。
2. 放行 UDP 端口。
3. 新建入站，协议选择 `TUIC`。
4. 开启 TLS，SNI 填证书域名。
5. 设置 UUID 和 Password。
6. `congestion_control` 不确定时可先使用：

```text
cubic
```

7. `udp_relay_mode` 不懂时保持默认或留空。

### 客户端配置要点

```text
协议：TUIC
地址：域名或服务器 IP
端口：服务端 UDP 端口
UUID：服务端 UUID
Password：服务端 Password
TLS：开启
SNI：证书域名
Congestion Control：与服务端一致
```

### 常见错误

- UUID 和 Password 只填了一个。
- 端口按 TCP 放行，实际需要 UDP。
- 客户端内核太旧不支持当前 TUIC 参数。
- 强制 network=tcp，导致协议不可用。

---

## 0.8 本地入口：Mixed / Tun / TProxy 怎么选

### Mixed

适合只给本机软件提供 HTTP + SOCKS 代理。

推荐：

```text
监听地址：127.0.0.1
监听端口：7890
用途：浏览器、Telegram、开发工具等手动设置代理的软件
```

不要直接监听公网 `0.0.0.0`，除非你知道如何做访问控制。

### Tun

适合 Windows / macOS / Linux 桌面全局代理。

重点：

- 需要管理员权限。
- 必须处理 DNS，否则容易 DNS 泄漏。
- 不要同时运行多个 Tun 类代理。

### TProxy

适合 Linux 网关、OpenWrt、软路由透明代理。

重点：

- 需要 iptables/nftables 和策略路由。
- 必须排除局域网、服务器 IP、面板地址。
- DNS 劫持和分流规则必须配套。

---

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

当前状态：**已完成一轮兼容范围内的协议依赖更新**。

已更新并通过测试 / 构建检查的模块：

- `github.com/quic-go/quic-go`：`v0.59.0` → `v0.59.1`
- `github.com/sagernet/quic-go`：`v0.59.0-sing-box-mod.4` → `v0.59.0-sing-box-mod.5`
- `github.com/sagernet/sing-shadowsocks`：`v0.2.8` → `v0.2.9`
- `github.com/sagernet/sing-shadowsocks2`：`v0.2.1` → `v0.2.2`
- `github.com/sagernet/sing-shadowtls`：伪版本 → `v0.2.1`
- 已移除 `github.com/quic-go/quic-go => v0.57.1` 的 `replace` 固定。

暂不更新的模块：

- `github.com/sagernet/sing-box` 保持 `v1.13.12`。
- `github.com/sagernet/sing-tun` 保持 `v0.8.9`。

原因：

- `sing-box v1.13.13` 在当前 Windows 构建环境下会调用 `sing-tun` 的 `MyInterface()`。
- `sing-tun v0.8.10` 及当前可获取的更新分支接口已变为 `MyInterfaces()`，导致 Windows 编译失败。
- 因此本项目当前选择保留可编译、可测试通过的组合：`sing-box v1.13.12` + `sing-tun v0.8.9`。

已完成验证：

```text
[x] go test ./...
[x] Windows 发布标签构建检查
```

后续建议：

```text
下个版本：继续观察 sing-box / sing-tun 是否发布兼容稳定组合。
发布前：重点测试 QUIC、TUIC、Hysteria2、Naive、ShadowTLS、Tun / Redirect / TProxy。
如果 sing-box v1.13.13 后续修复 Windows 依赖接口，再单独升级核心并重跑完整回归。
```
