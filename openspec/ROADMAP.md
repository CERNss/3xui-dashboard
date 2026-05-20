# 3xui-dashboard — Parity Roadmap

The working backlog tracking how far we are from full
**sspanel-uim** parity, organized strictly by the five pillars
sspanel itself markets. Each row is an actionable line item with a
single state. Update this file whenever a change ships.

- 📁 Canonical specs (what IS): `openspec/specs/<module>/spec.md`
- 📁 Proposed/in-flight changes: `openspec/changes/<name>/`
- 📁 This file: what's MISSING + in what order we close it

## At-a-glance

```
                            完成度    打分逻辑
1. 运维管理   ████████████░░░░░░░░  60%   后端 90% + admin UI 缺 3 页 + cron 缺 3 任务
2. 多协议     █████████████░░░░░░░  67%   节点 4/5 (3x-ui 真实 surface) + 订阅 2.5/5 (Clash/sing-box/SIP008 全缺)
3. 支付系统   ████░░░░░░░░░░░░░░░░  20%   骨架齐 + 0 个真实网关 + 计费模式单一
4. 通知系统   ████████░░░░░░░░░░░░  40%   webhook/mail 齐 + 0 个 bot + 邮件无队列
5. 用户界面   ██████████░░░░░░░░░░  50%   admin 50% + portal 0% + 设计系统 95%
─────────────────────────────────────────
综合（5 维均值） █████████░░░░░░░░░░░  ~47%
```

> **协议 scope**：节点能跑什么由 **3x-ui** 上游决定（sspanel 仅借鉴 5 大产品维度，不约束协议）。
> Hysteria2 / TUIC v5 不是 3x-ui 的能力（Xray-core 不支持），所以这些**不在 ROADMAP 里**。
> 同理 AnyTLS / Hysteria1 等也不在 scope。

Status legend used throughout:

- ✅ **done** — shipped, spec'd, tested
- ⚠️ **partial** — code or backend exists, but not user-reachable / not feature-complete
- ❌ **missing** — zero code
- 🚧 **in-flight** — has an `openspec/changes/<name>/` proposal open

---

## 1️⃣ 运维管理 | Operations Management — **60%**

Day-to-day operating the panel: nodes, traffic, schedule, settings,
admin moderation of users/plans/orders.

| 子项 | 状态 | 离 100% 差什么 |
|---|---|---|
| 节点 CRUD（添加/编辑/启停/删除） | ✅ | — |
| 节点健康探测（30s 周期） | ✅ | — |
| 节点 CPU/Mem 时序 | ⚠️ | 后端 API 在，前端图表没画 |
| 节点延迟 / Xray 版本上报 | ✅ | — |
| 入站管理（5-tab 编辑器，8 transport × 3 security） | ✅ | 我们比 sspanel 更细 |
| 客户端 CRUD + 流量重置 | ✅ | — |
| 用户列表/编辑/封停/调余额 | ⚠️ | 后端 API 全在，前端 0 行 |
| 套餐管理（admin 端） | ⚠️ | 后端在，前端 0 行 |
| 订单管理（admin 端） | ⚠️ | 后端在，前端 0 行 |
| 运行时设置（功能开关） | ✅ | — |
| Cron：节点探测 | ✅ | — |
| Cron：流量采集 | ✅ | — |
| Cron：Webhook 持久化重试 | ✅ | — |
| Cron：每日/每月流量重置 | ❌ | 全栈缺 |
| Cron：到期处理 + 过期提醒 | ❌ | 事件常量定义了，没人 publish |
| Cron：自动续费扣款 | ❌ | 全栈缺 |
| 数据库迁移（启动时自动） | ✅ | — |

**到 100% 缺**：3 个 admin 页面（用户/套餐/订单）+ 3 个 cron（流量重置 / 到期 / 自动续费）+ 1 个 UI 图表（节点时序）。

---

## 2️⃣ 多协议支持 | Multi-Protocol Support — **67%**

