# Hetu v0.1 — 设计规格（Design Spec）

> 日期：2026-08-08　状态：待评审（Draft for review）
> 上游：需求与用户故事见 [../../requirements.md](../../requirements.md)；v0.1 范围与架构基线见 [../../mvp-v0.1.md](../../mvp-v0.1.md)。
> 本文件是 v0.1 的**实现规格**：架构、模块布局、接口、存储、进程模型、数据流、状态、错误处理、测试。

---

## 1. 目标与非目标

**目标（v0.1）**
- Local-only，单 CLI = Claude Code。
- 命令式 CLI（`hetu <cmd>`），core-first；长驻 daemon 持有 driven session。
- 发现历史 Claude Code 会话（JSONL）并按 cwd 归属到 Project。
- 浏览 / 详情 / 新建 / resume / 发 prompt；driven session 实时状态。

**非目标（推后）**
- HITL 审批/回答、Needs Attention/Unread 完整模型、通知 → v0.2。
- Codex / OpenCode → v0.5。
- Remote、大量 session 分页/性能优化 → 后续。

---

## 2. 关键决策

| 项 | 决策 | 理由 |
|---|---|---|
| 语言/运行时 | **Go** | 驱动 CLI 靠语言无关的线协议；Go 并发契合多 session 事件多路复用，单二进制好分发，TUI 生态成熟 |
| 进程模型 | **Daemon 模型** | 长驻 daemon 持有 driven session 子进程 + 事件流；CLI 是薄客户端；契合 control-center 愿景，App 未来复用 |
| 统一层 | **Hetu 自定义 Agent 接口** | 跨 CLI 一致性由自身接口提供，不依赖外部标准 |
| ACP | **不引入** | ACP 只管驱动、不含发现；三个 CLI 的原生面更全且第一方。仅当未来某 CLI 的 ACP 明显更优时作为该 Driver 的可替换后端 |
| 适配结构 | **每 CLI = 原生 DiscoverySource + 原生 Driver** | 发现与驱动独立、藏在接口后；发现永远 per-CLI（无跨 CLI 标准） |
| 发现范围 | **v0.1 保留原生发现** | 实现为独立 DiscoverySource adapter；首次打开即看到历史 Claude 会话 |
| 存储 | **SQLite，纯 Go 驱动 `modernc.org/sqlite`** | 无 CGO → 跨平台单二进制交叉编译无痛 |
| 平台 | **Linux + macOS** | unix socket 两平台一致；纯 Go 依赖保交叉编译 |

---

## 3. 总体架构

```text
┌──────────────────────────────────────────────────────────┐
│  CLI 薄客户端  cmd/hetu                                    │
│  hetu sessions / resume / send / new / search …           │
│  （一次性进程；经 unix socket 连 daemon）                  │
└───────────────┬──────────────────────────────────────────┘
                │ unix socket（本地）· JSON 帧
┌───────────────▼──────────────────────────────────────────┐
│  hetu daemon（长驻）  cmd/hetud                            │
│   SessionManager ── DriverPool ── DiscoveryScheduler       │
│          │              │                │                │
│          ▼              ▼                ▼                │
│   ┌─────────────────────────────────────────────────┐     │
│   │  Agent adapters                                   │     │
│   │   claude: DiscoverySource(JSONL) + Driver(SDK)   │     │
│   │   (future: codex, opencode)                       │     │
│   └─────────────────────────────────────────────────┘     │
│          │                                                 │
│   ┌──────▼──────────────────────────────────────────┐     │
│   │  Store / SessionRegistry  (SQLite, WAL)          │     │
│   │  projects · agents · sessions · session_events   │     │
│   └──────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────┘
```

客户端只做展示与编排；所有状态、子进程、事件流都在 daemon。App 未来直接复用 daemon（同一 socket/api）。

---

## 4. 模块布局

```text
hetu/
  go.mod
  cmd/
    hetu/        # 薄客户端入口
    hetud/       # daemon 入口（hetu serve / 自启动）
  internal/
    api/         # daemon↔client 线协议类型 + codec（共享）
    client/      # 薄客户端（cmd/hetu 调用）
    daemon/      # IPC handler · SessionManager · DriverPool · DiscoveryScheduler
    store/       # SQLite（SessionRegistry）：schema · migrations · queries
    agent/       # Agent 接口（DiscoverySource + Driver）+ registry
    claude/      # Claude Code adapter：discovery(jsonl) + driver(sdk/subprocess)
    project/     # project 模型 · cwd→project 解析
    session/     # session 领域模型 · 统一 status 映射
    config/      # ~/.hetu 配置（agents · projects）
    cli/         # 命令实现 + 输出格式化
  docs/
```

全部置于 `internal/`（app 而非库）；每个 CLI 一个 adapter 包；领域模型（session/project）与 store 解耦；每包可独立测试。真实 claude 依赖被隔离在 `internal/claude` 一个包内。

