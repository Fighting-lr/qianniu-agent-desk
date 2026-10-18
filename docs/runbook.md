# AgentDesk · 运行手册

> L'Oréal 2nd Beauty Tech Hackathon · 赛题1「数据共情者——消费者的AI管家」
> 团队：Star of CAU（中国农业大学）

本文档是**唯一**的操作入口。按顺序执行即可从零跑通整套 Demo。

---

## 0. 前置环境

| 组件 | 版本 | 说明 |
|---|---|---|
| Docker Desktop | 任意近期版本 | 跑 MySQL / Qdrant / agent-desk |
| Python | 3.13 | **用隔离 venv**，已装 httpx + openpyxl |
| Node.js | 22+ | 已内置在 agent-desk 的 web 目录 |
| Go | 1.26 | 仅在不用 Docker、本地跑后端时需要 |

本项目用到的解释器路径（本机实测）：

```
Python  : C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe
Go      : C:/Users/ACER/.workbuddy-ai/binaries/go/v126/go/bin/go.exe
Node    : C:/Users/ACER/.workbuddy-ai/binaries/node/versions/22.22.2-2/node.exe
```

> 后续命令中的 `python` 均指上面这个 venv 解释器。
> 如未创建，先执行：
> ```bash
> C:/Users/ACER/.workbuddy-ai/binaries/python/versions/3.13.12/python.exe \
>     -m venv C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default
> C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe \
>     -m pip install httpx openpyxl
> ```

---

## 0.4 国内镜像源（★ 首次搭建必配）

四个包管理器**全部走国内镜像**。其中 **Go 的那一条最关键** —— 不配会导致编译奇慢。

### ⚠️ 最容易踩的坑：GOSUMDB 默认指向墙外

Go 下载每个模块后会向 `sum.golang.org` 校验哈希。**该域名在国内不可达**（实测 8 秒超时），
于是**每个模块都要白等一次超时**，`go build` 看起来像卡死。实测对比：

| 校验服务器 | 结果 |
|---|---|
| `sum.golang.org`（默认） | ❌ **不可达**，8s 超时 |
| `sum.golang.google.cn` | ✅ 200，1.3s |

> 这正是之前 Go 编译跑了 40+ 分钟的直接原因 —— **不是网络慢，是每个模块都在等校验超时**。

### 一键配置

```bash
# ---- 1. Go ----
# 注意：go env -w 需要 %AppData%，sandbox/精简 shell 里可能未导出
export APPDATA="${APPDATA:-C:\\Users\\ACER\\AppData\\Roaming}"
GO="C:/Users/ACER/.workbuddy-ai/binaries/go/v126/go/bin/go.exe"

"$GO" env -w GOPROXY="https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct"
"$GO" env -w GOSUMDB="sum.golang.google.cn"     # ★ 关键
"$GO" env -w GOTOOLCHAIN="local"                # 禁止自动联网拉其它 Go 版本

# ---- 2. npm / pnpm ----
# 用户级已配；项目级 .npmrc 已提交（agent-desk/web/.npmrc），clone 即可用
npm config set registry https://registry.npmmirror.com

# ---- 3. pip ----
PY="C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe"
"$PY" -m pip config set global.index-url "https://pypi.tuna.tsinghua.edu.cn/simple"
"$PY" -m pip config set global.extra-index-url "https://mirrors.ustc.edu.cn/pypi/web/simple"
"$PY" -m pip config set global.timeout "60"
"$PY" -m pip config set global.retries "5"
```

### 生效位置（换机器时对照）

| 工具 | 配置文件 | 关键值 |
|---|---|---|
| Go | `%AppData%\go\env` | `GOPROXY` / `GOSUMDB=sum.golang.google.cn` |
| npm/pnpm | `~/.npmrc` + `<repo>/web/.npmrc` | `registry=https://registry.npmmirror.com` |
| pip | `%AppData%\pip\pip.ini` | 清华主源 + USTC 备用源 |

### 0.4.1 ⚠️ 本机代理环境的一个陷阱（实测发现）

本机开着 **Clash Verge**，监听 **`7897`**（混合端口）。但当前 shell 环境变量里配的是
**另一个代理 `56583`**，且它是个**半残代理**：

