# 001 配套 Linux Agent 计划报告：转发 127.0.0.1/::1 监听（修订版 v2）

- 日期：2026-09-23（v2 修订）
- 状态：计划（待评审，已按“去配置化 + UDP 广播 + 随机通道”修订）
- 目标：WSL 内不管绑 `127.0.0.1/::1` 还是 `0.0.0.0/[::]`，所有监听端口自动转到 Windows 全网；零配置文件，开箱即用。

## 1. 你的意思，我的理解（先对齐）

1. **不要配置文件了**：删掉 `%HOMEPATH%/.wslpp/config.json` 整套（`onlyPredefined/predefined/ignore`），`lib/config` 直接删除。固定策略：**扫到啥就转啥**，无白名单。
2. **发现面用 UDP 广播**：Linux agent 在**固定、非独占 UDP 1033** 上周期广播；Windows 侧监听 `0.0.0.0:1033` 收广播。广播内容就是你的例子：
   ```
   ports
   22
   80
   443
   agent port
   3685
   ```
   即“端口清单 + 数据通道端口”。WSL 的 IP 不再靠 `wsl -- ip` 查，直接取 UDP 包的源地址，IP 漂移自愈。
3. **数据面走一个随机 agent 端口**：agent 启动时随机占一个可用 TCP 端口（如 `3685`），所有转发都收敛到这一条通道。Windows 侧每接到一个外部连接，就去连 `WSL_IP:agentPort` 并告诉 agent“要去几号端口”，agent 在 WSL 内部连 `127.0.0.1:P`（回环也能连上），再双向拷贝。

如果上面三条理解都对，继续往下看；不对的话只改这一节即可开工。

## 2. 为什么这样能解决 127.0.0.1 问题

老路径 `Windows → dial WslIp:P` 对只绑回环的服务必败（内核拒绝非 lo 到达的包，见 v1 §2）。新路径：

```
[局域网] → Windows 0.0.0.0:P → dial WslIp:agentPort(say 3685) → agent  dial 127.0.0.1:P → 服务
                                    ▲ 发送目标 "22\n"
```

回环的最后一跳发生在 WSL 网络命名空间**内部**，所以 127.0.0.1/::1 也能通。全网监听的端口同样走这条路，行为统一，不再区分 bind 地址。

## 3. 架构（固定方案，无分支）

```
WSL 内 wslpp-agent（新增，常驻）              Windows wslpp.exe（改造）
┌──────────────────────────────┐            ┌──────────────────────────────┐
│ discover: /proc/net + ss     │            │ UDP 0.0.0.0:1033 监听        │
│  全量 TCP LISTEN（不分 bind） │──广播────▶│ 得 {wslIp, ports[], agentPort}│
│ data: TCP :0 → 实际 agentPort │ 1033/UDP  │                              │
│  accept → 读 "P\n" → dial    │            │ 每 P 起 ListenTCP :P          │
│  127.0.0.1:P(→::1 兜底)→pipe │◀─TCP──────│ accept → dial agentPort →发"P"→pipe│
└──────────────────────────────┘            └──────────────────────────────┘
```

删掉的东西：`lib/config/*`、`storage.C`、`main.go` 里 config 轮询 goroutine、`GetWslIP()`、`GetLinuxHostPorts()` 的 `ss` 正则。留下的：`GetWindowsHostPorts()`（本机冲突跳过）、`proxy.go` 的 accept/pipe（改 dial 目标）。

## 4. 协议冻结（v1，不再可配）

### 4.1 发现：UDP 广播

- 端口：`1033/UDP`，两端都 `SO_REUSEADDR`（Linux 再加 `SO_REUSEPORT`，如可用）+ `SO_BROADCAST`，即“非独占”，允许多 agent/多实例共存、Windows 重启不抢占失败。
- 方向：只 Linux → Windows 单向广播，周期 `3s`。另加单播兜底：agent 同时向网关/Host IP（`/etc/resolv.conf` 的 nameserver，即 Windows）单播一份，防止 WSL2 NAT 下定向广播被吞。这是 WSL2 下广播可达性的关键兼容手段。
- 目标地址（按序都发）：`255.255.255.255` → 子网定向广播（如 `172.29.255.255`，按出口网卡掩码算）→ Host 单播。三发一收，Windows 去重即可。
- 负载（纯文本 `\n` 分隔，`LF`，末尾带 `\n`），严格沿用你的格式：
  ```
  ports\n22\n80\n443\nagent port\n3685\n
  ```
  解析规则：首行必须是 `ports`；遇到行 `agent port` 则下一行是 agent 通道端口；其余数字行是转发端口。未知行忽略（给未来留扩展位，不加版本号字段以保持你定的格式）。
- Windows 侧状态：`{wslIp=包源IP, ports, agentPort, lastSeen}`。`lastSeen > 9s`（3 个周期）视为 agent 失联，拆除该源建的全部代理（服务大概率随 WSL 关闭了）。

### 4.2 数据：单通道多路（每连接一目标）

- agent 启动：`ListenTCP :0` 取系统分配的随机空闲端口为 `agentPort`，写进广播。重启则变，Windows 以最新广播为准，旧通道连接自然断开重建。
- Windows 每收到外部连接 `c1`：
  1. `dial WslIp:agentPort` 得 `c2`（超时 5s，失败则关 `c1`）；
  2. 发 `"<P>\n"`（如 `"22\n"`，不带多余头）；
  3. `io.Copy` 双向转发（复用现有 `proxy.go:76-92` 逻辑）。
