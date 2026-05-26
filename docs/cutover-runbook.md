# Cutover Runbook — Vue → React/AntD

> **历史档案** — cutover **已于 2026-05-26 执行完成**（commits
> `65c377b` "replace Vue frontend with React" + `26fea5f` "point
> tooling at React frontend"）。下面的 T-24h / T-0 / T+1h 清单
> 是当时跑过的操作手册，保留作为参考。**不要再按这份 runbook
> 执行**——`frontend-react/` 目录已经不存在，`frontend/` 现在就
> 是 React 树。

操作手册。cutover 是 OpenSpec change `rewrite-frontend-react-antd` 的 P7 里程碑，一次性把 `frontend/` (Vue) 换成 `frontend-react/` (React)。这份文档列出 T-24h、T-0、T+1h 三个时间点要做的事。

执行者：项目维护者本人。前提：P0–P6 全部完成、`openspec validate rewrite-frontend-react-antd` 通过、`frontend-react/` 下 `npm run typecheck && npm run lint && npm run test && npm run e2e` 全绿。

---

## T-24 小时：准备

这一阶段没有不可逆操作。如果发现任何阻塞项，cutover 推迟一天再来一次 T-24h 清单。

### 代码与构建

- [ ] `git status` 干净，无未提交改动
- [ ] `main` 分支跟远程同步：`git pull --ff-only origin main`
- [ ] 当前提交跑通 `make build`（Vue 版本构建 + Go binary 编译）
- [ ] 当前 commit 的 SHA 记下来，写进 cutover 提交信息里作为回滚锚点
- [ ] `cd frontend-react && npm run typecheck && npm run lint && npm run test` 全绿
- [ ] `cd frontend-react && npm run e2e` 全绿（需要 backend 跑起来，端口 8080）
- [ ] `node frontend-react/scripts/check-locale-parity.mjs` 退出码 0
- [ ] `node frontend-react/scripts/check-spec-parity.mjs` 退出码 0
- [ ] `openspec validate rewrite-frontend-react-antd --type change` 通过

### 数据与配置

- [ ] PostgreSQL 数据库快照：`pg_dump` 出一份当前 schema + 数据，存到 `backups/cutover-YYYYMMDD.sql.gz`
- [ ] `.env` 备份到 `backups/cutover-YYYYMMDD.env`
- [ ] 当前 binary 备份：`cp 3xui-dashboard backups/cutover-YYYYMMDD-pre.bin`
- [ ] 列出当前所有节点：`curl -H 'Authorization: Bearer $ADMIN_JWT' http://localhost:8080/api/admin/nodes | jq '.[] | {id, host, status}' > backups/cutover-YYYYMMDD-nodes.json`
- [ ] 列出当前所有用户：导出 `users` 表行数到 `backups/cutover-YYYYMMDD-user-count.txt`（cutover 后做完整性对账用）

### 人 / 流程

- [ ] 通知所有内部用户："明天 X 点门户会有 ~10 分钟维护窗口，期间订阅 URL 仍可用，控制面短暂不可达"
- [ ] 跟自己确认：cutover 当晚至少留 2 小时不要做别的事
- [ ] 准备好回滚预案的命令（见下面 "回滚" 一节），打开记下来

---

## T-0：cutover

总耗时目标 ~10 分钟。

### 1. 最终确认（2 分钟）

- [ ] 没有任何 Vue 树的未合并 PR 在 `main` 上飘着
- [ ] `git pull --ff-only origin main` 再拉一次
- [ ] backend 当前在跑，验证 `curl http://localhost:8080/api/public/branding` 返回 2xx

### 2. 切换 commit（3 分钟）

```bash
# 切到新分支做 cutover
git checkout -b cutover-react-antd

# 第一个 commit：纯机械的删 + 重命名
rm -rf frontend
mv frontend-react frontend
git add -A
git commit -m "🔥 cutover: Vue → React/AntD

Replace frontend/ (Vue 3 + Pinia + Tailwind) with the React tree
prepared as frontend-react/. Pre-cutover anchor commit: <SHA>.

See docs/frontend-rewrite.md and the OpenSpec change
rewrite-frontend-react-antd for context."

# 第二个 commit：Makefile / README / docker 路径修复
# 编辑：
#   Makefile         — 删 dev-frontend-react / build-frontend-react 目标
#   README.md        — 任何 Vue / Pinia / Tailwind 描述改成 React / Zustand / AntD
#   deploy/*         — build 脚本里如果有 frontend-react 路径，改回 frontend
git add Makefile README.md deploy/
git commit -m "chore(cutover): point Makefile/README/deploy at the renamed frontend tree"
```

