# 前端改造方案 · 设计规范与实施

> **目标**：在 agent-desk 上做出「AI 辅助人工客服」的工作台，服务 3 分钟讲清价值的演示
> **技术栈**：Next.js 16.1.6 + React 19.2.3 + shadcn/ui + Tailwind CSS v4 + **Recharts 2.15.4**（已有）
> **日期**：2026-09-20

---

## 0. 最重要的一条纪律

> **复用现有设计 token，不新引入任何颜色。**

agent-desk 已经有一套完整的 CSS 变量体系（`web/app/globals.css`，368 行，**4 套调色板 × 明暗双模式**）。它的 `--primary` 等 token 已经通过 `shadcn/tailwind.css` 暴露成 Tailwind 类（`bg-primary` / `text-muted-foreground`）。

**我们所有的风险色、情绪色，都以「新增语义 token」的方式加进 `globals.css`**，而不是在组件里写死 `#E5484D`。理由：

1. 深色模式自动跟随（agent-desk 已有 `.dark` 变体）
2. 4 套调色板不会被打乱
3. **后期迭代维护成本最低** —— 改一处，全局生效

---

## 1. 现有设计资产盘点（`[确定]`）

### 1.1 设计 token（`web/app/globals.css`）

**默认调色板 `plain`**：

```
--background       #f9f9f9      浅灰底
--foreground       #151923      主文字（近黑）
--card             #ffffff      卡片白
--primary          #2475fc      品牌蓝 ← 注意：不是 Intercom 的 #4A8BFF
--secondary        #e6efff      浅蓝
--muted            #f4f7fc      极浅灰蓝
--muted-foreground #667085      次级文字
--border           #dfe7f3      边框（带蓝调）
--radius           0.3rem       ← 比 shadcn 默认 0.5rem 更方，偏「工具感」
```

**图表色板（`--chart-1` ~ `--chart-5`）**：
```
#2475fc 蓝 / #14b8a6 青 / #f59e0b 橙 / #ec4899 粉 / #8b5cf6 紫
```

**深色模式**：`#111827` 底 / `#182235` 卡 / `--primary` 提亮为 `#6da2ff`

**4 套调色板**：`plain`（蓝） / `green`（teal 系） / `blue` / `gray`

> **设计判断**：`--radius: 0.3rem` 说明这个产品走的是**紧凑工具风**，不是消费级圆润风。**我们的组件要沿用这个手感**，别引入 `rounded-xl`。

### 1.2 可复用组件（`web/components/ui/`）

shadcn 全套已在：`card` / `badge` / `button` / `tabs` / `avatar` / `dropdown-menu` / `chart` / `resizable` / `sheet` / `field` / `input` / `textarea` / `toggle-group` / `select`

**图表**：`ChartContainer` + `ChartTooltip` + `ChartConfig` 已封装好，对标 shadcn 官方 charts。

### 1.3 布局（`conversation-workbench.tsx`，535 行）

```
ResizablePanel  defaultSize 20%   minSize 10%  maxSize 40%  ← 会话列表，可折叠
ResizableHandle
ResizablePanel  defaultSize 50%   minSize 32%                ← 聊天窗口
ResizableHandle
ResizablePanel  defaultSize 500px minSize 20% maxSize 40%  ← 右侧面板，可折叠
```

**右侧面板容器**：`conversation-info-panel.tsx` 第 257–267 行

```tsx
<div className="space-y-4 py-3">
  <section><DetailRow label="会话ID" .../></section>
  <CustomerBody conversation={conversation} />
  <WorkflowRunsSection conversation={conversation} />
</div>
```

**这就是我们的插入点。**

### 1.4 菜单配置（`web/lib/navigation.tsx`）

`dashboardNavSections` 数组，三段：
- `nav.receptionCenter`（接待中心）：概览 / 会话 / 工单 / 会话监控 / 客户 / 公司
- `nav.agentConfig`（客服配置）：标签 / 快捷回复 / 客服 / 排班 / 渠道
- `nav.aiCapabilities`（AI 能力）：知识库 / 模型配置 / AI Agent / 工作流

**新增看板页**：在这加一项，或新开一段。