节点能跑哪些协议（**由 3x-ui 上游 capability 决定**）+ 订阅能分发哪些格式。

| 子项 | 状态 | 离 100% 差什么 |
|---|---|---|
| **节点侧协议（3x-ui 实际 surface）** | | |
| VLESS（含 Reality / XTLS-Vision） | ✅ | — |
| VMess | ✅ | — |
| Trojan | ✅ | — |
| Shadowsocks（含 2022 ciphers） | ✅ | — |
| **WireGuard**（3x-ui v2.3+ 支持） | ❌ | runtime/links/订阅渲染都没接，受众较小但 3x-ui 原生有 |
| ~~Hysteria2 / TUIC v5 / AnyTLS~~ | — | **不在 scope**：Xray-core 不支持，3x-ui 也不提供 |
| **订阅分发格式** | | |
| Base64 链接束 | ✅ | `Assembler.FormatBase64` 已实现 |
| Xray JSON config | ✅ | `Assembler.FormatJSON` 已实现 |
| Clash YAML（含 proxy-groups + rules + DNS） | ❌ | **完全没实现**（之前误标为"基础版"——实际无 `FormatClash` 方法）。🚧 `add-subscription-converter` |
| Sing-box JSON | ❌ | 🚧 `add-subscription-converter` |
| SIP008（Shadowsocks 标准订阅） | ❌ | 🚧 同上 |
| User-Agent 自动选格式 | ❌ | 🚧 同上 |
| Token 化的公开订阅 URL（无需登录） | ✅ | — |
| `Subscription-Userinfo` header（用量/到期） | ✅ | — |
| 多客户端聚合到一个订阅 ID（一个用户跨多节点） | ✅ | — |

**到 100% 缺**：
- 节点侧：WireGuard runtime + links + 订阅渲染（低优先级）
- 订阅格式：完整 Clash 模板 + sing-box + SIP008 + UA 自动选（高优先级，1 个 change 收口）

---

## 3️⃣ 支付系统 | Payment System — **20%**

| 子项 | 状态 | 离 100% 差什么 |
|---|---|---|
| 套餐定义（价格/流量/时长/IP 限制/enable） | ✅ | — |
| 用户余额 + 余额变动审计（balance_logs） | ✅ | — |
| 订单生命周期模型（created/paid/completed/failed） | ✅ | — |
| 幂等购买（idempotency_key + 行锁 + provisioning failure refund） | ✅ | 我们写得严谨 |
| 订单历史（admin 端） | ⚠️ | 后端 API 在，前端 0 行 |
| 支付宝当面付 | ❌ | 0 行代码 |
| Stripe | ❌ | 0 行代码 |
| PayPal | ❌ | 0 行代码 |
| Cryptomus（加密币） | ❌ | 0 行代码 |
| 支付回调 webhook 接收 + 验签 + 订单状态推进 | ❌ | 依赖前面任一网关 |
| 计费模式（包年/包月/按量/access-type） | ⚠️ | 当前只有"一次性扣余额"一种 |
| 退款（管理员手动触发） | ⚠️ | 自动 refund-on-failure 有，admin 手动触发 API 没 |

**到 100% 缺**：至少 2 个真实网关（推荐先做支付宝 + Stripe）+ 回调验签 + 多种计费模式 + 手动退款 API。

---

## 4️⃣ 通知系统 | Notification System — **40%**

| 子项 | 状态 | 离 100% 差什么 |
|---|---|---|
| 内部 event-bus（同步 pub/sub） | ✅ | — |
| 事件分类（node.*、user.registered、order.*、client.*） | ✅ | 常量定义齐 |
| 通用 Webhook 出站（HMAC 签名 + 持久化重试 + replay） | ✅ | 我们写得比较严谨 |
| SMTP 邮件发送（STARTTLS / 隐 TLS / 中文 subject） | ✅ | — |
| 邮件验证码（注册） | ✅ | — |
| 多 SMTP 切换 + 失败 fallback | ❌ | 单 SMTP 配置 |
| 异步邮件队列（数据库持久化 + retry） | ❌ | 当前同步发送 |
| Telegram bot 通知 + 命令 | ❌ | 0 行代码 |
| Discord webhook 适配（含 embed 模板） | ⚠️ | 通用 webhook 能转发但无 Discord embed 格式化 |
| Slack 适配（含 block-kit 模板） | ❌ | 0 行代码 |
| 到期提醒 / 流量阈值事件触发器（cron 里 publish） | ⚠️ | 事件常量定义了，但没人在 cron 里 publish |