| 目标 | 经 `56583` | 经 Clash `7897` |
|---|---|---|
| `goproxy.cn`（国内镜像） | ✅ 0.75s | ✅ 0.73s |
| `registry.npmmirror.com` | ✅ 1.05s | ✅ 0.87s |
| **`proxy.golang.org`（墙外）** | ❌ **超时 10s** | ✅ **0.96s** |
| **`github.com`（墙外）** | ❌ **超时 10s** | ✅ **1.09s** |

**结论：`56583` 只能通国内镜像，墙外一律超时。** 需要访问墙外源时必须显式走 Clash 7897。

### 0.4.2 ★ 走代理会更快吗？—— 结论：国内镜像**不会**，墙外源**会**

实测对比（延迟，越低越好）：

| 目标 | 真直连 | 经 Clash 7897 | 谁快 |
|---|---|---|---|
| goproxy.cn | 1.12s | 0.73s | 代理略优（噪声级） |
| 清华 pip 源 | **1.01s** | 1.13s | **直连更快** |
| sum.golang.google.cn | **0.85s** | 1.00s | **直连更快** |
| proxy.golang.org | ❌ 不可达 | ✅ 0.52~1.10s | **只能靠代理** |

**原理**：国内镜像服务器本身就在境内，走代理反而绕远路（+TCP 握手 + 中转跳数），
差异通常在噪声范围内。**代理的唯一不可替代价值，是访问墙外源。**

**所以正确的策略是「分层」而不是「全走代理」**：

```bash
# 1) 国内镜像优先 —— 快
# 2) 墙外源兜底 —— 只在镜像缺失该模块时才用到
# 3) 让 go/git 的 direct 回退能走 Clash
export HTTPS_PROXY="http://127.0.0.1:7897"
export HTTP_PROXY="http://127.0.0.1:7897"
# 国内域名不绕代理（避免反向变慢）
export NO_PROXY="localhost,127.0.0.1,goproxy.cn,mirrors.aliyun.com,registry.npmmirror.com,pypi.tuna.tsinghua.edu.cn,mirrors.ustc.edu.cn"
```

> **不建议**全量设 `HTTP_PROXY` 而不设 `NO_PROXY` —— 那样连国内镜像也会绕代理，
> 实测反而更慢（清华源 1.01s → 1.13s）。

### 0.4.3 ★ pnpm 首次安装的两个坑（不处理会跑 1.5 小时）

**坑 A：pnpm 12.x 默认开供应链校验，国内环境等于灾难**

实测 `pnpm install --frozen-lockfile` 跑了 **1h28m55s 后失败**：

```
[WARN] Request took 193520ms: https://registry.npmmirror.com/is-finalizationregistry
✗ Lockfile failed supply-chain policy check (908 entries in 1h 28m 52.7s)
Error: ERR_PNPM_META_FETCH_FAIL
```

原因是 pnpm 会为 lockfile 里**每个条目逐个联网校验**（908 条）。

> **诊断要点**：单独 `curl` 那些"卡住"的 URL 其实**只要 0.4~0.9 秒**，
> 32 并发压测镜像站也不限流 → 说明**不是网络问题**，是 pnpm 内部校验。

**修复**（已写入 `web/.npmrc`，也可命令行加参数）：

```bash
pnpm install --frozen-lockfile --trust-lockfile
```

**坑 B：pnpm store 默认落在盘符根目录**

项目在 D 盘 → store 在 `D:\.pnpm-store`，可能撞沙箱/权限。如需改：

```bash
pnpm install --store-dir "D:/workbuddy output/2026-09-15-20-56-42/.pnpm-store"
```

**坑 C（反直觉）：并发不要调大**

本机 20 核，曾把 `network-concurrency` 从 16 提到 32「提速」，
结果请求延迟劣化到 **190~270 秒**，install 直接失败。
**已改回默认 16/5** —— 调并发前先压测，确认瓶颈真的是并发数。

### 0.4.4 pnpm 并发与超时（`web/.npmrc` 已配）

```ini
network-concurrency=16
child-concurrency=5
fetch-retries=5
fetch-timeout=180000
trust-lockfile=true
```

### 0.4.5 ★ 依赖没装完时，别干等 —— 先跑秒级静态自检

`pnpm install` / `go build` 动辄几十分钟。**不要干等**，用下面这些**不依赖依赖**的检查
提前排除低级错误。实测这几项 3 秒内出结果，能拦掉大部分"等一小时才发现"的问题。

