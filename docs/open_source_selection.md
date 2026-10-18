# 开源选型与魔改清单 · 数据共情者

> 团队：Star of CAU ｜ 用途：为赛题1 选开源底座，避免重复造轮子
> 检索时间：2026-09-15 ｜ 标注：`[确定]` 已核实原文 ｜ `[推测]` 方向可信待实测 ｜ `[待确认]` 需自行核实

---

## 零、先说结论

**没有能直接 fork 成一个完整参赛作品的开源项目。** 但有三类东西可以省掉大约 **10–15 人天**（占总量 40–60%）：

| 层级 | 用法 | 能省 |
|---|---|---|
| **Tier A** | 真 fork，当底座（1 个候选） | 5–8 人天（**前提：队里有人会 Go**） |
| **Tier B** | 只抄架构和设计，不抄代码 | 2–3 人天 |
| **Tier C** | 直接用现成零件（脚手架 / 模型） | 3–5 人天 |
| **Tier D** | **别碰**（看着对口，实为坑） | 避免浪费 5 人天以上 |

---

## 一、Tier A：值得真 fork 的底座

### `huabeitech/agent-desk` — 开源 AI 客服系统（Apache-2.0）

| 项 | 内容 | 确定性 |
|---|---|---|
| License | **Apache-2.0** | `[确定]` |
| Star / Fork | 约 155 ★ / 48 fork | `[推测]` 第三方目录站数据 |
| 后端 | **Golang + Gin + GORM** | `[确定]` 官方 README |
| 前端 | **Next.js 16 + React 19 + shadcn/ui + Tailwind CSS** | `[确定]` |
| 数据库 | SQLite / MySQL | `[确定]` |
| 向量库 | Qdrant（**已支持 LanceDB**，可去掉 Qdrant 依赖） | `[确定]` |
| 部署 | Docker Compose 一键起，默认 8083 端口 | `[确定]` |
| 活跃度 | 最近提交 **2026-09-14**（检索前一天），近 90 天 759 commits | `[确定]` |
| 白标 | 支持 `COMPANY_NAME` / `COMPANY_LOGO_URL` 环境变量，登录页 / 条款页 / 支持中心头部均可换品牌 | `[确定]` |

**它自带什么（正是我们要的）**
- **Agent Workspace**：会话列表、消息收发、会话标签、关联客户、工单上下文
- 知识库 RAG：文档、FAQ、切片、向量检索、检索日志、质量分析
- **Answerability Gate**：判断检索结果是否足以回答，不够就转人工 —— 防幻觉的好设计
- 工单系统：会话转工单、分类、指派、状态流转、进度记录、闭环
- 运行时支持 **Skills / MCP / OpenAI 兼容模型接口**
- 管理后台 + 客户侧聊天页 + 可嵌入 SDK

**魔改切口（具体到地方）**

| # | 要改什么 | 改哪里 | 难度 |
|---|---|---|---|
| 1 | **定位反转**：它是"AI 先答→人工兜底"，我们要"AI 只给建议→人工回复" | AI reply runtime，关掉自动回复，改成生成候选 | 中 |
| 2 | **右侧加插件容器** | `web/app/dashboard/conversations/`（Agent Workspace 页面）插入右栏 | 低 |
| 3 | **换数据源**：把我们 8 张表映射到它的 conversation / ticket 模型 | `internal/models/` + `internal/migration/` | 中 |
| 4 | **关掉 Qdrant**：我们的检索量小，用 LanceDB 或纯规则 | `.env.example` + `internal/ai/` | 低 |
| 5 | 砍掉不需要的：多渠道接入（Discord 等）、设备控制 | 按模块剔除 | 低 |

**代价与风险**
- **Go 后端**：队里没人会 Go，后端就只能改配置不能改逻辑 —— **这是最大的决策点**
- **155★ 的小众项目**：出问题没有社区答案，只能读源码
- **定位相反**：它是"AI 替代人工"，我们是"AI 辅助人工"，改的是它的核心链路 `[推测]` 不是小改
- 它没有"风险预警看板"，看板仍需自建

**判断**：`[推测]` 如果队里有 1 个会 Go 的人 → 值得 fork，能省掉整个工作台界面 + 知识库 + 工单模块的从零开发；如果没有 → **只用它的前端（Next.js 部分），后端自己写 Python，两者用 API 对接**。

---

## 二、Tier B：抄架构，不抄代码