**到 100% 缺**：Telegram bot 通道 + Discord/Slack 模板 + 异步邮件队列 + 把 cron 里到期/阈值的 publish 接上。

---

## 5️⃣ 用户界面 | User Interface — **50%**

`admin console` + `portal` + 底层设计系统。

### 5a. Admin Console（运营用）

| 页面 | 状态 | 离 100% 差什么 |
|---|---|---|
| 系统状态（KPI + 节点健康表） | ✅ | — |
| 节点管理 | ✅ | — |
| 入站管理（5-tab 编辑器） | ✅ | 我们独有，sspanel 没这粒度 |
| 面板设置 | ✅ | — |
| 用户管理 | ❌ | 后端 API 全在 |
| 套餐管理 | ❌ | 后端 API 全在 |
| 订单管理 | ❌ | 后端 API 全在 |
| 统计页（活跃用户/收入趋势/图表） | ❌ | 全栈缺 |

### 5b. Portal（终端用户用）

| 页面 | 状态 | 离 100% 差什么 |
|---|---|---|
| 仪表盘（流量图 + 套餐状态） | ⚠️ | 94 行 stub，仅文字 |
| 订阅页（URL + QR + 多格式切换 + 复制按钮） | ❌ | 后端订阅 endpoint 在，前端 0 行 |
| 套餐对比 + 购买 | ❌ | 后端 plan/purchase API 在 |
| 订单历史 + 余额显示 | ❌ | 后端 order API 在 |
| 资料 / 改密 / 绑邮箱 | ❌ | 后端 profile/change-pw/bind-email API 在 |

### 5c. 设计系统 + 通用 UI

| 子项 | 状态 | 备注 |
|---|---|---|
| 设计 token（字体/颜色/圆角/阴影/动效） | ✅ | 完整 |
| Light/Dark 主题（OS 跟随 + localStorage） | ✅ | — |
| 三种 Layout（Auth / Admin / Portal） | ✅ | — |
| 统一 `/login`（admin/user 自动判定 + 注册 tab + OIDC 占位） | ✅ | 我们超出 sspanel |
| i18n（en / zh） | ✅ | — |
| 移动端响应式 | ❌ | admin/portal 都是 desktop-first |

**到 100% 缺**：4 个 admin 页（用户/套餐/订单/统计）+ 5 个 portal 页（仪表完整版/订阅/套餐/订单/资料）+ 移动端响应式。

---

## 速查：5 维度离 100% 还差的全部清单

按维度横向汇总，每一项链到下方 Change 队列里的承接编号。