### 1.5 i18n

`web/messages/zh-CN.json` + `en-US.json`，用 `useI18n()` → `t("key")`。

**我们新增的所有文案都要双语补齐**，否则切英文会显示 key 本身。（演示只用中文，但**别留坑**。）

---

## 2. 风险与情绪的语义 token（新增）

**加进 `web/app/globals.css`**，在 `:root` 块末尾追加：

```css
:root,
:root[data-palette="plain"] {
  /* ...原有 token 不动... */

  /* ===== 新增：风险分级语义色 ===== */
  --risk-none:     #667085;              /* 无风险 - 中性灰 */
  --risk-low:      #3ecf8e;              /* 低 - 绿 */
  --risk-medium:   #f7b731;              /* 中 - 黄 */
  --risk-high:     #ff8c42;              /* 高 - 橙 */
  --risk-critical: #e5484d;              /* 高危 - 红 */

  /* 高危浅底（不刺眼的关键：低饱和底 + 高饱和边） */
  --risk-critical-bg:  #fff5f5;
  --risk-high-bg:      #fff7ed;
  --risk-medium-bg:    #fffbeb;

  /* ===== 新增：情绪语义色 ===== */
  --emotion-negative: #e5484d;
  --emotion-neutral:  #667085;
  --emotion-positive: #3ecf8e;

  /* ===== 新增：AI 置信度 ===== */
  --confidence-high:   #3ecf8e;
  --confidence-medium: #f7b731;
  --confidence-low:    #ff6b6b;
}

/* 深色模式：只降亮度，不换色相 */
.dark[data-palette="plain"] {
  --risk-critical: #c93a3f;
  --risk-high:     #e07a37;
  --risk-medium:   #d9a02a;
  --risk-critical-bg:  #2a1719;
  --risk-high-bg:      #2a1f16;
  --risk-medium-bg:    #2a2416;
}
```

**再在 `@theme inline` 块里注册**，才能用 Tailwind 类：

```css
@theme inline {
  /* ...原有不动... */
  --color-risk-none:     var(--risk-none);
  --color-risk-low:      var(--risk-low);
  --color-risk-medium:   var(--risk-medium);
  --color-risk-high:     var(--risk-high);
  --color-risk-critical: var(--risk-critical);
  --color-risk-critical-bg: var(--risk-critical-bg);
  --color-risk-high-bg:     var(--risk-high-bg);
  --color-risk-medium-bg:   var(--risk-medium-bg);
  --color-emotion-negative: var(--emotion-negative);
  --color-emotion-neutral:  var(--emotion-neutral);
  --color-emotion-positive: var(--emotion-positive);
  --color-confidence-high:   var(--confidence-high);
  --color-confidence-medium: var(--confidence-medium);
  --color-confidence-low:    var(--confidence-low);
}
```

之后就能写 `bg-risk-critical-bg border-risk-critical text-risk-critical`。

> ⚠️ **`green` / `blue` / `gray` 三套调色板也要补**这几行，否则切调色板会掉色。**这是必须做的，不是可选项。**

---

## 3. 核心组件设计

### 3.1 风险徽章 `<RiskBadge>`

**四档，不用百分比。**

| 档位 | 视觉 | 用法 |
|---|---|---|
| 低 | 仅一个 6px 色点 | 列表行内 |
| 中 | 色点 + 文字标签 | 列表行内 |
| 高 | 4px 左色条 + 浅橙底卡 | 右侧面板 |
| 高危 | 4px 左色条 + 浅红底 + `AlertTriangle` 图标 + **脉冲动画** | 右侧面板 + 顶部告警条 |

```tsx
// web/components/workbench/risk-badge.tsx
const RISK_STYLE = {
  none:     { dot: "bg-risk-none",     text: "text-muted-foreground", label: "无风险" },
  low:      { dot: "bg-risk-low",      text: "text-risk-low",         label: "低" },
  medium:   { dot: "bg-risk-medium",   text: "text-risk-medium",      label: "中" },
  high:     { dot: "bg-risk-high",     text: "text-risk-high",        label: "高" },
  critical: { dot: "bg-risk-critical", text: "text-risk-critical",    label: "高危" },
} as const;
```

