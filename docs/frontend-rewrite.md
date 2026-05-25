# 前端 React + AntD 重构计划

## 为什么要重构

当前前端是 Vue 3 + Pinia + vue-router + 自研 Tailwind 设计系统，~18.8K 行代码、23 个 view（含 OpsMonitor 和 settings 子目录）、9 个共享组件。两个结构性问题逼着换栈：

**组件漂移**。同样语义的 widget（刷新按钮、page header、状态徽章、KPI 卡片）在每个 view 里被重新写一遍，Tailwind class 串各不相同。`refactor-admin-portal-ui-primitives` 这个 OpenSpec 已经尝试抽过一轮 primitives，但每加一个 admin 页面都还在长出新的一次性副本。用户在 2026-05-24 Status+Stats merge 时明确点名这是质量问题。

**表单 / 表格效率低**。4 个重量级 admin view（Settings 1565、Users 1300、Inbounds 1282、InboundEditorModal 1178）加上 OpsMonitor（658）占了约 63% 的代码量，几乎全是手写表单 + 表格 + inline SVG 图表。AntD 的 Form / Table / Modal / Drawer 能把这些代码量砍掉 3–5 倍，同时把分页、选择、验证、i18n 数字日期格式这些事统一交给库。

还有一个外部条件：**窗口期**。项目还是 pre-launch greenfield，没有真实用户，没有生产部署。这是做 wholesale 平台换栈成本最低的时刻。launch 之后，操作员会被训练在某一种设计语言上，再做这种切换的成本是 5 倍以上。

## 新栈

| 角色 | 选什么 | 为什么 |
|---|---|---|
| UI 框架 | React 18 + TypeScript | 招聘市场最大池子 |
| 组件库 | Ant Design 5 | admin panel 类 UI 的最强候选，token 体系完整 |
| 路由 | React Router v6 | 成熟，跟 AntD 没冲突 |
| 客户端状态 | Zustand | 跟 Pinia 一对一翻译最直接 |
| 服务端状态 | TanStack Query v5 | 缓存 / 重试 / 失效 / 乐观更新都白送 |
| i18n | react-i18next + i18next | vue-i18n 的 `{var}` 插值语法兼容 |
| HTTP | axios (不变) | 21 个 api 模块零改动 |
| 日期 | dayjs | AntD DatePicker 的默认适配 |
| 构建 | Vite + @vitejs/plugin-react | Vite 沿用 |
| 单测 | Vitest + @testing-library/react | Vitest 沿用，RTL 是 React 圈的 @vue/test-utils |
| E2E | Playwright (不变) | 只改 selector |

被淘汰的：vue、vue-router、vue-i18n、pinia、@vueuse/core、@vitejs/plugin-vue、@vue/test-utils、vue-tsc、eslint-plugin-vue。

## 迁移策略

并行建 `frontend-react/`，做完一刀切。

不走 file-by-file in-place 替换（vue + react 双 Vite 插件会打架，eslint 两套也吵架）；不走"先删 Vue 再建 React"（中途没可运行版本，全压在最后联调）。Vue 树在重构窗口期保持可运行，可以继续修 bug；新代码全在 `frontend-react/` 里。

cutover 是一次性的：`rm -rf frontend && mv frontend-react frontend`，加一个 sweep commit 更新 Makefile / README / docker build 路径。后端的 `go:embed dist` 一行代码都不改——`backend/internal/web/dist/` 的消费目录契约保持。

cutover 前的回滚成本：`rm -rf frontend-react`。
cutover 后的回滚成本：单 commit `git revert`。

## 八个里程碑

按里程碑级 spec 写死了 Entry / Exit criteria，每个独立验收。

**P0 — Scaffold（~1 天）**
建 `frontend-react/`，装依赖，配 vite / tsconfig / eslint，AntD ConfigProvider 主题从现有 tailwind tokens 移植，跑通 hello world。dev port 5174 避开 Vue 的 5173。

**P1 — 横切层（~2 天）**
21 个 axios API 模块原样搬；每个 endpoint 包 TanStack Query hook；i18n locale 文件 1:1 移植；5 个 Pinia 改 Zustand（adminAuth / portalAuth / app / theme + branding 改成 `useBranding()` Query hook）；共享组件映射到 AntD（EmptyState / Skeleton / AccountMenu / LocaleSwitcher / PageHeader / RefreshButton；ConfirmModal + useConfirm 不移植，4 个 callsite 改用 `Modal.confirm`）。

**P2 — Layout + 路由（~1 天）**
3 个 layout 用 AntD `<Layout>` + `<Sider>` + `<Header>` 重做，React Router v6 路由表镜像现有 router/index.ts，auth 守卫做成 `<ProtectedRoute area="admin"|"portal">` HOC。

**P3 — 认证面（~1 天）**
Login（含失败 cooldownTimer）、OIDCCallback、NotFound。`make dev-frontend-react` 端到端跑通：开 `/login` → 输入 admin 凭证 → 落到 `/admin/status`。