### 3. 构建并替换 binary（4 分钟）

```bash
# 全量重新构建
make clean 2>/dev/null || true
make build

# 启新 binary 之前 backup 旧的
mv 3xui-dashboard backups/cutover-YYYYMMDD-pre.bin 2>/dev/null || true

# 重启服务（systemd / docker / 直接进程，按部署方式选一种）
systemctl restart 3xui-dashboard
# 或
docker compose restart dashboard
# 或
pkill 3xui-dashboard && ./3xui-dashboard -env deploy/.env &
```

### 4. 即时冒烟（1 分钟）

- [ ] 浏览器开 `http://<dashboard-host>/` — 落到 React 版的 Login 页面
- [ ] 输入 admin 凭据登录 — 落到 `/admin/status`（React 版 Overview）
- [ ] AdminLayout chrome 正常显示（sider + 顶栏）
- [ ] 点开两个 tab 看数据能加载

如果上面任意一项失败，**立刻回滚**（见下面）。不要尝试 hotfix。

---

## T+1 小时：稳定性验证

不停手，逐项过：

- [ ] 登录态：admin + portal 各开一个浏览器窗口，confirm 都能登录
- [ ] 数据一致：节点数 / 用户数对照 T-24 备份的数字，无偏差
- [ ] 关键写入路径各做一次：创建一个测试 plan、改一次节点配置、删一个测试用户、发一个测试订阅
- [ ] AlipayPayModal：模拟一次小额下单（如果 Alipay sandbox 可用），看 QR 显示 + 3 秒轮询是否工作
- [ ] OIDC 登录：用一个测试公司账号走一遍 SSO，确认 emailConflict / 正常登录两条路径
- [ ] Mobile 适配：手机访问 `/portal/subscription`，确认底部 nav 出现、订阅 QR 可见
- [ ] `make test` 全过（Go 后端单测）
- [ ] backend 日志这一小时无新增 ERROR

如果到此为止一切正常：

- [ ] `git push origin cutover-react-antd:main` （或者合 PR 到 main）
- [ ] 通知用户："cutover 完成，新版上线"
- [ ] 把这份 runbook 复制一份到 `backups/cutover-YYYYMMDD-runbook.md` 留档

---

## 回滚预案

cutover 后 24 小时内随时可以回滚。代价：操作员看到 UI 变回 Vue 版，没数据损失。

### 紧急回滚（< 5 分钟）

```bash
# 选项 A: 用备份的 binary 直接换回去
cp backups/cutover-YYYYMMDD-pre.bin 3xui-dashboard
systemctl restart 3xui-dashboard
```

```bash
# 选项 B: revert git 提交 + 重新构建（更慢但更干净）
git revert --no-edit HEAD~1 HEAD   # revert 两个 cutover commit
git push origin main
make build
systemctl restart 3xui-dashboard
```

选项 A 快但 git 状态会跟运行的 binary 不一致；选项 B 慢但状态干净。线上严重故障用 A，普通问题用 B。

### 回滚后

- [ ] 确认 Vue 版本恢复正常（登录、看节点、看 plan）
- [ ] 写一份 incident note 记录 cutover 失败的原因
- [ ] 修完问题后重新走 T-24h 清单，下次再 cutover

### 不能回滚的情况

数据库 schema 没动过（design D2 / cutover spec 都保证了），所以只要 binary + 代码能回滚，数据就回得去。

唯一可能的不可逆点是 cutover commit push 到 main 之后，如果有人在这个 commit 之上又提交了别的东西。所以 cutover commit 是 **"push 之后立刻进 T+1 小时验证"**，验证期间不允许其他 PR 合并到 main。

---

## 不要做的事

- 不要在 cutover 当天合任何其他 PR
- 不要在 cutover commit 之前做 `git rebase -i` 或 squash（保留 cutover commit 作为单独的可识别点）
- 不要把 cutover commit 跟其他改动合并到一个 commit 里（review 困难、回滚也困难）
- 不要在没跑 T-24h 清单的情况下尝试 cutover（"应该没问题" 是失败的前兆）
- 不要修改 backend 任何代码作为 cutover 的一部分（backend 零改动是 cutover 的核心保证）