**「不刺眼但一定被看到」的三个具体手段**（来自竞品调研）：

1. **高危不用大面积红填充** → 用 `1px` 描边 + 8% 透明浅底 + `4px` 左色条
2. **顶部告警条** 32px 高，插在 `workspaceContent` 的顶栏下方 —— 比右侧面板**更早进入视野**
3. **`animate-pulse`** 1.5s 一次，幅度极小（Tailwind 内置，零成本）

> 第 3 条是「不刺眼但被看到」的核心。**动画会强制吸引注意力，但低幅度不会造成焦虑。**

### 3.2 置信度胶囊 `<ConfidenceChip>`

**硬规则：用 3 档，不用百分比。**

```
高  → 灰字（弱化，不要每条都上色）
中  → #F7B731
低  → #FF6B6B + 必须带原因文字
```

**为什么不用百分比**：未校准的数字是负债。评委（业务专家）看到「92.4%」第一反应是「你怎么校准的」。**看到「高 / 中 / 低」不会有这个问题。**

**低置信度必须给出口**：
```
低置信度 · 知识库无同类记录，建议人工核对
```
只说「AI 不确定」是给用户焦虑不给出口。

### 3.3 建议回复卡 `<ReplySuggestionCard>`

**给 3 条，不是 1 条。** 这是演示的「哇」时刻。

```
┌─────────────────────────────────────┐
│ [专业]                        ⭐推荐  │  ← 语气 chip
├─────────────────────────────────────┤
│ 您好，非常抱歉给您带来不适。关于您  │
│ 提到的过敏情况，请您先停用该产品…   │  ← 最多 4 行，超出滚动
├─────────────────────────────────────┤
│ [采用]  [编辑]  [换一条]            │
└─────────────────────────────────────┘
```

**三档语气**：`专业` / `亲切` / `简洁`

**一键采用的关键设计**：
- hover 时浮出 `Tab 采用` 快捷键提示
- **采用后流式写入输入框，不直接发送** ← 保留人工确认，这是客服场景的底线，也是答辩时能讲的点

### 3.4 右侧面板三 Tab 结构

改 `conversation-info-panel.tsx`：

```
会话ID（保留）
    ↓
CustomerBody（保留）
    ↓
【新增】Tabs
  ├─ AI 洞察（默认）  ← 演示主角
  ├─ 客户档案
  └─ 时间线
    ↓
WorkflowRunsSection（保留，移入 AI 洞察 Tab 底部）
```

**AI 洞察 Tab 内的固定信息层级（顺序不要改）**：

| # | 区块 | 高度 | 内容 |
|---|---|---|---|
| 1 | **状态行** | 40px | `意图：退换货咨询` + 置信度胶囊 |
| 2 | **情绪 + 风险双卡** | 72px | 各 50% 宽并排 |
| 3 | **建议回复** | flex-1 | 3 张卡 |
| 4 | **AI 执行记录** | 折叠 | 复用现有 `WorkflowRunsSection` |

> **保留 `WorkflowRunsSection` 是个心机** —— 它证明我们的 Agent **真的跑了**，不是前端写死的假数据。评委问「这是真的还是假的」，你能当场展开给他看。

---

## 4. 新页面：风险预警看板

**路由**：`web/app/dashboard/risk-board/page.tsx`
**菜单**：加进 `navigation.tsx` 的 `nav.receptionCenter` 段

```
{
  titleKey: "nav.riskBoard",
  url: "/dashboard/risk-board",
  icon: <ShieldAlertIcon />,
  requiredPermission: "conversation.view",   // 复用现有权限，别新建
}
```

> **复用现有权限码**是个重要决策 —— 新建权限要改后端权限表 + 迁移脚本，**没必要**。

### 页面结构（按调研结论，**只放 6 个指标**）

**第 1 行：4 张数字卡**（高 96px，带环比箭头 + sparkline）

| 指标 | 值（MOCK 数据） | 来源 |
|---|---|---|
| 总会话数 | 138 | 已核实 |
| 命中风险会话 | 88（64%） | 已核实 |
| 未闭环工单 | 28 | 已核实 |
| 话术同质化率 | 35% | 已核实 |