**P4 — Admin 视图（~5–7 天）**
按 tier 拆：
- 轻量（每个 1 天）：Plans 360、Orders 232、AuditLog 227（含 300ms 搜索防抖）、ProvisioningPools 454
- 中量（每个 1.5 天）：Overview（Status+Stats tabs，KeepAlive → mounted Set + display:none）、Nodes 543、Webhooks 504、OpsMonitor 658（含 inline SVG 图表组件 DonutGauge / TrendLine / BarsPanel / DotsGrid）
- 重量（每个 2–3 天）：Users 1300（含 autoRefreshTimer 通过 `refetchInterval` 翻译）、Inbounds 1282 + InboundEditor 1178（6 个协议拆子文件：vless / vmess / trojan / shadowsocks / hysteria / wireguard）、Settings 1565（7 tab + 已有 DataCollectionSettings 子目录沿用）

**P5 — Portal 视图（~3 天）**
Subscription（QR + 一键复制）、Usage、Plans（含购买流）、Orders、Profile（邮箱 / 密码 / OIDC 解绑）、AlipayPayModal（3 秒轮询，不是 5 秒）。

**P6 — 测试（~2 天）**
24 个 Vue spec 用 RTL 重写（useConfirm.spec 不需要对应，已写进 parity 脚本的 exclusion list）；shared `renderWithProviders` 测试 helper；e2e selectors 更新；locale-parity 脚本入 CI。

**P7 — 切换（~0.5 天）**
单 commit 删 Vue 树 + rename；sweep commit 改 Makefile / README / docker；归档 OpenSpec change。

单人工期 16–22 天；P4/P5 双线并行可压到 ~12 天。原型一周搓完已经证明工程难度可控，所以这个估值是上限不是均值。

## 几个非显然的设计选择

设计意图里有些东西光看代码看不出来，挑几个写下来：

**服务端状态全走 TanStack Query**。所有 HTTP 调用包 `useQuery` / `useMutation`，禁止 `useEffect + axios + useState` 模式。`queryKey` 用 `[area, resource, op, ...args]` 三段约定（area ∈ admin / portal / public），mutation 的 `onSuccess` 用 prefix 失效。这条规则一旦松了，缓存模型立刻坏。

**客户端状态全走 Zustand，localStorage key 跟 Vue 一致**。`3xui.adminAuth` / `3xui.portalAuth` 等持久化 key 跟 Vue 树一模一样——cutover 那一刻已登录用户不需要重新登录，操作员零打扰。

**AntD 主题从现有 brand palette 派生**。`colorPrimary` = indigo `#4f46e5`（Vue 树的 primary-600），`colorSuccess` = teal `#0d9488`（accent-600）。primary 按钮看起来还是公司品牌色，不是 AntD 默认的蓝。Geist Sans 字体保留，已经在 `@fontsource/geist-sans` 里。

**i18n key 1:1 保留**。所有 `admin.*` `portal.*` `nav.*` 等 key 一个不动从 zh.ts/en.ts 搬过去；react-i18next 配 `returnNull: false` + `keySeparator: '.'`，missing key 在 dev 模式直接报错。CI 加 locale-parity 脚本守门，未来任何 PR 改 key 都会被拦。

**图表保持 inline SVG，不引图表库**。OpsMonitor 当前用 inline SVG 画的圆环 / 折线 / 柱状 / 点阵继续这个路子，每种做成 30–80 LOC 的展示组件放 `src/components/charts/`。`@ant-design/charts` 加 ~150KB 还带第二套主题系统，目前 4 种图表只在一个 view 用，不值。如果以后做交互式图表（缩放、tooltip、多 series）再切。

**KeepAlive 用 mounted Set + display:none 模拟，Transition 用 AntD 内置 motion**。不引 react-keep-alive（跟 React 18 strict mode 不合）、不引 framer-motion（4 个 fade in/out 用不上 50KB）。

**Vue 树在重构窗口期 freeze**。bugfix only，新功能要么进 React 要么 PR 里双写。靠社交约束执行——一个 author、一个 main 分支、写在 CLAUDE.md 里。这条破了，cutover 时差会一直累积，cutover 日期就一直推。

**移动端响应式不能丢**。Vue 树有真实的 mobile chrome：AdminLayout 在 `md` 以下用 hamburger + drawer，PortalLayout 在 `lg` 以下用底部 nav，多个 list view 在 mobile 切换 card / table 视图。React 这边对应的做法是：AdminLayout 用 AntD `<Drawer>`，PortalLayout 加 fixed bottom `<Menu mode="horizontal">`，list view 全部走一个共享的 `<ResponsiveListTable>` 包装器（自动按 `MD_BREAKPOINT` 在 `<Table>` 和 `<List>` 之间切）。breakpoint 数值定在 `src/theme.ts` 里，不在视图里硬编码 768/1024。

**Tailwind 在 cutover 时直接删**。既然选了"全 AntD 设计语言"，layout 完全交给 `<Flex>` / `<Space>` / `<Row>` / `<Col>`，Tailwind 留着只是惯性。省 ~12KB gzip + 一个 config 文件 + 不会有"两套样式系统"的认知负担。