| 项目 | 抄什么 | 为什么 |
|---|---|---|
| `meomeolucifr/agentic-support-suite` | **Orchestrator + 4 Agent 分工**（Router 分类 / Knowledge 检索 / Sentiment 情绪 / Decision 决策） | 跟我们的 S1 理解 / S2 风险 / S3 决策**几乎一模一样** → 拿它验证我们的分工是对的；同时抄它的 prompt 分工方式与置信度设计 |
| `chattermate/chattermate.chat`（Apache-2.0） | **AI Ticketing 的 guardrail 实现**：SQL 连接器强制只读、查询解析成 AST、非单条 SELECT 直接拒绝、表级白名单、强制 LIMIT、注入前剥离注释 | 这就是"防 LLM 幻觉"的工业级做法，直接搬进我们 S2 风险 Agent 的数据访问层 |
| `tgoai/tgo`（License `[待确认]`） | **UI Widget System**：把订单 / 物流 / 商品 / 比价渲染成卡片，用标准化 URI 协议做交互 | 正是我们"买家 360 全景卡"要的形态，可以直接照抄它的卡片定义 |

> ⚠️ `tgoai/tgo` 的 License 我只看到仓库里有 `LICENSE` 和 `LICENSE_CN` 文件，**未读到具体类型** `[待确认]`。且它是 10+ 微服务架构、最近提交 **2026-04-28**（比 agent-desk 旧 4 个多月）。**不建议 fork，只建议看设计**。

---

## 三、Tier C：直接用的零件

### 3.1 前端脚手架（选一个，只用它的布局/路由/表格）

| 项目 | 技术栈 | Star | 与我们的匹配度 |
|---|---|---|---|
| `ant-design/ant-design-pro` | React + Ant Design + UmiJS | 约 38.4k | 生态最全、文档最好；**如果 AgentDesk 用不上，这是首选** |
| `vbenjs/vue-vben-admin` | Vue3 + Vite + TS + Ant Design Vue | 约 32.7k | 功能最强、配置最灵活；学习成本最高 |
| `soybeanjs/soybean-admin` | Vue3 + Naive UI + UnoCSS | 约 14.5k | **上手最快、打包体积最小**；功能相对简单，适合赶工 |
| `PanJiaChen/vue-element-admin` | Vue2 + Element UI | 约 87k | ⚠️ Vue2，**已过时，别选了** |

### 3.2 中文情绪识别模型（离线、便宜、快）

| 模型 | 类别 | 特点 | 注意 |
|---|---|---|---|
| `Johnson8187/Chinese-Emotion-Small` | 8 类：平淡 / 关切 / 开心 / 愤怒 / 悲伤 / 疑问 / 惊奇 / 厌恶 | 基于 `MoritzLaurer/mDeBERTa-v3-base-mnli-xnli` 微调，README **明说适用于"客服情绪监控"** | 同作者有大参数版 `Chinese-Emotion`（560M） |
| `foxyanuo/chinese-chat-sentiment-8class` | 8 类：开心 / 难过 / 生气 / 惊讶 / 害怕 / 厌恶 / 中性 / 关心 | **上下文感知**（输入 `[上文]` 对话历史 + `[当前]` 目标消息）；ONNX FP16 格式、623.7 MB、**不需要 PyTorch** | 自报验证集 98.86% —— **高度可疑，必须自己实测** |
| `tabularisai/multilingual-emotion-classification` | 11 类多标签：anger/contempt/disgust/fear/frustration/gratitude/joy/love/neutral/sadness/surprise | XLM-R base，23 语言；**多标签**（能表达"frustration + sadness"混合情绪） | 自报 F1-micro 0.840，相对可信 |

**选型建议**：先用大模型（Qwen）出情绪基线，跑完 138 会话的指标；**只有当延迟或成本成为瓶颈时，才换小模型**。小模型的价值是"离线 + 便宜 + 20ms 级延迟"，不是"更准"。

### 3.3 RAG 后端（如果真要独立的知识库服务）

| 项目 | License | 资源要求 | 说明 |
|---|---|---|---|
| **RAGFlow** | **Apache-2.0**（四家里最宽松） | 最低 4 核 / 16GB / 50GB，Docker ≥24，**预构建镜像仅 x86** | 深度文档理解 + 多模态，最重但最开放 |
| MaxKB | GPLv3 | 4 核 / 8GB / 100GB | ⚠️ 社区版有功能限制：1 个成员、5 个应用、50 个知识库 |
| Dify | **修改版 Apache 2.0** | Docker Compose | ⚠️ 不能用源码运营多租户 SaaS；**去除前端品牌需商业授权** |
| FastGPT | **Apache 2.0 + 附加条件** | Docker Compose | ⚠️ **不得移除或修改控制台内的 LOGO 与版权信息**；不得用源码运营同类多租户 SaaS |

> **但我的判断：我们根本不需要 RAG 平台。** 要检索的只是"售后政策 + 商品信息 + 历史同类处置"，量级是几页文档。上 RAGFlow 要多花 16GB 内存和一个故障点，收益是负的。**关键词 + 规则匹配就够。**

---

## 四、Tier D：别碰（看着对口，实为坑）