```bash
cd "D:/workbuddy output/2026-09-15-20-56-42"

# ---- Go：语法 + 跨包符号引用 ----
GOFMT="C:/Users/ACER/.workbuddy-ai/binaries/go/v126/go/bin/gofmt.exe"
"$GOFMT" -l internal/models/cs_models.go     # -l 列出待格式化文件（有语法错误也会列出）
"$GOFMT" -l internal/...                     # 批量检查

# ---- 前端：i18n key + 具名 import（不需要 node_modules）----
"/c/Program Files/nodejs/node.exe" scripts/check_i18n_keys.cjs
"/c/Program Files/nodejs/node.exe" scripts/check_imports.cjs

# ---- 数据：官方场景映射三处锁步 ----
"C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe" \
    scripts/verify_scene_mapping.py
```

> ⚠️ **易错点**：`go fmt -n <file>` 只**打印将执行的命令**，**不是**报错信号。
> 判断语法错误用 `gofmt -l`（列出待格式化文件）。
> 注意：`gofmt -e` 会把**格式化后的完整代码**打印到 stdout，不是只报错。

**已通过的基线**（2026-09-20）：

| 检查 | 结果 |
|---|---|
| 7 个新增 Go 文件 `gofmt -e` | ✅ 无语法错误 |
| Go 跨包符号（models 10 / registry 15 / runtime 5） | ✅ 全部存在 |
| 工作流节点注册点（`executor.go` L280 / `registry.go` L33,336） | ✅ 已挂载 |
| 前端 i18n（97 key / 60 引用） | ✅ 全部命中 |
| 前端具名 import（11 模块 / 34 处） | ✅ 全部命中 |
| 场景映射 41 类三处锁步 | ✅ PASS |

### 验证镜像是否真的生效

```bash
# Go：应当 3 秒内完成，不应有 8 秒级停顿
"$GO" mod download github.com/gin-gonic/gin

# 四个镜像连通性（应全部 200）
for u in https://goproxy.cn https://sum.golang.google.cn \
         https://registry.npmmirror.com https://pypi.tuna.tsinghua.edu.cn/simple; do
  curl -L -o /dev/null -s -w "%{http_code} %{time_total}s  $u\n" --max-time 10 "$u"
done
```

---

## 0.5 一键自检（提交前必跑）

```bash
cd "D:/workbuddy output/2026-09-15-20-56-42"
bash scripts/selfcheck.sh
```

它会把四条独立链路串起来验一遍，任何一条断了都能立刻定位：

| 环节 | 检查内容 | 通过标准 |
|---|---|---|
| 环境 | Go / Node / venv / 官方数据源 | 四项均存在 |
| 数据管道 | `pipeline.py` 重跑 | 会话数 = 138，critical = 10（**不随映射表变动而变**） |
| 落库 | `seed_risk_db.py` 写 SQLite | 138 会话 + >0 条风险事件 |
| 前端静态 | i18n key + import 一致性 | 全部命中（**秒级，不需 node_modules**） |
| 后端 | `go build ./...` + 规则引擎测试 | 编译无错、测试全绿 |
| 前端类型 | `pnpm typecheck` | 无类型错误 |

依赖未就绪时可跳过单项：

```bash
bash scripts/selfcheck.sh --skip-go --skip-web
```

**两个秒级静态检查也可以单独跑**（改完前端立刻验，不用等 typecheck）：

```bash
node scripts/check_i18n_keys.cjs   # 抓 t("不存在的key")，这类错误 TS 抓不到
node scripts/check_imports.cjs     # 抓 import 了不存在导出名
```

---

## 1. 数据准备（纯离线，可随时重跑）

### 1.1 生成风险数据

```bash
cd "D:/workbuddy output/2026-09-15-20-56-42/scripts"

C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe pipeline.py \
    --xlsx "D:/浏览器下载/赛题 1：数据共情者-业务数据.xlsx" \
    --out ../build
```

**产出**：

- `build/cs_session_context.json` —— 138 个会话的完整上下文
- `build/summary.json` —— 统计摘要

**预期输出**：

```
会话总数: 138
风险等级分布: {'critical': 10, 'high': 28, 'medium': 43, 'low': 46, 'none': 11}
命中风险会话数: 127
命中多类风险会话数: 37
风险类别分布: {'timeliness': 72, 'safety': 32, 'opinion': 9, 'compliance': 23, 'experience': 44}
证据不一致数: 9
```

