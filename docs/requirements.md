# Hetu 项目需求

> 状态：产品需求基线（2026-08-08 冻结）。本文件为 Hetu 完整产品需求与用户故事，描述 **1.0 愿景**。
> v0.1（MVP）的范围裁剪见 [docs/mvp-v0.1.md](./mvp-v0.1.md)。

## 1. 产品定位

**Hetu（河图）是一个统一管理 AI Coding CLI、Project 和 Session 的工作空间。**

它面向经常同时使用多个 AI Coding 工具的开发者，例如：

```text
Claude Code
Codex
OpenCode
未来其他 Coding Agent CLI
```

Hetu 同时提供：

```text
Hetu CLI
Hetu App
```

并支持：

```text
Local Host
Remote Host
```

Hetu 主要解决三个核心问题：

> 我有哪些 AI Coding Session？

> 这些 Session 现在分别是什么状态？

> 哪些 Session 正在等待我处理？

---

## 2. 用户痛点

当开发者同时使用多个 AI Coding CLI 时，会逐渐出现大量 Session：

```text
Project Alpha
├── Claude Session
├── Codex Session
├── Claude Session
└── OpenCode Session

Project Beta
├── Codex Session
└── Claude Session

Remote Host
├── Claude Session
└── Codex Session
```

现有工具通常各自管理自己的 Session，导致用户很难快速知道：

```text
有哪些 Session
Session 属于哪个 Project
Session 来自哪个 CLI
最近使用的是哪个 Session
哪个 Session 仍然在运行
哪个 Session 等待输入
哪个 Session 等待 Approval
哪个 Session 已经完成
哪个 Session 出错
哪个 Session 有新消息
如何找到过去的 Session
如何快速 Resume
```

Hetu 希望把这些分散状态统一起来。

---

## 3. 核心产品模型

Hetu 从用户角度主要包含以下概念：

```text
Host
Project
Session
Agent
Status
Interaction
Notification
```

关系可以理解为：

```text
Host
└── Project
    └── Session
        ├── Agent
        ├── Status
        ├── Activity
        └── Interaction
```

其中：

**Host**

代表运行 Coding CLI 的机器。

例如：

```text
Local
Development Server
Remote Workstation
```

---

**Project**

代表用户的一个代码工作空间。

初期一个 Project 可以关联一个目录：

```text
Project Alpha
/home/user/projects/project-alpha
```

---

**Agent**

表示创建或者运行 Session 的 AI Coding 工具：

```text
Claude Code
Codex
OpenCode
```

---

**Session**

代表一次完整的 AI Coding 会话。

Session 是 Hetu 中非常重要的一等对象。

---

## 4. Project-first 产品原则

Hetu 的核心组织方式应该是：

```text
Project
    ↓
Session
```

而不是：

```text
Project
    ↓
Claude / Codex / OpenCode
    ↓
Session
```

也就是说：

> **CLI 类型是 Session 的属性，而不是默认的一级信息层级。**

例如用户进入：

```text
Project Alpha
```

默认看到的是所有 Agent 的 Session：

```text
Needs Attention
────────────────────────────────────────
[Codex]   API redesign       Waiting Input
[Claude]  Refactor module    Waiting Approval

Running
────────────────────────────────────────
[Claude]  Add caching        Running
[OpenCode] Add tests         Running

Recent
────────────────────────────────────────
[Codex]   Fix parser         Completed
[Claude]  Update docs        Completed
```

而不是默认分成：

```text
Claude
  ...

Codex
  ...

OpenCode
  ...
```

因为用户最重要的问题不是：

> Claude 在干什么？

而是：

> **整个项目现在有什么事情需要我处理？**

---

## 5. 按 Agent 分类仍然需要支持

虽然默认 Session 打平展示，但必须支持按照 CLI / Agent：

```text
筛选
搜索
分组
```

例如：

```text
[ All ] [ Claude ] [ Codex ] [ OpenCode ]
```

或者允许切换：

```text
View

● Activity
○ Agent
○ Time
```

默认：

```text
Activity
```

按照关注程度组织：

```text
Needs Attention
Running
Recent
```

切换到：

```text
Agent
```

后可以变成：

```text
Claude
────────────────
Session A
Session B

Codex
────────────────
Session C
Session D

OpenCode
────────────────
Session E
```

因此可以总结为：

> **跨 Agent 聚合是默认体验，Agent 分类是辅助浏览能力。**

---

## 6. CLI / Agent 管理

用户需要能够配置 Hetu 可以管理哪些 Coding CLI。