| 项目 | 为什么别碰 |
|---|---|
| **Chatwoot**（MIT，约 27k★） | 技术栈 Ruby on Rails + Vue.js + PostgreSQL + Redis，最低 **8GB RAM（建议 16GB）**、60GB SSD、Ubuntu 22.04。要把它改成千牛工作台的样式，工作量约等于自己重写。**但可以偷它的数据模型设计**（inbox / conversation / message / label 的表结构）。 |
| **Dify / FastGPT** | 它们是为"**对话式知识库应用**"设计的，我们的核心是"**批处理式理解流水线**"（结构化输出 + 打分），形态不匹配。引入 = 多一层部署 + 多一层调试 + 演示多一个故障点。而且 FastGPT 明确禁止去掉控制台 LOGO。 |
| **所有语音 / Whisper 系客服分析项目** | 我搜到的 5 个"客服智能分析"开源项目（Customer-Service-Intelligence、Digital-Assistant-for-Call-Centers、AI-Powered-Call-Center-Intelligence 等）**全部是音频转写 + 语音情绪**。我们的数据是**纯文本聊天记录，没有任何音频**。一个都用不上 —— **别在这上面浪费时间**。 |
| `tgoai/tgo` 当底座 | 10+ 微服务（tgo-ai / api / cli / device-agent / device-control / platform / plugin-runtime / rag / web / workflow + 5 个 widget SDK），最近提交 2026-04-28。3 人 2 周扛不住。 |

---

## 五、License 红线（这条比选型更重要）

赛题原文要求：**"所有参赛代码、算法模型及方案必须确保原创性，不得侵犯第三方知识产权。"**

| 风险 | 后果 | 规避 |
|---|---|---|
| 用了开源但**不声明** | "原创性承诺"站不住，被发现直接出局 | 交付物里加**《开源组件与引用声明》**页 |
| 用了 **GPL 系**（如 MaxKB） | 传染性，衍生物需同样开源 | 比赛 Demo 无影响，但别当产品底座 |
| 用了**有品牌限制**的（FastGPT / Dify） | 私自去 LOGO 违约 | 别选它们当底座 |
| 用了 Apache/MIT 却声称"全自研" | 学术诚信问题 | 声明清楚，反而显得专业 |

**建议动作**：在 PPT 里加一页表——

| 开源项目 | License | 用在哪 | 我们改了什么 |
|---|---|---|---|
| agent-desk | Apache-2.0 | 工作台界面底座 | 反转 AI 回复链路、新增右侧插件栏、替换数据源 |
| soybean-admin | MIT | 前端脚手架 | 只保留布局与路由 |
| Chinese-Emotion-Small | 见仓库 | 情绪识别 | 直接调用 |

**这一页不是减分项，是加分项**——它证明团队懂开源规矩，比声称"全自研"可信得多。

---

## 六、我的推荐路线

**不 fork 整套系统。** 理由：把别人的系统改成千牛样式 + 换数据模型 + 换业务逻辑，工作量跟自己写差不多，还额外背上"读懂别人代码"和"被原架构绑住"两项成本。

**推荐：抄架构 + 用零件**

| 部分 | 做法 | 省 |
|---|---|---|
| 前端工作台 | 拿 `soybean-admin` 或 `ant-design-pro` 当脚手架，只保留布局 / 路由 / 表格，三块面板照千牛截图手写 | 2–3 人天 |
| Agent 后端 | **自研**（约 300 行 Python），架构照抄 `agentic-support-suite` 的 Orchestrator + 4 Agent 分工 | 1 人天 |
| 防幻觉数据层 | 照抄 `chattermate` 的 guardrail 思路（只读 + 白名单 + 强制 LIMIT） | 0.5 人天 |
| 买家全景卡 | 照抄 `tgo` 的 Widget 卡片形态（订单卡 / 物流卡 / 商品卡） | 1 人天 |
| 情绪识别 | 先用 Qwen 出基线，有瓶颈再换 `Chinese-Emotion-Small` | 0.5 人天 |
| 看板 | ECharts 手搭，不上 BI 平台 | 0 |
| 工作台界面 | **仅当你队里有会 Go 的人**，才考虑 fork `agent-desk` | 5–8 人天 |

**唯一值得真 fork 的是 `agent-desk`。** 判断条件只有一个：**队里有没有人会 Go。** 有 → fork，省最多；没有 → 只取它的前端（Next.js + React），后端自己写。

---

## 七、下一步

| # | 要做的验证 | 花多久 | 产出 |
|---|---|---|---|
| 1 | clone `agent-desk`，Docker Compose 跑起来，看 Agent Workspace 实际长什么样 | 1–2 小时 | 从"听说"变"确定"：它的界面能不能塞我们的数据 |
| 2 | 在 138 个会话上实测 `Chinese-Emotion-Small`，跟 Qwen 对比 | 2 小时 | 情绪识别选型的真实数据（而不是自报的 98.86%） |
| 3 | 查清 `tgoai/tgo` 的 License 类型 | 10 分钟 | 决定它能不能被引用 |