> **基线变更说明（2026-09-20）**：上述数字是**补齐官方 41 类场景映射后**的新基线。
> 修复前 `SCENE_MINOR_TO_RISK` 只覆盖 15 类，导致 29 个官方场景在离线 Demo 里
> **完全不具备风险等级**（静默失效，不报错）。修复后 `none` 从 67 降至 11。
>
> **关键：`critical` 仍为 10，未升未降。** 这条是本修复的安全底线——
> 补齐新类别不得稀释既有的最高危判定。改动映射表后请核对这个数。

### 1.2 数据落库（二选一）

#### 方案 A：SQLite 直写（**推荐，零依赖**）

不需要服务在线、不需要渠道、不需要登录态。

```bash
C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe seed_risk_db.py \
    --build ../build \
    --sqlite "<agent-desk 数据目录>/agentdesk.db" \
    --truncate
```

#### 方案 B：MySQL 直写

```bash
C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe seed_risk_db.py \
    --build ../build \
    --dsn "cs_ai_agent:cs_ai_agent_password@127.0.0.1:3306/cs_ai_agent" \
    --truncate
```

> **前置条件**：后端必须用新代码启动过至少一次，
> 否则 `cs_session_context` / `cs_risk_event` 两张表不存在。
> `AutoMigrate` 会自动建表。

#### 方案 C：走 HTTP API 导入会话与消息

仅当需要"真实会话流"（工作台左侧能逐个点开）时使用。

```bash
C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe import_to_agentdesk.py \
    --build ../build \
    --base http://127.0.0.1:8083 \
    --channel "<dashboard 里的渠道标识>" \
    --xlsx "D:/浏览器下载/赛题 1：数据共情者-业务数据.xlsx"
```

> **注意**：`--channel` 是 `X-Channel-Id`，三个接口都会校验。
> 取值：dashboard → 渠道管理 → 列表里的「渠道标识」。
> 先用 `--dry-run` 空跑确认流程。

---

## 2. 启动服务

### 2.1 Docker 方式（推荐）

```bash
cd "D:/workbuddy output/2026-09-15-20-56-42/agent-desk"
docker compose up -d
```

访问 `http://localhost:8083`。

> ⚠️ Docker 方式会**重新构建镜像**，需要重建 Go 后端才能带上新增代码。
> 首次构建约 5-15 分钟。

### 2.2 本地方式（改后端代码时更快）

**后端**：

```bash
cd agent-desk

# 配置数据库（复制示例配置）
cp config/config.example.yaml config/config.yaml
# 编辑 config.yaml，指向你的 MySQL/SQLite

go run ./cmd/server
```

**前端**（开发模式，热更新）：

```bash
cd agent-desk/web
pnpm install
pnpm dev
```

---

## 3. 配置豆包 API

### 3.1 关键配置项

在 dashboard → **AI 配置** 里新建一条：

| 字段 | 值 |
|---|---|
| 名称 | 豆包（Demo） |
| Base URL | `https://ark.cn-beijing.volces.com/api/v3` |
| API Key | 你的方舟 API Key |
| 模型名 | 推理接入点 ID，如 `ep-2026xxxxxx-xxxxx` |

### 3.2 三条硬约束

1. **不能用 embedding 模型做向量检索**
   方舟的向量化模型**不兼容** OpenAI 的 `/v1/embeddings` 协议。
   本项目已放弃向量检索，改用**关键词匹配**（10 万字的条款用关键词足够，
   且更快、更可解释）。参见 `豆包API接入方案.md`。

2. **必须关闭 thinking**
   `doubao-seed-*` 默认开启深度思考，会导致首字延迟 5-15 秒。
   代码已自动处理（见 `isVolcengineArkThinkingModel`）。

3. **模型名填推理接入点 ID，不是模型代号**
   形如 `ep-2026...`，不是 `doubao-seed-1.6`。

### 3.3 验证清单

| # | 验证项 | 期望 |
|---|---|---|
| V1 | 配置保存后点「测试连接」 | 返回成功 |
| V2 | 在会话里发一条消息 | 3 秒内返回回复，无 `reasoning_content` |
| V3 | 查看 Agent 运行记录 | `promptTokens` / `completionTokens` 有值 |
| V4 | 发一条含「过敏」的消息 | 右侧面板出现 critical 风险提示 |