例如：

```text
Claude Code     Enabled
Codex           Enabled
OpenCode        Disabled
```

用户应该可以：

```text
添加 Agent
启用 Agent
禁用 Agent
删除 Agent
查看 CLI 是否安装
查看 CLI 版本
查看 CLI 是否可用
查看连接状态
```

Hetu 应尽量屏蔽不同 CLI 的内部差异。

用户不需要知道：

```text
Session 存在哪个目录
Session 文件是什么格式
Resume 命令是什么
某个 CLI 使用什么内部协议
```

---

## 7. Project 管理

用户可以：

```text
创建 Project
删除 Project
重命名 Project
修改关联目录
打开关联目录
查看最近活动
查看 Session 数量
查看需要关注的 Session 数量
```

例如：

```text
Projects

Project Alpha                 2
Project Beta                  0
Project Gamma                 4
```

数字代表：

```text
Needs Attention / Unread
```

数量。

---

## 8. Session 自动发现

这是 Hetu 的核心需求之一。

Hetu 不应该只能看到：

```text
通过 Hetu 创建的 Session
```

还需要尽可能发现用户过去通过原生 CLI 创建的 Session。

例如用户之前执行过：

```bash
cd ~/projects/project-alpha
claude
```

或者：

```bash
codex
```

之后第一次打开 Hetu，也应该能够看到过去存在的 Session。

因此：

> **Existing Session Discovery 是核心产品能力。**

Hetu 应该能够将历史 Session 自动归属到对应 Project。

---

## 9. Session 列表

每个 Session 至少应该展示：

```text
Title / Summary
Agent
Status
Host
Created Time
Last Activity
Unread
Needs Attention
```

例如：

```text
[Claude] Refactor authentication

Running
Updated 2 minutes ago
Local
```

或者：

```text
[Codex] Implement API client

Waiting for Approval
Updated 20 seconds ago
Remote Host
```

---

## 10. Session 状态

不同 Agent 的底层状态可能不同，但 Hetu 应该提供统一状态。

建议用户层至少包括：

```text
Running

Idle

Waiting for Input

Waiting for Approval

Completed

Error

Disconnected

Unknown
```

其中最重要的是：

```text
Waiting for Input
Waiting for Approval
```

因为这意味着：

> Agent 无法继续，需要用户介入。

---

## 11. Session 默认排序

默认排序不应该仅仅是：

```text
updated_at DESC
```

而应该体现用户关注优先级。

推荐默认逻辑：

```text
Needs Attention
      ↓
Running
      ↓
Recently Active
      ↓
Historical
```

因此一个 20 分钟前等待 Approval 的 Session，可以比一个 5 秒前完成的 Session 更值得出现在顶部。

---

## 12. Session 搜索

用户应该能够搜索历史 Session。

例如搜索：

```text
authentication
database
websocket
testing
```

搜索范围可以逐步覆盖：

```text
Session title
Session summary
Conversation content
Project
Agent
```

---

## 13. Session Filter

用户应该可以按照以下维度筛选：

```text
Agent

Claude
Codex
OpenCode
```

状态：

```text
Running
Waiting
Completed
Error
```

时间：

```text
Today
Last 7 Days
Last 30 Days
Custom
```

Host：

```text
Local
Remote Host A
Remote Host B
```

---

## 14. Pagination / 大量 Session

一个用户长期使用以后可能产生：

```text
数百
数千
甚至更多 Session
```

Hetu 必须支持大量 Session 浏览。

用户体验上可以表现为：

```text
Pagination
```

或者：

```text
Infinite Scroll
```

但产品需求本身是：

> Session 数量增加后，搜索和浏览体验不能明显下降。

---

## 15. 创建 Session

用户可以在某个 Project 内：

```text
+ New Session
```

然后选择 Agent：

```text
Claude Code
Codex
OpenCode
```

例如：

```text
Project Alpha

New Session

Agent:
> Claude Code
  Codex
  OpenCode
```

创建后，Session 自动与：

```text
Project
Folder
Host
Agent
```

关联。

---

## 16. Resume Session

用户应该能够选择过去的 Session：

```text
Resume
```

继续之前的上下文。

用户不需要记住：

```text
Session ID
CLI Resume 参数
Session 文件位置
```

理想体验就是：

```text
找到 Session
    ↓
Resume
    ↓
继续工作
```

---

## 17. Session 交互

Hetu 不应该只是一个 Session Browser。

对于 Hetu 管理中的活跃 Session，用户应该能够处理 Agent 发出的人工交互请求。