---
---

# 补充篇：Java / Python 主流框架路线

> 追加时间：2026-09-15 23:10 ｜ 触发原因：**上一轮推荐的 `agent-desk` 是 Go 技术栈。如果队伍是 Java / Python 背景，那条建议作废，改用本篇。**

## 八、结论

| 语言 | 有没有"工作台级"的开源项目 | 结论 |
|---|---|---|
| **Java** | ✅ **有，而且很对口** | `Bytedesk 微语`（Spring Boot + React）自带客服工作台、工单 SLA、知识库、AI Agent。**这是目前最接近"能直接魔改"的项目** |
| **Python** | ❌ **没有** | 我搜到的全部是个人小项目（几十～几百 star）。Python 的强项在 **AI 编排层**，不在业务工作台 |

**修正后的推荐**：
- **有 Java 能力** → 走 `Bytedesk` 当底座 + `Spring AI Alibaba` 做编排（**阿里官方，直连 Qwen + 百炼**）
- **Python 能力** → 自研 FastAPI 后端（架构抄 LangGraph 多 Agent 流水线）+ 现成前端脚手架

---

## 九、Java 路线

### 9.1 业务底座：`Bytedesk 微语` —— 目前最对口的候选

| 项 | 内容 | 确定性 |
|---|---|---|
| 仓库 | `https://github.com/Bytedesk/bytedesk` | `[确定]` |
| License | **AGPL-3.0**（主仓库 README 明确写 GNU AFFERO GENERAL PUBLIC LICENSE v3） | `[确定]` |
| 后端 | **Spring Boot**（Maven 多模块单体仓库，根 `pom.xml`） | `[确定]` |
| 前端 | React（`frontend/` 目录）；另有 React Native 移动端、Electron 桌面端 | `[确定]` |
| 版本 | 早期 README 写 `springboot-3.2.0`；`channels/` 的提交信息显示 "upgrade main to **2.0.0 on Spring Boot 4.0.2**" → **具体分支版本需拉下来确认** | `[待确认]` |
| 依赖 | MySQL / PostgreSQL / Oracle + **Redis + Elasticsearch + Artemis 或 RabbitMQ** | `[确定]` |
| 部署 | Docker Compose（**有独立仓库 `Bytedesk/bytedesk-docker-compose`**，可只下镜像配置、不必 clone 全仓库）／宝塔／源码 | `[确定]` |
| 端口 | 9003（另有 9885） | `[确定]` |
| 活跃度 | `deploy/` 最后提交 **2026-09-15**（检索当天）；README 更新 2026-08-29 | `[确定]` |
| Star 数 | README 里是动态 badge 图片，未抓到具体数字 | `[待确认]` |

**它自带的、正是我们要的模块**

| 模块 | 内容 |
|---|---|
| **Customer Service** | 多渠道接入、多种路由策略、考核指标、**Seating workbench（客服工作台）** ← 就是千牛那个"客服工作台"的对应物 |
| **Ticket（工单）** | 工单管理、**工单 SLA 管理**、工单统计与报表 ← 我们的"闭环跟踪看板"数据源 |
| **Knowledge Base** | 内部文档 / 帮助中心 / FAQ |
| **AI Agent** | 对接 Ollama / DeepSeek / ZhipuAI 等、**RAG 知识库问答**、Function Calling、**MCP** |
| **Workflow** | 自定义表单 / 流程 / **工单流程可视化** |
| TeamIM | 多级组织架构、角色与权限、聊天记录监控 |
| Open Platform | 完整 RESTful API + 多语言 SDK |

**⚠️ AGPL-3.0 意味着什么（必须看懂）**

| 场景 | 是否可行 |
|---|---|
| 拿去打比赛、做 Demo、录视频、现场演示 | ✅ **没问题**（未对外提供网络服务、未分发） |
| 交源码 zip 给评委会 | ✅ 可以，**但必须声明**用了 Bytedesk 且是 AGPL-3.0 |
| 赛后再拿它当产品 / 部署成 SaaS 卖给客户 | ❌ **不行**。AGPL 有强传染性；且项目方自己声明「仅支持企业内部员工自用，**严禁用于销售、二次销售或者部署 SaaS 方式销售**」 |
| 修改后闭源分发 | ❌ AGPL 要求衍生作品同样以 AGPL 开源 |

→ **比赛用可以，但要在《开源组件与引用声明》里写清楚，并且明确"本项目为赛事用途，不作商业分发"。** 这句话写上去，比藏着强。

### 9.2 大模型编排层：四个候选