**OIDCCallback 必须区分 7 种 backend 错误，不能合并成"登录失败"**。后端 2026-05-21 的 `fb353a1` 修了几个静默失败路径，现在 `/api/user/auth/oidc/callback` 会返回 `ErrOIDCEmailConflict` / `ErrOIDCEmailMismatch` / `ErrOIDCStateInvalid` / `ErrDomainNotAllowed` / `ErrUserSuspended` / `ErrOIDCEmailRequired` / `ErrNotImplemented` 等。React 这边要按状态码 + body substring 分流，每种错误给针对性的提示和恢复动作（比如"邮箱已绑定 → 提示先登录那个账户再 Profile 里关联"，"state 失效 → 提供重新发起登录的按钮"）。

**Subscription 页面有 7 种订阅格式**（base64 / clash / singbox / sip008 / wireguard / wireguard-zip / json），不是单一 URL。base64 是默认，URL 不带 query；其它带 `?format=X`。`wireguard-zip` 是 downloadOnly，得隐藏 copy 和 QR 改成 "下载 ZIP" 按钮。QR 重生成要 monotonic token 防快速切格式时旧请求覆盖新结果。这套逻辑 Vue 树已经有，移植时不能简化掉。

**Settings 是数据驱动表单，不是手写字段**。每个 tab 从 `/api/admin/settings` 拿 `SettingItem[]` 数组，用一个共享 `<SettingRow>` 组件渲染。drafts 缓冲未保存值，save / clear 是行级操作。这个约束很重要——意味着后端加一个新的可配置项（在 `app_settings` 加一行），UI 自动多一个字段，不用动 React 代码。

**a11y 和性能立硬指标，不靠"感觉"**。a11y 走 `@axe-core/playwright` 跑 e2e smoke 的几个页面，serious / critical 违反直接 fail CI；性能立 4 个 Lighthouse 指标（TTI < 1.5s、LCP < 2.5s on Mid-Tier-Mobile、初始 JS gzip < 500KB、关键路径 < 200KB），回归超过 10% 要 PR 显式承认。这两个东西如果不在 CI 里守门，永远会在 deadline 压力下第一个被牺牲。

## 风险

**AntD 视觉跟现有 Tailwind 风格差异明显**。锐角更多、间距更紧、focus ring 不一样。cutover 那一刻所有人会感觉"画风变了"。这是定方向的时候已经同意的——选的是"全 AntD 设计语言"而不是"AntD 组件 + Tailwind 视觉"。brand 色保留能减轻一些这种感觉，但视觉变化是真的、不可避免。

**TanStack Query 默认缓存语义跟 Vue 树的 `onMounted(reload)` 不一样**。配错了会出现"我刚加了一个 plan，列表里没看到"的体验。每个 mutation 必须显式 invalidate prefix，每个重量级 view（Users / Inbounds / Settings）P4 时单独审一遍 cache 路径。

**4 个重量级 view 的表单验证容易丢**。手写验证器迁到 AntD Form rules 的过程中，密码复杂度、端口范围、邮箱格式、节点 host 格式这些容易漏掉一条。每个验证器都要有反向测试（输入非法值断言报错），不只测正向。

**cutover 单 commit 改动巨大**。把 cutover 拆成两个 commit：第一个是 `rm -rf frontend && mv frontend-react frontend`（纯机械），第二个才是 Makefile / README / docker 修复（人工 review）。第一个 commit 不需要细读 diff。

## Open questions

**e2e selector 策略**。`data-testid` 加在 React 组件上（parity 视角友好）还是用 AntD role-based 语义 selector（更稳但要重写）。第一版先 `data-testid` 求 parity，cleanup 留给后面。

之前两个 OQ 已经 resolve 掉了：Tailwind 留删 → cutover 时直接删（见上面"部署上刻意做得简单"那段相关讨论）；Stripe 时序 → 已经在 Vue 树里完工（`portal/Plans.vue` 已经在调 `purchaseViaPayment('stripe', ...)`），P5 直接 1:1 移植即可。

## 现在到哪了

立项完毕，未实施。

OpenSpec change `rewrite-frontend-react-antd` 在 `openspec/changes/` 下，2571 行：

- 1 个 `proposal.md`（197 行，列出 1 个 platform capability + 8 个里程碑 capability）
- 1 个 `design.md`（493 行，13 条决策 D1–D13）
- 1 个平台 spec `frontend-platform-react/spec.md`（272 行，cutover 后留作长期契约）
- 8 个里程碑 spec `frontend-rewrite-p0-scaffold` … `p7-cutover`（1465 行）
- 1 个 `tasks.md`（131 行，75 个可勾选 task）
- 总共 50 个 Requirement、~150 个 WHEN/THEN 可观测场景

`openspec validate rewrite-frontend-react-antd` 通过。

下一步是 `/opsx:apply` 或者手动按 P0 task 开始。前面起过一次 P0 但被中断回滚了；`frontend-react/` 目录现在不存在。