---

## 4. 验证 Demo 效果

### 4.1 风险看板

访问 `http://localhost:8083/dashboard/risk-board`

**应该看到**：

- 顶部四张卡：会话总数 138 / 待处置风险 / 图文证据不一致 9 / 等级分布
- 左侧列表：按风险等级倒序，每行带 4px 左色条
- 右侧：子场景 Top 8 条形图 + 风险类别分布

### 4.2 工作台四 Tab

访问 `http://localhost:8083/dashboard/conversations`

点开任一会话，右侧应出现四个 Tab（默认停在「会话洞察」）：

1. **会话洞察** —— 风险概况 + 判定依据（带来源标签）+ 建议动作　★ 默认落点
2. **客户全景** —— 历史会话数 / 累计实付 / 上次进线结论 / 关联订单与工单
3. **回复建议** —— 语气候选 + 置信度 + 引用证据（高危时锁定为固定指引）
4. **会话信息** —— 上游原有内容（客户资料 / 关联工单 / AI 执行记录）

另有两条贯穿整个工作台的风险提示：

- **左侧会话列表**：high / critical 会话带 4px 左色条，critical 带脉冲
- **顶部告警条**：high / critical 会话在消息区上方出现 32px 告警条，
  右侧「查看全部」直达风险看板

> 没有风险数据的会话（非演示会话）会显示空态并引导到风险看板，
> **不会弹错误提示** —— 这是刻意设计，避免现场满屏报错。

### 4.3 验证豆包结构化输出（★ 最高风险项）

整个方案里唯一的技术未知：**豆包能否稳定返回纯 JSON**。
上线前必须验一次，不通过就要改走纯规则路线。

```bash
cd "D:/workbuddy output/2026-09-15-20-56-42/scripts"

export ARK_API_KEY="你的火山方舟 API Key"
export ARK_MODEL="doubao-seed-2-1-pro-260628"   # 换成你的 Model ID

C:/Users/ACER/.workbuddy-ai/binaries/python/envs/default/Scripts/python.exe \
    probe_doubao_json.py --runs 5 --verbose
```

**脚本会验三件事**：

1. 最小连通性（Key / base_url / 模型名是否正确）
2. 关 thinking + JSON mode 下连打 5 次，统计**解析成功率**
3. 开启 vs 关闭 thinking 的延迟对照

**判读标准**：

| 结果 | 含义 | 处置 |
|---|---|---|
| 成功率 100%，无围栏无前言 | 可以直接接 S1 节点 | 无需改动 |
| 偶发围栏/前言 | 前端加 stripFence 兜底 | 可接受 |
| 成功率 < 80% | prompt 约束不住 | 改两段式：先自由答，再规则抽字段 |
| content 为空 | thinking 没关掉 | 确认补丁生效，或换非 thinking 模型 |

---

## 5. 常见问题

### Q1: `错误 error.e0211`

渠道无效。检查：
- `--channel` 与 dashboard 里的渠道标识**完全一致**（区分大小写）
- 渠道 `status` 为启用

### Q2: `错误 error.e0150`

外部身份缺失。Guest 通道需要 `X-External-Id` + `X-External-Name`；
User 通道需要 `Authorization: Bearer <userToken>`。

### Q3: 风险看板空白

1. 确认两张表有数据：
   ```bash
   sqlite3 agentdesk.db "SELECT COUNT(*) FROM cs_session_context;"
   ```
2. 确认后端是新代码构建的（旧镜像没这两张表）
3. 打开浏览器控制台看 `/api/dashboard/cs-risk/board` 的返回

### Q4: 豆包返回空内容

thinking 没关。检查 `Base URL` 是否含 `volces.com`，
模型名是否以 `doubao-seed` 开头。

### Q5: Go 编译报 `go: command not found`

本项目验证用的 Go 在本机路径：

```bash
C:/Users/ACER/.workbuddy-ai/binaries/go/v126/go/bin/go.exe
```

配置环境变量：

```bash
export GOPATH="C:/Users/ACER/.workbuddy-ai/go-path"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOPROXY="https://goproxy.cn,direct"
```

---

## 6. 目录速查