| 框架 | 定位 | 规模 | 关键事实 | 状态 |
|---|---|---|---|---|
| **Spring AI Alibaba** | 阿里官方，深度集成**通义千问 Qwen + 阿里云百炼**，兼容 Spring AI 标准 API | — | **跟我们的技术选型（百炼 + Qwen）完美对齐，且直接呼应赛题"鼓励使用阿里开源系列大模型"** | `[确定]` 存在，GitHub `alibaba/spring-ai-alibaba` |
| **Spring AI** | Spring 官方 AI 抽象层，强依赖 Spring Boot | 约 9,000★ | 1.0 于 **2025/5 GA**；1.1（**2025/11**）加入 **MCP 集成 + Advisors API + 20+ 模型后端**；**2.0.0-M2 于 2026/1，基于 Spring Boot 4**。需 **Java 17+**，2.0 需 **21+** | `[确定]` 版本脉络 |
| **LangChain4j** | 框架中立（Spring / Quarkus / Micronaut / 纯 Java 都能用） | 约 11,800★（另一来源写 5k+，**有出入**） | v1.13.1，**30+ 向量存储**，RAG 能力三方最强 | `[待确认]` star 数 |
| **AgentScope Java 2.0** | 阿里开源，生产级 Agent 平台 | — | 分布式部署、多层记忆、多 Agent 编排、HITL（人在环） | `[确定]` 存在 |

> 另有 `Solon AI`（国产，框架中立，约 1,000★）和 `DJL`（AWS，强在本地模型推理）、`Semantic Kernel`（微软，跨平台）。

**选型建议**：**首选 `Spring AI Alibaba`**——理由不是技术最强，而是**它同时满足三件事**：① 阿里官方，跟百炼/Qwen 零摩擦 ② 赛题明确鼓励阿里开源模型，用它等于把"技术选型"和"评分点"绑在一起 ③ 兼容 Spring AI API，万一要换回标准 Spring AI 成本很低。

### 9.3 Java 路线的代价

| 代价 | 说明 |
|---|---|
| **依赖重** | 要同时起 Redis + Elasticsearch + MySQL + 消息队列（Artemis/RabbitMQ）。3 人 2 周里，光"把环境跑起来 + 摸清 Bytedesk 的模块边界"就可能吃掉 3–5 人天 |
| **AGPL 传** 染 | 见 9.1 的表格 |
| **魔改点不明** | Bytedesk 是"AI 先答 + 人工接待"的完整产品，我们要的是"AI 只给建议"。要改哪一层、改动多大，**必须拉下来读源码才知道** `[待确认]` |
| 社区规模 | 中等偏小，遇到问题主要靠自己读源码 |

---

## 十、Python 路线

### 10.1 残酷事实：没有工作台级项目

我按"Python + 客服 + 工单"搜了一圈，**全部是个人小项目**，没有一个能当底座。但它们的设计可以直接抄——**抄架构比抄代码便宜得多**。

| 项目 | 技术栈 | 值得抄什么 | 规模 |
|---|---|---|---|
| `MasirJafri1/Customer-Support-Agent` | FastAPI + **LangChain + LangGraph** + SQLAlchemy + SQLite + Pydantic；React 双端（客户门户 + 员工看板） | ⭐ **8 节点 Agent 流水线**：Validation → Categorization → Sentiment → Priority → Response → Suggestion → **Re-evaluation → Escalation**。跟我们 S1/S2/S3 的思路一致，而且多了"再评估"和"升级决策"两个我们方案里还没有的环节 | 小 |
| `abdulrehman411/ai-customer-support` | FastAPI + Next.js 16 + **ChromaDB** + sentence-transformers(`all-MiniLM-L6-v2`) + Groq；**MIT License**；有 Docker Compose | ⭐ ① **RAG 带 citation（引用来源）** ② 意图分类 + 优先级打分 + **情感分析** ③ 回复生成带**语气控制 + 置信度** ④ **人工审核工作流**（inline 编辑 + 审批）。这四点几乎是我们插件模块的完整功能清单 | 小 |
| `zrn2003/AI-customer-sev` | **Django + DRF** + React(Vite) + PostgreSQL + scikit-learn | ① 用 **scikit-learn 做严重度打分（1–10）**并估算处理时长 ② Admin Console **按 AI 严重度排序**工单 ③ **Audit Logging**（所有状态变更留痕） | 小 |
| `vetrinanda/Customer-Support-Agent` | FastAPI + **LangGraph** + Google Gemini 2.5 Flash + React 19 + Vite 8 + Tailwind 4 + shadcn/ui + uv | **路由式 Agent 状态机**：categorize → analyse_sentiment → **conditional route**（负面情绪自动升级人工）→ 分类 handler | 小 |