**第 2 行：折线图**（Recharts `LineChart`，双线同色系深浅）
- 近 7 日会话量 vs AI 介入量
- **用折线不用柱状** —— 趋势讲得清

**第 3 行：横向条形榜** `风险会话 TOP 5`（带风险色条，可点击跳转）

**第 4 行：商品×批次聚类表**（我们独有的亮点）

| 商品 | 批次 | 不良反应数 | 标记 |
|---|---|---|---|
| B5 面膜 | 20250301 | 1 | |
| B5 面膜 | 20250415 | 1 | |
| 修护精华 | — | 2 | |
| 焕活礼盒 | — | 2 | |
| **B5 面膜** | **00** | **1** | ⚠️ **数据质量异常** |

**为什么这张表是亮点**：技术只要 `group by`，但**做了真人运营会做的事** —— 评委席上的业务专家一眼就认。

> **不要用仪表盘/环形图**：MOCK 数据下环形图显得空，横向条形信息效率高 3 倍。

---

## 5. 会话列表的风险标识

**文件**：`web/app/dashboard/conversations/_components/conversation-list.tsx`

### 5.1 现有结构（`[确定]`，已读源码）

```
第 23-46 行   <ScrollArea> → conversations.map() → 外层 div
              className="cursor-pointer border-b border-border/80 px-3 py-2 ..."
第 47-80 行   <div className="overflow-hidden">
                ├─ 第 48-77 行：flex 行 → Avatar(size-7) + 客户名 + 未读角标 + 时间
                └─ 第 78-80 行：lastMessageSummary（truncate 单行）
```

**关键**：外层 `div` **没有 `flex`**，是普通块级 —— 这意味着**可以直接在最前面插入一个绝对定位的色条**，或改成 `flex` 后加一个固定宽度的色条。**推荐后者**（更稳）。

### 5.2 改法

```
│▌ 张三                    14:32 ●│   ← 色条 + 客户名(未读加粗) + 时间 + 未读角标
│  我用了之后脸过敏了，要投诉…       │   ← 摘要，单行截断
│  [高危] [过敏] [投诉]              │   ← 新增标签行
```

**具体步骤**：

1. 外层 `div` 加 `flex`，插入色条元素：
```tsx
{/* ===== [AgentDesk-BEGIN] 风险色条 ===== */}
<span
  aria-hidden
  className={`w-1 shrink-0 self-stretch ${RISK_BAR_STYLE[riskLevel]}`}
/>
{/* ===== [AgentDesk-END] ===== */}
```

2. 原 `overflow-hidden` 内层包一层 `min-w-0 flex-1`

3. 摘要下方追加标签行

### 5.3 关键设计

- **色条放最左侧**，`w-1` + `self-stretch` 贯穿整行 —— 比徽章更快被扫到
- **不要只靠颜色**：高危行**再加浅红底** `bg-risk-critical-bg`
- **高危置顶** + `置顶` 分组标题，隔开「需要立刻处理」和「常规」
- **超时不用红色**：把时间戳变成橙字 `距上次回复 8 分钟`，超 10 分钟转红 + 沙漏图标 —— **用语义文字而非纯色，评委一眼能懂**

### 5.4 数据来源问题 ⚠️

`conversation-list.tsx` 的数据来自 `useAgentConversationsStore`，其类型是 `AgentConversation` —— **这个类型里没有 `riskLevel` 字段**。

**两个解法**：
- **解法 A（推荐）**：前端从 `cs_risk_event` 额外拉一次，在 store 里合流成 `Map<conversationId, riskLevel>`
- **解法 B**：改后端 `BuildConversationWithLocale` 加字段（**动了上游 builder，不推荐**）

**选 A** —— 符合「不上下游 builder」的纪律。

> **MOCK 数据的关键技巧**：138 个会话只需在**前 5 行**做出层次（1 个高危置顶、2 个未读、2 个超时），剩下的用同一模板批量生成。**评委只会看前 5 行。**

---

## 6. 改动文件清单

