# Hetu v0.1（MVP）功能定义

> 状态：**已冻结**（2026-08-08），随讨论持续更新。本文件为 v0.1 实现基线。
> 上游：完整需求与用户故事见 [requirements.md](./requirements.md)；三 CLI 可行性调研结论：方案 B（官方线协议）对 Claude Code / Codex / OpenCode 均成立。

## 范围决策（已确认）

- **平台**：Local-only（Remote 推后）
- **Agent**：单 CLI = **Claude Code**（Codex / OpenCode 推后至 v0.5）
- **界面**：**CLI 优先**（命令式 `hetu <cmd>`），core-first；全屏 TUI 紧随；App(GUI) 以后接同一个 store
- **HITL（审批/回答）**：推后至 **v0.2**
- **通知**：推后至 **v0.2**（事件流已具备，差通知 UX）
- **语言 / 运行时**：**Go**（驱动三个 CLI 靠语言无关的线协议，与语言无关；选 Go 因其并发模型契合多 session 事件多路复用、单二进制好分发、TUI 生态成熟）
- **ACP**：**不引入**。统一层是 Hetu 自己的 Agent 接口；每 CLI 用原生 `DiscoverySource` + `Driver`。ACP 仅当未来某 CLI 上明显更优时，作为该 CLI Driver 的可替换后端（详见"架构基线"）

## 架构基线（已确认）

**每个 CLI = 两个独立 adapter，藏在 Hetu 的 Agent 接口之后：**

```text
Agent 接口（Hetu 统一层，不依赖 ACP）
├── DiscoverySource   发现/枚举 session + 读 metadata      （per-CLI 原生）
└── Driver            驱动 session：prompt / events / permission  （per-CLI 原生）
```

- **DiscoverySource（原生）**：Claude = parse JSONL（`~/.claude/projects/<enc-cwd>/*.jsonl`）；未来 Codex/OpenCode = 各自 list/read API。**没有跨 CLI 的发现标准，这一层永远 per-CLI。**
- **Driver（原生）**：Claude = Agent SDK / headless stream-json / hooks；未来 Codex = `app-server` JSON-RPC，OpenCode = HTTP+SSE。接口设计成 `prompt / events / permission` 的自然形状（恰好与 ACP 重合，但并非追随 ACP）。
- **SessionRegistry（Hetu 本地 store）**：单一 source of truth，记录 Hetu 创建/接管过的 session；core-first，天生可被 CLI 与 App 共享。DiscoverySource 发现的"外部创建"session 也纳入这里。
- **为什么不用 ACP**：ACP 只管驱动、不含发现；即便 OpenCode（ACP 第一方），其 HTTP API 已同时覆盖驱动+发现+权限，用 ACP 反而要多挂一套发现接口；Claude 的 ACP 仍非第一方（`anthropics/claude-code#6686`）。故三个 CLI 上 ACP 都不更省事。唯一改口的条件：ACP 像 LSP 那样全 CLI 第一方 + 补上发现能力——今天不是。

## 状态规则（统一表述）

> **Hetu 驱动的 session = 实时状态；未被接管的（含原生 CLI 开的）= 历史态（最近状态来自 transcript）。**
> 任何 session 经 Hetu resume 即转为 driven，获得实时状态。

## 发现范围（已确认）

- v0.1 **保留原生发现**（US-04 / US-05），实现成独立的 **DiscoverySource adapter**（与核心解耦）。
- 效果：首次打开 Hetu 即自动看到所有历史 Claude Code 会话，并按 cwd 自动归属到 Project。

## IN — P0（必做）

1. **Claude Code adapter**：DiscoverySource（JSONL 发现）+ Driver（Agent SDK 驱动 + 流事件状态）
2. **Project 模型**：路径绑定；session 按 cwd **自动归属**到项目；支持手动建项目绑目录
3. **Session 发现**：枚举历史 Claude Code 会话，读 metadata（title/summary、created/updated、cwd、消息数、agent）
4. **Session 列表/浏览**：project-first 打平、统一状态映射、默认按"关注/活跃"排序
5. **Resume + 发 prompt**：Hetu 接管 stream → 实时状态；给活跃 session 发新 prompt
6. **新建 session**：在某 Project 内创建并绑定 Project/Folder/Agent/Host
7. **SessionRegistry / 本地 store**：单一 source of truth，core-first，CLI 与 App 共享

## IN — P1（可裁）

8. 基础搜索（title/summary）+ 按状态/时间筛选
9. Session 详情（transcript 摘要/浏览）

## OUT（明确推后）

- HITL 审批/回答、完整 Needs Attention / Unread 模型、通知 → **v0.2**
- Codex / OpenCode → **v0.5**
- Remote、大量 session 分页/性能优化 → 后续

## CLI 命令面（v0.1 命式）

```text
hetu agents                              # 配置/查看 agent，检测是否安装/可用
hetu projects                            # 列项目（含 session 计数）
hetu sessions [--project P] [--status S] # 列 session（project-first 打平）
hetu session <id>                        # 详情
hetu new [--project P]                   # 新建 session
hetu resume <id>                         # resume（Hetu 接管 stream）
hetu send <id> "<prompt>"                # 给活跃 session 发 prompt
hetu search <query>                      # 搜索 session
```

## 未决（下一步）

- **详细架构提案**：目录结构、store 选型（倾向 SQLite）、Agent / Driver / DiscoverySource 接口定义、driven session 进程模型、CLI 分层。
  - 语言（Go）、ACP（不引入）、双 adapter 结构（每 CLI = 原生 DiscoverySource + 原生 Driver）**已定**。