**共同点（也是共识）**：都是 FastAPI 或 Django + 关系库 + 一个轻量向量库 + LangGraph 编排。**没有一个用重型框架**——这反过来验证了我们"不上 LangGraph 之外的编排平台"的判断。

### 10.2 Python 真正的强项：AI 编排层

| 项目 | License | 说明 |
|---|---|---|
| **LangGraph / LangChain** | MIT | 上面 4 个项目里 2 个用它。Python 侧事实标准 |
| **DB-GPT** | MIT | AI 原生数据应用框架，含 AWEL、Multi-Agents、Text2SQL |
| **Quivr** | Apache-2.0 | 多模型 RAG 平台 |
| **PrivateGPT** | Apache-2.0 | 本地私有化 |
| **RAGFlow** | **Apache-2.0**（四家里最宽松） | 深度文档理解 + 多模态；最低 4核/16GB/50GB，**预构建镜像仅 x86** |
| **Dify** | 修改版 Apache 2.0 | ⚠️ 不能用源码运营多租户 SaaS；**去除前端品牌需商业授权** |

> 再强调一遍第六节的判断：**我们不需要 RAG 平台。** 要检索的只是售后政策 + 商品信息，几页文档，关键词匹配就够。上面这些列出来是给你做技术选型时的备查，不是推荐用。

---

## 十一、两条路线对比与最终推荐

| 维度 | Java 路线（Bytedesk） | Python 路线（自研） |
|---|---|---|
| 有没有现成工作台 | ✅ 有，而且带工单 SLA | ❌ 没有，要自己写 |
| 上手速度 | ❌ 慢（依赖重、要摸清模块边界） | ✅ 快（FastAPI 半天起服务） |
| 与我们的数据模型贴合度 | ⚠️ 要改（它的 conversation/ticket 模型 ≠ 我们 8 张表） | ✅ 高（直接对着官方字段建表） |
| AI 编排 | ✅ Spring AI Alibaba / LangChain4j | ✅✅ LangGraph（生态最成熟，且是我们方案里写的） |
| 与赛题契合 | ✅ Spring AI Alibaba 直连 Qwen + 百炼 | ✅ 调百炼 API 同样可行 |
| License 风险 | ⚠️ **AGPL-3.0** | ✅ MIT / Apache 为主 |
| 演示可靠性 | ⚠️ 多中间件 = 多故障点 | ✅ 单进程 + SQLite，稳 |
| 适合的队伍 | **有 Java 主力、想要现成工作台** | **有 Python 主力、想快、想完全可控** |

### 我的推荐（分情况）

| 你的队伍 | 推荐 |
|---|---|
| **有 Java 主力 + 时间充裕** | fork **Bytedesk**，只保留「Customer Service + Ticket + Knowledge Base」三块，砍掉 Call Center / Video / TeamIM；AI 层用 **Spring AI Alibaba** 接百炼 Qwen |
| **有 Python 主力** | **自研**。FastAPI + SQLite + Pydantic v2；架构照抄 `MasirJafri1` 的 8 节点 LangGraph 流水线；功能清单照抄 `abdulrehman411`（RAG citation + 语气控制 + 置信度 + 人工审核）；前端拿 `soybean-admin` 只留布局路由 |
| **两种都有** | **Java 做业务后台（工单/工作台）+ Python 做 AI 服务**，中间用 REST 对接。代价是多一个进程、多一个对接面，但各取所长 |
| **都不会，只会前端** | 走 Python，让 AI 全程辅助写后端 |

**一句话**：**Java 那条路的诱惑是"现成工作台"，代价是 AGPL + 重依赖 + 未知的魔改量；Python 那条路的代价是"从零写工作台"，但换来完全可控 + 单进程演示稳。** 就我们 25 人天的预算和"现场要跑得起来"的硬要求，`[推测]` **Python 自研 + 抄架构的概率更高、风险更小**——除非队里有一个能两天内把 Bytedesk 跑通的 Java 主力。

---

## 十二、本篇新增的待办

| # | 事项 | 花多久 |
|---|---|---|
| 1 | 拉 `Bytedesk` 源码，确认它当前分支的 Spring Boot 版本、模块边界、以及"AI 先答"要改成"AI 建议"要动哪一层 | 半天 |
| 2 | 确认参赛资格与 Java/Python 人力（决定走哪条路） | — |
| 3 | 若走 Java：先跑 `bytedesk-docker-compose` 验证环境能不能起来（比 clone 全仓库快） | 1 小时 |
| 4 | 若走 Python：把 `abdulrehman411/ai-customer-support` 拉下来跑通，抄它的 RAG citation 与人工审核工作流 | 2 小时 |

---

## 十三、若依（RuoYi）方案评估 —— 能不能直接拿它魔改？

> 追加时间：2026-09-15 23:20 ｜ 结论：**可以，而且对 Java 队伍可能是最优起点。但必须把"魔改"这个词拆开看——若依给你"壳"，给不了"业务"。**