| 文件 | 改动 | 类型 |
|---|---|---|
| `web/app/globals.css` | 新增风险/情绪/置信度语义 token（**5 个块**） | 改 |
| `web/components/workbench/risk-badge.tsx` | **新建** | 新 |
| `web/components/workbench/confidence-chip.tsx` | **新建** | 新 |
| `web/components/workbench/reply-suggestion-card.tsx` | **新建** | 新 |
| `web/components/workbench/customer-panorama.tsx` | **新建** | 新 |
| `web/components/workbench/risk-alert-bar.tsx` | **新建** | 新 |
| `web/app/dashboard/conversations/_components/conversation-info-panel.tsx` | 插入 Tabs（257-267 行） | 改 |
| `web/app/dashboard/conversations/_components/conversation-list.tsx` | 风险色条 + 置顶分组 | 改 |
| `web/app/dashboard/conversations/_components/conversation-workbench.tsx` | 插入告警条 | 改 |
| `web/app/dashboard/risk-board/page.tsx` | **新建** | 新 |
| `web/lib/navigation.tsx` | 加菜单项 | 改 |
| `web/messages/zh-CN.json` | 加文案 | 改 |
| `web/messages/en-US.json` | 加文案 | 改 |

**全部改动集中在 `web/components/workbench/`（新目录）和 3 个既有文件。** 这是刻意的 —— 见下节。

---

## 7. 前端迭代维护规划

### 7.1 核心难题：上游更新会冲掉我们的改动

agent-desk 是活跃项目（最后提交 2026-09-14）。如果我们直接改它的文件，上游一更新，**我们的改动就冲突了**。

### 7.2 解法：三层隔离

```
┌─────────────────────────────────────────┐
│ 第 1 层：上游文件（尽量不改）            │
│   conversation-workbench.tsx / list.tsx  │
│   → 只做「插入」，改动控制在 5 行以内    │
├─────────────────────────────────────────┤
│ 第 2 层：我们的组件（新目录）            │
│   web/components/workbench/**            │
│   → 所有业务组件都在这，上游文件不含此目录│
│   → 冲突概率 ≈ 0                         │
├─────────────────────────────────────────┤
│ 第 3 层：我们的页面（新路由）            │
│   web/app/dashboard/risk-board/**        │
├─────────────────────────────────────────┤
│ 第 4 层：我们的 token（追加式）          │
│   globals.css 末尾追加，不删不改原行     │
└─────────────────────────────────────────┘
```

**规则**：
1. **新功能一律新建文件**，不改上游文件
2. 必须改上游文件时，改动**只做「加法」**（插入一行 import + 插入一个组件），**绝不删改原代码**
3. **绝不改** `web/components/ui/**`（shadcn 原子组件）
4. 所有我们新增的代码块加统一标记注释：

```tsx
{/* ===== [AgentDesk] 开始 ===== */}
<Tabs defaultValue="insight">...</Tabs>
{/* ===== [AgentDesk] 结束 ===== */}
```

**这样上游更新时，用 `grep -r "AgentDesk"` 就能定位全部改动点，合并成本趋近于零。**

### 7.3 Git 分支策略

```
upstream/main  ← 上游（只读，用来同步）
    ↓
main           ← 我们的主线
    ↓
feat/*         ← 功能分支
```

**建议**：
- 我们自己的改动**全部在 `main` 上**，`upstream/main` 保持纯净
- 需要跟随上游时：`git fetch upstream && git merge upstream/main`，冲突只可能在那 3 个上游文件里
- **第一天就 `git remote add upstream <原仓库地址>`** —— 别等要用了才加

### 7.4 后端同理

| 层 | 文件 | 策略 |
|---|---|---|
| 上游 | `internal/ai/workflow/registry/registry.go` | **追加枚举值**，不删原有。加 `[AgentDesk]` 标记 |
| 上游 | `internal/ai/application/runtime/eino_agent_loop.go` | 补丁 A：**新增函数 + 改一行调用** |
| 我们 | `internal/services/` 新增服务 | 新文件，如 `cs_risk_service.go` |
| 我们 | `cmd/` 新增导入命令 | 新文件，如 `cmd/csimport/` |

### 7.5 配置与凭据