| 维度 | 缺口 | 由哪个 change 承接 |
|---|---|---|
| **1. 运维** | (a) Admin UI 4 页：用户管理 / 套餐管理 / 订单管理 / 统计页 | #3 `add-admin-business-views` |
| | (b) Cron 3 任务：每日/每月流量重置 / 到期处理 + 过期提醒 / 自动续费扣款 | #4 `add-billing-cron-jobs` |
| | (c) 节点 CPU/Mem 时序图表（后端有 API） | #3 `add-admin-business-views`（顺带）|
| **2. 多协议** | (a) Clash 完整模板（proxy-groups + rules + rule-providers + DNS） | #1 `add-subscription-converter` |
| | (b) Sing-box JSON 输出 | #1 |
| | (c) SIP008 输出 | #1 |
| | (d) User-Agent 自动选格式 | #1 |
| | (e) WireGuard 节点侧 runtime + links + 订阅渲染 | #8 `add-protocol-wireguard`（低优先级） |
| **3. 支付** | (a) 支付宝当面付（含回调验签） | #5 `add-payment-alipay` |
| | (b) Stripe（Checkout + Webhook） | #6 `add-payment-stripe` |
| | (c) PayPal | 未排期（市场需求低于前两个） |
| | (d) Cryptomus（加密币） | 未排期 |
| | (e) 计费模式扩展：包月/包年/按量/access-type | #5/#6 同期，作为 plan model 扩展 |
| | (f) 退款（admin 手动触发 API） | #3 顺带（admin 订单管理页里加按钮） |
| **4. 通知** | (a) Telegram bot 通道（推送 + 命令交互） | #7 `add-notification-channels` |
| | (b) Discord webhook 适配（含 embed 格式化模板） | #7 |
| | (c) Slack 适配（含 block-kit 模板） | #7 |
| | (d) 异步邮件队列（DB 持久化 + retry） | #7 |
| | (e) 多 SMTP 切换 + 失败 fallback | #7 |
| | (f) 在 cron 里挂到期/流量阈值的事件 publisher | #4（cron 实做时一并） |
| **5. 用户界面** | (a) Portal 4 页：订阅 / 套餐 / 订单 / 资料 | #2 `add-portal-views` |
| | (b) Portal 仪表盘扩充（流量图表替换 stub） | #2 |
| | (c) Admin 4 页：用户 / 套餐 / 订单 / 统计 | #3 `add-admin-business-views` |
| | (d) 移动端响应式（admin + portal） | #9 `add-mobile-responsive` |

合计 25 项明确缺口，由 9 个 change 关联承接。

---

## Change 队列（按上线价值排序）

落地节奏：每个 change `~2-5 天`。状态用上面的图例。

| # | Change | 主要影响维度 | 预期推进 | 状态 |
|---|---|---|---|---|
| 1 | `add-subscription-converter` | 多协议 | 45% → 80% | 🚧 proposal+design 在写 |
| 2 | `add-portal-views` | 用户界面 | 50% → 75% | ❌ 未开 |
| 3 | `add-admin-business-views` | 用户界面 + 运维 | UI 75% → 90% / 运维 60% → 75% | ❌ 未开 |
| 4 | `add-billing-cron-jobs` | 运维 | 75% → 90% | ❌ 未开 |
| 5 | `add-payment-alipay` | 支付 | 20% → 45% | ❌ 未开 |
| 6 | `add-payment-stripe` | 支付 | 45% → 60% | ❌ 未开 |
| 7 | `add-notification-channels` | 通知 | 40% → 80% | ❌ 未开 |
| 8 | `add-protocol-wireguard` | 多协议 | 节点 4/5 → 5/5（WireGuard runtime/links/sub） | ❌ 未开（低优先级） |
| 9 | `add-mobile-responsive` | 用户界面 | 90% → 95% | ❌ 未开 |

做完 1-9 → 5 维度都 ≥ 80%，综合 ~85%，可以真上线给真用户。

---

## 已完成的 change（参考）

| Change | 时间 | 影响 |
|---|---|---|
| `bootstrap-central-panel` | 2026-05-18 | 全部 9 个 v1 模块的初次落地 |
| `add-email-verification-and-oidc-hook` | 2026-05-20 | 邮箱验证码 + OIDC providers stub + ADMIN_PASSWORD 自动生成 |

---

## 一步步推进的方式

每次我们决定开一个 change，流程：

1. 在这里查表，挑下一项（按优先级或当下重点）
2. 在 `openspec/changes/<name>/` 用 `/openspec-propose` 风格 scaffold proposal + design + tasks + spec deltas
3. 实现 + 测试 + 部署
4. 回到这里：把这一项的 ❌/⚠️ → ✅，更新维度百分比、综合百分比、整体进度条
5. 进度条 ≥ 80% 之前不停

> **当前焦点**：`add-subscription-converter`（# 1）— proposal.md + design.md 已就位，tasks.md + spec deltas 待写。