### 13.1 若依基本事实

| 项 | 内容 | 确定性 |
|---|---|---|
| License | **MIT** —— 作者只保留版权，无其他限制（个人企业均可免费商用，需在发行版内保留原许可声明） | `[确定]` |
| 官网 | `ruoyi.vip`，**2026-03 仍在更新** | `[确定]` |
| 官方版本 | **RuoYi** 单体不分离 v4.8.3（2026-03-23）｜**RuoYi-Vue** 前后端分离 v3.9.2（2026-03-26）｜**RuoYi-Cloud** 微服务 v3.6.8（2026-03-30）｜RuoYi-App（UniApp） | `[确定]` |
| 技术栈 | Spring Boot + Spring Security/Shiro + MyBatis + JWT + **Vue2 + Element UI**；**MySQL + Redis** | `[确定]` |
| 功能 | 用户/角色/菜单/部门/岗位/字典/参数/公告 · 操作日志/登录日志/在线用户/SQL 监控/定时任务 · **代码生成器**（单表 / 树表 / 主子表）+ Knife4j + 表单构建器 | `[确定]` |
| 生态 | 另有 RuoYi-Python / RuoYi-Net / RuoYi-Go / RuoYi-PHP / RuoYi-React / RuoYi-Antdv / RuoYi-Cms / RuoYi-Activiti | `[确定]` |
| 社区魔改版 | **RuoYi-Vue-Plus**（`dromara`，MyBatis-Plus + Sa-Token，JDK 17/21，Vue3 + TS，支持多租户）｜**RuoYi-Vue-Pro**（芋道，内置 BPM 工作流 + 商城 + 支付 + 公众号，**"半成品 SaaS"，代码量巨大**） | `[确定]` |

### 13.2 ⚠️ 第一个坑：版本兼容性（选错版本白干一周）

**Spring AI 硬性要求 JDK 17+ 和 Spring Boot 3.x。**

| 你的选择 | 能不能挂 Spring AI |
|---|---|
| **RuoYi-Vue v3.9.2（官方默认，教程最多）** | ❌ 默认 **Spring Boot 2.x / JDK 8**，强上要先升 JDK + Spring Boot，**光这一步可能折腾一周** |
| **RuoYi-Vue-Plus / RuoYi-Cloud-Plus** | ✅ 已经是 Spring Boot 3.x + JDK 17+，**直接用** |
| 老的 RuoYi-Vue（SB2） | ⚠️ 官方建议：**新建一个独立的 Spring Boot 3 模块**专跑 AI 服务，不要硬升框架 |
| **RuoYi-Cloud（官方微服务版）** | ❌ **别碰**。Nacos + Gateway + Sentinel + Seata，3 人 2 周扛不住 |

→ **结论：选 `RuoYi-Vue-Plus`（dromara）当起点，或者直接用社区已有的 Spring Boot 3 分支。**

**现成的起步点** `[推测]`（来自技术博客，需自行核实仓库是否仍在）：有开发者维护了已集成 Spring AI Alibaba 的若依分支（如 `springboot3dev` 分支），拉下来可直接改。用前先确认活跃度。

### 13.3 若依给你什么、不给什么

| ✅ 它给（白送） | ❌ 它不给（全要自己写） |
|---|---|
| 登录 / RBAC 权限 / 菜单 / 字典 / 日志 | **会话列表与消息流（聊天气泡）** |
| 后台布局、表格、分页、表单、弹窗 | **右侧插件容器**（我们的设计目标位） |
| 接口文档（Knife4j）、监控 | **意图/情绪/风险判断链路** |
| **代码生成器**：建好表一键生成前后端 CRUD | **风险评分算法与规则引擎** |
| 定时任务（可用于批量跑会话分析） | **所有客服业务逻辑**（它一个都没有） |

**关键判断**：**若依没有任何客服、会话、工单、风控模块。** 所以"用若依魔改出客服系统"这个说法不成立——真实情况是"**用若依当脚手架，自研客服辅助系统**"。

📌 **这句话必须写进 PPT**。评委里如果有技术背景的人，"基于若依魔改"和"基于若依脚手架自研"是两种完全不同的评价。

### 13.4 若依 vs Bytedesk：本质区别

| 维度 | **Bytedesk** | **若依（RuoYi-Vue-Plus）** |
|---|---|---|
| 有业务模块吗 | ✅ 有（客服工作台 / 工单 SLA / 知识库 / AI Agent） | ❌ 没有，纯脚手架 |
| License | ⚠️ **AGPL-3.0**（禁 SaaS 化、禁去品牌） | ✅ **MIT**（只需保留版权声明） |
| 依赖 | MySQL + Redis + **Elasticsearch** + MQ | **MySQL + Redis**（轻） |
| 上手难度 | 要摸清别人的模块边界 | 国内教程最多、坑最少 |
| "魔改"的性质 | 改**别人的业务**（工作量未知） | 写**自己的业务**（工作量已知） |
| 数据模型贴合度 | ⚠️ 要映射到它的 conversation/ticket | ✅ 直接对着官方 8 张表建模 |
| 适合谁 | 想要现成工作台、能接受 AGPL | 想要干净地基、要 License 自由 |