- agent 每收到 `c2`：
  1. 读一行 `P`（只收纯数字，非法直接关）；
  2. `dial 127.0.0.1:P`，失败则试 `::1:P`，都失败则关 `c2`（Windows 侧客户连接随之断开，日志一条）；
  3. 双向 `io.Copy`。agent 不做任何过滤（无白名单），只排除自身 `agentPort` 不进广播清单，避免自环（`1033` 是 UDP 端口，而发现只扫 TCP LISTEN，天然不会混入，无需排除）。

### 4.3 冲突与多源规则（固定，不可配）

- Windows 本机 `0.0.0.0:P` 已被占（`Netstat LISTENING` 命中）→ 该 P 跳过，日志一条（沿用“已占用则省略”）。
- 多 WSL 发行版同时广播（都有 agent）：同一 P 先到先得，后到的源的该 P 跳过并告警。本期只保证单发行版体验，多源只做到不崩。

### 4.4 用户程序停止监听后的断开语义（冻结）

先明确内核语义：**关 listener 不影响已建立连接**。所以分三种情况，不主动误杀：

- **已有连接（established）继续跑**：`client ↔ Windows ↔ agent ↔ backend` 四段中 `agent ↔ backend` 已经建连的，即使 discover 下轮扫不到该 P，也让它跑到任意一端 FIN/RST 为止。Windows 关闭 `Listener :P` 时只停新 accept，在飞连接继续 drain。
- **新连接立刻失败关闭**：端口消失后 agent 侧 `dial 127.0.0.1:P`（再试 `::1:P`）必败 → agent 直接关 `c2`；Windows 侧 `io.Copy` 遇到 EOF → 关 `c1`。客户端观感是“连上立刻断”（Windows 侧握手已完成，随后 FIN），日志采样一条，防刷屏。广播下个周期（≤3s）不再带该 P，Windows 随后拆 `Listener :P`。
- **后端 mid-connection 崩溃**：backend socket 发 FIN/RST → agent 侧 copy 返回 → agent 关 `c2` + backend leg → Windows 侧 copy 返回 → 关 `c1`。级联关闭即可，不需要额外 RST/错误码协议。
- **关闭规则**：任一方向 `io.Copy` 返回（EOF 或错）就 `Close` 两端全部连接。首期直接全关（现有 `proxy.go:76-92` 就是这个语义，沿用）；半关闭（`CloseWrite`）后续有长连接单向关需求再加。
- **防泄漏**：两端 `SetKeepAlive 15s` + `TCP_NODELAY`；agent 读首行 `"P\n"` 加 5s 读超时，`dial backend` 加 5s 超时；Windows `dial agentPort` 5s 超时。WSL 整体关闭则走 §4.1 的 9s 过期拆 Listener，在飞连接自然收敛；agent 重启（agentPort 变化）则旧连接随进程退出而断，客户端重连即自愈到新端口。

## 5. 改动清单（落到文件）

- 新增 `cmd/agent/main.go`（同仓，与主程序共一个 module）：discover（`/proc/net/tcp{,6}` 主 + `ss -tlnH` 兜底，含 127/::1/0.0.0.0/::全收）+ UDP 广播器 + TCP 通道服务。三者同进程，每 3s reconcile。
- 改造 `lib/service/service.go`：删 `GetWslIP/GetLinuxHostPorts`，新增 `discovery.go`（UDP 1033 监听、广播解析、过期清理）；保留 `GetWindowsHostPorts`。
- 改造 `lib/proxy/proxy.go`：`Proxy` 结构把 `WslIp` 改为 `{WslIp, AgentPort, TargetPort}`，`Start()` 的 dial 从 `WslIp:Port` 改为 `WslIp:AgentPort` + 首包发目标端口。
- 改造 `main.go`：删 config 相关全部（含 `storage.C`、`lib/config` 目录），主循环简化为 `收广播 → 差集(广播ports - Windows占用 - 自身排除) → 启停 ProxyPool → 1s sleep`。
- `Makefile`：`build`（windows exe）不变，新增 `build-agent`（`GOOS=linux GOARCH=amd64/arm64` 静态 `wslpp-agent`）+ `build-all`。
- 打包：`install-agent.sh`，`wsl --exec bash` 一键装（三档：systemd → cron `@reboot` → nohup 兜底）；README 重写“零配置两步跑”。

## 6. 分阶段实施

- Phase 0 Spike（0.5d）：WSL2 实测广播可达性（`255.255.255.255` vs 定向广播 vs Host 单播，Windows 收包率），定最终“三发”参数；实测 `dial 127.0.0.1:P` 桥接链路。
- Phase 1 agent（2d）：discover + 广播 + 通道服务 + 自排除；`GOOS=linux` 双架构编过。
- Phase 2 Windows（1-2d）：UDP 发现 + 新 proxy dial + 失联清理 + 删 config 全套。
- Phase 3 易用（1d）：安装脚本 + README + `make build-all`；默认 systemd、无 systemd 兜底验证。

## 7. 测试（无配置后更简单，断言更硬）

- `python3 -m http.server 18080 --bind 127.0.0.1`、`sshd:22`、`vite:3000` 全从局域网经 `WindowsIP:P` 可达，≤5s 生效。
- agent 重启（agentPort 变化）后 Windows 自动跟随，无需重启 exe。
- WSL 重启/IP 漂移后自愈；广播停 9s 后代理拆除。
- Windows 侧 P 被占用（如本机已有 443）则跳过不崩；UDP 1033 被多实例复用不报错。

## 8. 风险（只剩两个真风险）

1. **WSL2 广播不可达**：对策就是 §4.1 的“三发 + Host 单播”，Spike 阶段实测定参；最坏情况退化为纯 Host 单播（仍是 UDP 1033，只是点对点），架构不变。
2. **暴露面变大（你主动选的）**：以前只绑回环的服务现在会出网。这是“全转、无白名单”的代价，README 必须加粗警告；不做开关，要收敛只能停 agent 或关 Windows 程序。