包括：

```text
输入文本

确认

选择

Approval

Permission
```

例如：

```text
Claude requires approval

Run:

npm install

[Approve] [Deny]
```

或者：

```text
Codex needs your input

Which approach should be used?

[REST]
[gRPC]
[JSON-RPC]
```

---

## 18. Needs Attention

Hetu 需要有一个非常明确的：

```text
Needs Attention
```

概念。

以下事件通常可以导致 Session 进入 Needs Attention：

```text
Waiting for Input

Waiting for Approval

Error

新的重要消息
```

这样用户打开 Hetu 后，第一眼应该能看到：

> 哪些 Agent 正在等我？

---

## 19. Unread

Session 还应该具有：

```text
Read
Unread
```

概念。

例如：

```text
Project Alpha        3
```

表示该 Project 有 3 个值得关注的未读 Session / Event。

用户查看对应内容后，Unread 状态可以清除。

---

## 20. Notification

如果 Session 当前没有被用户查看，同时产生重要事件：

```text
Waiting for Input

Waiting for Approval

Completed

Error
```

Hetu 应该能够产生通知。

例如：

```text
Hetu

Codex needs your approval

Project Alpha
Implement API client
```

点击通知以后：

```text
Project
   ↓
Session
   ↓
具体 Interaction
```

---

## 21. Local Host

Hetu 默认管理当前机器。

例如：

```text
Local

Project Alpha
Project Beta
Project Gamma
```

其中可能同时运行：

```text
Claude
Codex
OpenCode
```

---

## 22. Remote Host

用户还可以添加远程机器。

例如：

```text
Hosts

Local

Development Server

Remote Workstation
```

每一个 Host 都可能拥有自己的：

```text
Projects
Agents
Sessions
```

例如：

```text
Development Server
└── Project Alpha
    ├── [Claude] Session A
    └── [Codex] Session B
```

---

## 23. Local / Remote 一致体验

这是一个重要产品原则：

> **Session 在 Local 还是 Remote，不应该彻底改变用户的操作方式。**

理想情况下，无论 Session 在哪里，用户都应该可以：

```text
Open
Resume
Create
Search
Send Input
Approve
View Status
```

Host 只是 Session 的一个重要属性。

---

## 24. Hetu CLI

Hetu 必须提供 CLI 产品。

用户可以：

```bash
hetu
```

进入交互式界面，也应该支持常见命令式操作。

CLI 产品需要覆盖核心场景：

```text
查看 Hosts
查看 Projects
查看 Sessions
搜索 Session
筛选 Session
创建 Session
Resume Session
查看 Session 状态
处理人工交互
```

CLI 不是 Desktop App 的附属品。

即使用户只通过 SSH 使用服务器，也应该可以完整使用 Hetu 的核心功能。

---

## 25. Hetu App

同时提供 GUI App。

App 主要服务：

> 长期开着多个 Coding Agent，希望有一个统一控制中心的用户。

App 应该能够：

```text
浏览 Hosts
浏览 Projects
浏览 Sessions
搜索
筛选
创建 Session
Resume
查看状态
处理人工交互
接收通知
```

---

## 26. CLI 与 App 一致性

CLI 和 App 可以有不同交互设计，但看到的核心对象和状态必须一致：

```text
Host
Project
Session
Agent
Status
Interaction
Notification
```

例如：

```bash
hetu sessions
```

看到：

```text
Project Alpha

Codex    API Client    Waiting Approval
Claude   Refactor      Running
```

Desktop App 中也应该看到相同状态。

---

# 核心用户故事

## Project & Agent

**US-01**

作为用户，我希望配置 Claude Code、Codex、OpenCode 等 AI Coding CLI，这样可以在 Hetu 中统一使用它们。

**US-02**

作为用户，我希望看到某个 Agent 当前是否可用，这样可以快速发现安装或连接问题。

**US-03**

作为用户，我希望创建一个 Project 并绑定本地或远程目录，这样 Session 可以按照项目组织。

---

## Session Discovery

**US-04**

作为用户，我希望 Hetu 自动发现以前通过原生 Claude/Codex/OpenCode 创建的 Session，这样安装 Hetu 后不会失去过去的工作历史。

**US-05**

作为用户，我希望 Hetu 自动判断 Session 属于哪个 Project。

---

## Session Browsing

**US-06**

作为用户，我希望进入 Project 后直接看到所有 Agent 的 Session，而不用分别进入 Claude、Codex、OpenCode。

**US-07**