**一句话**：**Bytedesk 是"别人盖好的房子，你得改户型还得接受 AGPL"；若依是"毛坯房 + 全套工具"，业务自己盖但地基干净。** 对我们这种"业务逻辑很特殊（AI 只辅助不替代、风险分级、证据校验）"的赛题，`[推测]` **毛坯房可能更快**——因为不需要先理解一套复杂的既有业务，再想办法把它扭过来。

### 13.5 已知的坑（别人踩过的，直接抄答案）

| 坑 | 现象 | 解法 |
|---|---|---|
| **权限上下文丢失** ⭐ 最坑 | 在若依里用 Spring AI 的 **Tool Calling** 时抛 `No thread-bound request found`。原因：若依用 **ThreadLocal**（`SecurityUtils.getLoginUser()` / `RequestContextHolder`）存会话上下文，而 Spring AI 的工具调用跑在**独立异步线程**（ExecutorService 或 Reactor），继承不到 ThreadLocal，于是 `DataScopeAspect` 拿不到上下文就报错 | 抽象"**双模上下文持有者**"，同时支持同步 ThreadLocal（MVC）与异步 Reactive Context（WebFlux） |
| 版本不兼容 | 老若依（SB2/JDK8）挂不上 Spring AI | 见 13.2，选 Vue-Plus 或独立 SB3 模块 |
| base-url 写错 | 404 | **不要**在 base-url 里多写 `/v1/chat/completions`，Spring AI 底层会自动拼 |
| Nginx 504 | AI 流式响应超时 | 把 `proxy_read_timeout` 调大（如 300s） |
| 中文乱码 | 流式输出乱码 | Controller 的 `produces` 明确写 `charset=UTF-8` |
| API Key 泄露 | — | 别写死代码里，尤其别提交到公开仓库 |

### 13.6 工作量重算

| 项 | 原估 | 用若依后 |
|---|---|---|
| P4 前端（工作台 + 插件 + 看板） | 6 人天 | **3–4 人天** |
| 其中省下的 | — | 布局/菜单/权限/登录/路由/表格组件 ≈ 2–3 人天；**8 张表的 CRUD 页面靠代码生成器**≈ 1–2 人天 |
| 其中没省下的 | — | 聊天界面（消息流+气泡）≈ 1 人天；右侧插件容器 ≈ 1 人天；看板 ECharts ≈ 1.5 人天 |
| 新增踩坑成本 | — | 权限上下文 + 版本兼容 + 布局改造 ≈ 0.5–1 人天 |
| **净收益** | | **约省 2–3 人天 → 总工作量 25 降到 22–23 人天** |

> 注意：净收益没有第一眼看上去那么大，因为**若依换不掉"业务自研"这部分**。它省的是"壳"，而我们最花时间的从来不是壳。

### 13.7 用之前必须先验证的一件事

**若依默认布局是「顶栏 + 左侧菜单 + 内容区」；我们要的是千牛那种「左会话列表 + 中聊天窗口 + 右插件栏」三栏布局。**

**能不能改造这个布局，决定若依的价值上限。** 如果改不动，若依就只剩"代码生成器 + 权限"两块价值，净省可能只有 1 人天。

→ **验证动作（1 小时）**：拉 `RuoYi-Vue-Plus`，跑起来，然后尝试把主内容区改成三栏。改得动 → 用；改不动 → 回 Python 自研。

### 13.8 更新后的推荐（覆盖第十一节）

| 你的队伍 | 推荐 | 理由 |
|---|---|---|
| **有 Java 主力** | ✅ **`RuoYi-Vue-Plus` 当脚手架 + `Spring AI Alibaba` 接百炼 Qwen**，业务全自研 | MIT + 轻依赖 + 代码生成器 + 教程最多。**比 Bytedesk 更适合我们**，除非你们想要现成的客服业务 |
| 有 Java 主力、且不想写业务 | Bytedesk | 代价是 AGPL + 重依赖 + 要把"AI 先答"扭成"AI 建议" |
| **有 Python 主力** | 自研 FastAPI + 抄 LangGraph 架构；前端 `soybean-admin` | 单进程演示稳、License 干净 |

**若依是这一轮最实用的发现。** 如果你是 Java 队伍，`[推测]` 我建议从它起步——**前提是先花 1 小时验证三栏布局改不改得动。**