---

## 5. 核心接口（统一层，Go）

```go
// internal/agent/agent.go
package agent

type Agent interface {
	Name() string // "claude"
	DiscoverySource() DiscoverySource
	Driver() Driver
}

// DiscoverySource 枚举磁盘上已存在的 session（外部创建或 Hetu 创建），用于"原生发现"。
type DiscoverySource interface {
	Discover(ctx context.Context, opts DiscoverOpts) (<-chan DiscoveredSession, error)
}

type DiscoveredSession struct {
	Agent        string
	ExternalID   string    // CLI 原生 session id（如 claude session uuid / 文件名）
	CWD          string    // 工作目录，用于归属 Project
	Title        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
}

// Driver 驱动一个 session：新建/恢复、发 prompt、流事件。
// 审批应答（approve/deny）留给 v0.2。
type Driver interface {
	Start(ctx context.Context, req StartRequest) (Session, error)   // new 或 resume
	StatusOf(ctx context.Context, externalID string) (Status, error) // 非 driven session 的历史态推断
}

type StartRequest struct {
	Mode   StartMode // StartNew | StartResume
	CWD    string
	Prompt string   // 可选，新建时立即发首条
	ExternalID string // resume 时指定
}

// Session = 一个 Hetu 正在驱动的活 session（daemon 持有）。
type Session interface {
	ID() string
	ExternalID() string
	Send(ctx context.Context, prompt string) error
	Events() <-chan Event // text / tool / status（approval 事件 v0.2）
	Status() Status
	Close() error
}
```

`Event`、`Status`、`DiscoverOpts` 等附属类型定义在 `internal/session` 与 `internal/agent` 包。

---

## 6. 存储（SQLite，SessionRegistry）

- 驱动：`modernc.org/sqlite`（纯 Go，无 CGO）。开启 `PRAGMA journal_mode=WAL`。
- 迁移：嵌入 SQL 文件 + 极简迁移器（v0.1 不引 golang-migrate）。
- 写并发：单写入串行化（SQLite 单写者）；读并发无锁。

```sql
CREATE TABLE projects (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  path        TEXT NOT NULL UNIQUE,
  created_at  INTEGER NOT NULL
);

CREATE TABLE agents (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,   -- "claude"
  enabled     INTEGER NOT NULL DEFAULT 1,
  binary      TEXT,                   -- 可选，覆盖 PATH 探测
  created_at  INTEGER NOT NULL
);

CREATE TABLE sessions (
  hetu_id      TEXT PRIMARY KEY,       -- Hetu 内部 id
  agent        TEXT NOT NULL,
  external_id  TEXT NOT NULL,          -- CLI 原生 id
  project_id   INTEGER REFERENCES projects(id),
  host         TEXT NOT NULL DEFAULT 'local',
  cwd          TEXT,
  title        TEXT,
  status       TEXT NOT NULL,          -- Running/Completed/Error/Idle/Unknown
  driven       INTEGER NOT NULL DEFAULT 0,
  unread       INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL,
  last_event_at INTEGER,
  UNIQUE(agent, external_id)           -- 重复发现 = 幂等 upsert
);

CREATE TABLE session_events (
  id           INTEGER PRIMARY KEY,
  session_hetu TEXT NOT NULL REFERENCES sessions(hetu_id),
  seq          INTEGER NOT NULL,
  kind         TEXT NOT NULL,          -- text/tool/status/...
  payload      TEXT,                   -- JSON
  ts           INTEGER NOT NULL,
  UNIQUE(session_hetu, seq)
);
```

`(agent, external_id)` 唯一约束 → 重复发现是幂等 upsert。

---

## 7. Daemon 进程模型

**启动**
- `hetu serve`：前台跑（开发/调试）。
- 自启动：客户端连不上 socket 时，detach 拉起 `hetud`（`Setsid` re-exec），轮询 socket 就绪后连接 → `hetu sessions` 无需预先启动 daemon 即可用。

**Driven session 生命周期**
- 每个 driven session = **1 个 claude 子进程 + 1 个读事件流的 goroutine**，注册在 `SessionManager`（`map[hetuID]*liveSession`）。
- goroutine 读 stream-json 事件 → 更新内存 Status + 写 store（status/last_event_at）+ 扇出给已订阅客户端。
- 客户端可订阅某 session 事件流（长连接，`--attach`/`--watch`；Ctrl-C 退出订阅但**不打断** daemon 内的 session）。**订阅从订阅时刻起向前推送新事件**；历史事件经 `hetu session <id>` 从 store 读取。

**v0.1 简化**
1. **daemon 关闭 = driven session 停止**：SIGTERM 优雅停 → `Close()` 所有子进程。不 detach 孤儿进程。Claude transcript 持续落盘（JSONL），session 不真丢，事后可 re-resume。
2. store 写并发走 WAL + 单写入串行化。