```
D:/workbuddy output/2026-09-15-20-56-42/
├── agent-desk/                     上游开源项目（含我们的改造）
│   ├── internal/models/cs_models.go              新增：风险数据模型
│   ├── internal/services/cs_risk_service.go      新增：风险规则引擎
│   ├── internal/services/cs_risk_service_test.go 新增：规则引擎测试
│   ├── internal/repositories/cs_repository.go    新增：仓储层
│   ├── internal/handlers/dashboard/cs_risk_handler.go  新增：看板接口
│   ├── internal/ai/runtime/workflow/cs_risk_guard_node.go  新增：工作流节点
│   ├── internal/ai/workflow/registry/cs_risk_guard_spec.go 新增：节点元数据
│   ├── web/components/workbench/                  新增：前端业务组件（8 个）
│   ├── web/components/workbench/cs-risk-tabs.tsx  新增：四 Tab 容器
│   ├── web/app/dashboard/risk-board/page.tsx      新增：风险看板页
│   ├── web/lib/api/cs-risk.ts                     新增：前端 API 客户端
│   └── web/lib/stores/cs-risk-map.ts              新增：列表角标数据源
├── scripts/
│   ├── pipeline.py               数据管道（离线跑）
│   ├── seed_risk_db.py           落库（SQLite/MySQL）
│   ├── import_to_agentdesk.py    API 导入（可选）
│   ├── probe_doubao_json.py      豆包结构化输出探针（★ 必跑）
│   ├── selfcheck.sh              一键自检（提交前必跑）
│   ├── check_i18n_keys.cjs       i18n key 引用校验（秒级，不依赖 node_modules）
│   ├── check_imports.cjs         import/export 一致性校验（秒级，同上）
│   ├── add_csrisk_i18n.cjs       i18n 全量注入
│   ├── patch_csrisk_source_i18n.cjs  i18n 增量补丁（幂等）
│   └── fix_runbook_python_path.cjs   本手册的路径替换工具
├── build/                        管道产出
├── agent-desk-魔改方案V1.md       总方案
├── 豆包API接入方案.md              API 接入细节
├── 前端改造方案.md                 前端设计规范
├── 架构与迭代维护规划.md            变更边界与长期维护
└── 运行手册.md                    本文件
```

---

## 6.5 改动清单（上游文件只动了这些）

改动全部包在 `[AgentDesk-BEGIN/END]` 标记里，`grep -rn "AgentDesk"` 可全量核对。

| 上游文件 | 改动量 | 内容 |
|---|---|---|
| `internal/models/models.go` | +4 行 | 注册两张新表 |
| `internal/ai/workflow/registry/registry.go` | +4 行 | 注册 `risk_guard` 节点 |
| `internal/ai/runtime/workflow/executor.go` | +4 行 | 节点分发（插在 reply_policy 前） |
| `internal/bootstrap/routes.go` | +10 行 | 4 条 cs-risk 路由 |
| `internal/bootstrap/server.go` | +3 行 | 挂载路由组 |
| `internal/ai/application/runtime/eino_agent_loop.go` | ~+25 行 | 豆包 thinking 关闭 |
| `internal/ai/llm.go` | ~+25 行 | 同上（另一 SDK 调用点） |
| `web/app/dashboard/conversations/_components/conversation-info-panel.tsx` | ~+8 行 | 包一层 Tab 容器 |
| `web/app/dashboard/conversations/_components/conversation-workbench.tsx` | ~+90 行 | 告警条 + 自取数组件 |
| `web/app/dashboard/conversations/_components/conversation-list.tsx` | ~+20 行 | 风险色条 |
| `web/lib/navigation.tsx` | +8 行 | 侧栏「风险看板」入口 |
| `web/messages/zh-CN.json` / `en-US.json` | 追加命名空间 | `csRisk`（28 组） |

---

## 7. 变更边界速记

改代码前请记住（详见《架构与迭代维护规划.md》）：

- **禁止改** `web/components/ui/**` —— 这是 shadcn 的原子层
- **只追加** 上游既有文件（每处改动 ≤ 5 行，用 `[AgentDesk-BEGIN/END]` 包裹）
- **我们自己的**代码放新文件/新目录

自检 originality：

```bash
cd agent-desk
grep -rn "AgentDesk" . --include=*.go --include=*.ts --include=*.tsx --include=*.css
```