**绝对不要提交 API Key。** 已确认 agent-desk 的模型配置在**数据库里**（`models.AIConfig`），不在配置文件 —— 这天然安全。

**但要补**：
- `.gitignore` 确认包含 `config/config.yaml`
- 团队共享用 1Password / 微信单独传，**别贴群里、别进仓库**
- `cmd/testdata` 的种子数据里有 `aiconfig` 初始化 —— **确认它不会写入真实 Key**

### 7.6 后续可扩展点（预留设计）

设计时刻意留了几个口子，后期加功能不用重构：

| 扩展想做的事 | 预留的口子 |
|---|---|
| 加第 4 个 Tab | AI 洞察 Tab 是数组驱动的，加一项配置即可 |
| 加新的风险类型 | `--risk-*` token 已参数化，加一套色值即可 |
| 加新的评估指标 | 看板卡片是组件化的，加一个 `<MetricCard>` 实例 |
| 接真实千牛渠道 | `Channel` 模型已支持多渠道路由 |
| 换模型供应商 | `AIConfig` 后台可配，**不用改代码** |

### 7.7 演示前的固化

**比赛交付前必须做**：
1. `git tag v1.0-demo` 打标签
2. 导出 SQLite 数据库文件（含灌好的 MOCK 数据）
3. 写 `RUNNING.md`：一条命令跑起来的步骤
4. **用预构建镜像，别现场 `pnpm build`**（要 10–30 分钟）

---

## 8. 执行顺序

| 序 | 内容 | 人天 | 依赖 |
|---|---|---|---|
| 1 | `globals.css` 加语义 token（5 个块） | 0.3 | — |
| 2 | 新建 `web/components/workbench/` 4 个组件 | 1 | 1 |
| 3 | 改 `conversation-info-panel.tsx` 插三 Tab | 0.5 | 2 |
| 4 | 改 `conversation-list.tsx` 风险标识 | 0.5 | 2 |
| 5 | 改 `conversation-workbench.tsx` 插告警条 | 0.2 | 2 |
| 6 | 新建 `risk-board/page.tsx` + 菜单 | 1.5 | 1,2 |
| 7 | i18n 中英双语补齐 | 0.3 | 全部 |
| 8 | 演示数据打磨（前 5 行做层次） | 0.7 | 全部 |

**合计 5 人天**（前端部分）

---

## 9. 待确认

| # | 事项 | 状态 |
|---|---|---|
| 1 | 右侧面板当前 500px 固定宽，**装三 Tab 够不够**？可能要调成 360px 固定或改百分比 | 待实测 |
| 2 | ~~`conversation-list.tsx` 的行结构~~ | ✅ **已读源码，见 §5.1** |
| 3 | 看板的 7 日趋势图，MOCK 数据**从哪来**？可能要造一份 | 待定 |
| 4 | 情绪等级 1–5 的语义色，**5 档还是压成 3 档**？建议压 3 档（正/中/负） | 待定 |
| 5 | `AgentConversation` 类型加 `riskLevel` 的合流方式（见 §5.4 解法 A） | 待实施 |

---

## 10. 演示动线（设计服务于 3 分钟）

调研结论给出的动线，**所有 UI 设计都要服务于此**：

```
① 会话列表      发现高危置顶 → "为什么它在最上面？"
        ↓
② 点进去        顶部告警条亮起 + 脉冲
        ↓
③ 右侧 AI 洞察  一屏看完 意图 / 情绪 / 风险 / 建议回复
        ↓
④ 点「采用」    建议回复流式写入输入框
        ↓
⑤ 切到看板      138 会话 64% 有风险信号，其中 12 个涉及安全
                收尾："这些在今天，全靠客服个人的经验去发现。"
```

**砍掉不做的**（省工时）：
- 深色模式打磨（做但不必讲）
- 移动端适配（加一句「请使用桌面端」提示即可）
- 审计轨迹、客户档案 Tab 完整度（放 3 条 MOCK 数据）
- 看板的筛选下钻

---

*基于 2026-09-20 源码快照。若拉取上游新版本，`conversation-info-panel.tsx` 第 257-267 行需重新核对。*