---

## 8. 关键命令数据流

| 命令 | 流程 |
|---|---|
| `hetu sessions [-p P] [-s S]` | client→daemon `ListSessions` → 查 store ＋ driven 取实时态 → 快照 → 格式化（project-first 打平） |
| `hetu resume <id>` | 未驱动则 `Driver.Start(resume)` → 起 claude 子进程 + 注册 + 事件 goroutine → 返回；`--attach` 持续流事件 |
| `hetu send <id> "…"` | 未驱动则先 takeover(resume)，再 `Session.Send(prompt)` → ack；`--watch` 流式回包 |
| `hetu new [-p P]` | `Driver.Start(new)` 在 project cwd 起新 claude session → 注册 → 返回 id |
| `hetu session <id>` | 元数据 + 近期事件 + 实时状态 |
| 发现（后台） | `DiscoveryScheduler`：daemon 启动时 + 周期 / `hetu discover` 按需 → 各 `DiscoverySource.Discover` → upsert store + cwd→project 归属 |

---

## 9. 统一状态模型（v0.1 子集）

- 枚举：`Running / Completed / Error / Idle / Unknown`（`Waiting-for-Input/Approval` 随 v0.2 HITL）。
- **Driven session**：状态来自事件流。流式 tool/text→`Running`；Stop→`Completed`；error→`Error`。
- **非 driven（历史态）**：`Driver.StatusOf(externalID)` 解析 JSONL transcript 末尾 → 末条 assistant 结果→`Completed`；error→`Error`；否则 `Idle/Unknown`（启发式，文档标注）。

---

## 10. 跨平台（Linux + macOS）

- **unix socket** 两平台一致：`net.Listen("unix", p)` / `net.Dial("unix", p)`，代码零差异。
- 仅 socket **文件路径约定**按 `runtime.GOOS` 解析：
  - Linux：优先 `$XDG_RUNTIME_DIR/hetu/hetu.sock`，回退 `~/.hetu/hetu.sock`
  - macOS：优先 `$DARWIN_USER_TEMP_DIR/hetu.sock`，回退 `~/.hetu/hetu.sock`
  - 任意平台可用 `HETU_SOCKET` 覆盖。
- **纯 Go 依赖**（含 SQLite）→ 交叉编译一行：`GOOS=darwin GOARCH=arm64 go build` / `linux/amd64` / `linux/arm64`。
- Claude 存储路径 `~/.claude/projects/...` 在两平台均基于 `$HOME`，一致；CLI 探测用 `exec.LookPath("claude")`，不写死路径。
- Windows 非目标（设计不排除，但不投入）。

---

## 11. 错误处理

- **Adapter** 返回类型化错误（agent 未装、session 文件缺失、子进程退出）；daemon 包装回传，CLI 友好提示。
- **claude 崩溃/非零退出**：goroutine 检测 EOF/exit → Status=`Error`（干净退出=`Completed`）→ 落盘 → 移出 registry；session 仍可 re-resume。
- **store busy/locked**：WAL + 退避重试；致命错误记日志并降级。
- **socket 连不上**：尝试自启动；仍失败 → 明确提示 `hetu serve`。
- **幂等**：发现 upsert；resume 已驱动 session → 返回现有 handle，不重复 spawn。
- **context**：所有长操作带 ctx；客户端断开只取消订阅，不打断 daemon 内 session。

---

## 12. 测试策略

接口优先 → `Driver`/`DiscoverySource` 是接口，daemon/CLI 全可用 fake 测；真实 claude 依赖隔离在 `internal/claude`。

- **DiscoverySource(claude)**：testdata JSONL fixture → 断言 metadata/cwd/历史状态。无需真 claude。
- **Driver(claude)**：单测 stream-json 事件解析器（录制 event fixture → 断言 Status 跃迁）。
- **daemon / SessionManager**：用 `fakeDriver` 测生命周期 / 事件扇出 / 并发 / 优雅停。
- **Store**：临时文件 SQLite 测迁移 / upsert 幂等 / 查询。
- **CLI**：表格输出 golden test + 内嵌 fake daemon 的往返测试。
- **（可选）真 e2e**：CI 跑真实 `claude -p` trivial prompt，需 API key，gated，非阻塞。

---

## 13. 未来（v0.2+）

- HITL：审批/回答（Claude `can_use_tool` 回调 / hooks；事件流已具 `approval` 事件位）。
- 通知：desktop notify + Needs Attention / Unread 完整模型。
- 多 CLI：Codex（app-server JSON-RPC）、OpenCode（HTTP+SSE），各加一个 adapter 包。
- Remote host。
- 会话内全文搜索（SQLite FTS5）、大量 session 分页/性能。
- detach 孤儿子进程、daemon 重启后 driven session 续接。