作为用户，我希望每一个 Session 都清楚显示是由哪个 Agent 创建的。

**US-08**

作为用户，我希望可以只查看 Claude、Codex 或 OpenCode 的 Session。

**US-09**

作为用户，我希望可以切换成按照 Agent 分组的 Session 视图。

---

## Search & Filter

**US-10**

作为用户，我希望搜索历史 Session，这样可以快速找到之前讨论过某个问题的会话。

**US-11**

作为用户，我希望按照 Agent、Status、Time、Host 筛选 Session。

**US-12**

作为用户，我希望在存在大量 Session 时仍然能够快速分页或连续浏览。

---

## Activity & Status

**US-13**

作为用户，我希望看到 Session 当前是否 Running、Waiting、Completed 或 Error。

**US-14**

作为用户，我希望需要人工处理的 Session 自动出现在更加显眼的位置。

**US-15**

作为用户，我希望 Session 默认按照"需要关注程度 + 活跃程度"排序，而不仅仅按照创建时间排序。

---

## Creating & Resuming

**US-16**

作为用户，我希望从 Hetu 中直接创建 Claude、Codex 或 OpenCode Session。

**US-17**

作为用户，我希望一个新 Session 自动关联当前 Project、Folder、Agent 和 Host。

**US-18**

作为用户，我希望找到历史 Session 后点击 Resume 就可以继续工作。

---

## Human in the Loop

**US-19**

作为用户，我希望看到哪些 Session 正在等待我的输入。

**US-20**

作为用户，我希望看到哪些 Session 正在等待我的 Approval。

**US-21**

作为用户，我希望直接在 Hetu 中回答 Agent 的问题。

**US-22**

作为用户，我希望直接在 Hetu 中 Approve / Deny Agent 请求。

---

## Notifications

**US-23**

作为用户，我希望一个后台 Session 需要我时收到通知。

**US-24**

作为用户，我希望 Agent 工作完成以后可以收到通知。

**US-25**

作为用户，我希望 Session 出错时得到通知。

**US-26**

作为用户，我希望点击通知后直接跳到产生事件的 Session。

---

## Local & Remote

**US-27**

作为用户，我希望 Hetu 管理当前电脑上的 Coding Sessions。

**US-28**

作为用户，我希望添加一台远程机器。

**US-29**

作为用户，我希望看到远程机器上的 Project 和 Session。

**US-30**

作为用户，我希望本地与远程 Session 拥有基本一致的操作体验。

**US-31**

作为用户，我希望能够明确知道某个 Session 当前运行在哪台 Host。

---

## CLI & App

**US-32**

作为用户，我希望通过 Hetu CLI 完成主要 Session 管理操作，这样可以在纯 Terminal / SSH 环境中工作。

**US-33**

作为用户，我希望通过 Hetu App 获得完整的可视化 Session 管理体验。

**US-34**

作为用户，我希望 CLI 与 App 看到一致的 Projects、Sessions 和状态。

**US-35**

作为用户，我希望在 App 中长期观察多个 Agent，同时只在需要我的时候介入。

---

# Hetu 当前最核心的用户旅程

一个典型用户打开 Hetu：

```text
Hetu

Projects

Project Alpha            2
Project Beta             0
Project Gamma            1
```

进入：

```text
Project Alpha
```

看到：

```text
Needs Attention

[Codex]
Implement authentication
Waiting for Approval


[Claude]
Refactor API
Waiting for Input


Running

[OpenCode]
Add test coverage
Running


Recent

[Claude]
Update documentation
Completed
```

用户先处理：

```text
Waiting for Approval
```

然后离开这个 Session。

一段时间后：

```text
Claude completed
Project Alpha · Refactor API
```

Hetu 发出通知。

用户点击：

```text
Notification
    ↓
Project Alpha
    ↓
Claude Session
```

继续工作。

---

## 一句话需求定义

最终可以把 Hetu 的产品需求浓缩成：

> **Hetu 是一个统一管理本地与远程 AI Coding Sessions 的工作空间，让用户能够跨 Claude Code、Codex、OpenCode 等 Agent 发现、搜索、创建、恢复和监控 Session，并及时知道哪些 Agent 正在等待人工介入。**

同时有一条很重要的产品原则：

> **Project-first，Session-first，Agent-second。**

也就是说：

```text
Project
  ↓
Session
```

是 Hetu 的主要用户心智模型，而：

```text
Claude / Codex / OpenCode
```

主要作为 Session 的来源、标识、Filter 和可选分组维度存在。
