# AgentDesk · 数据共情者（客服 AI 管家）项目审查指引

> **参赛团队**：Star of CAU（中国农业大学）  
> **赛题方向**：欧莱雅第二届美妆科技黑客松 · 赛题1「数据共情者——消费者的 AI 管家」  
> **工程基础**：基于开源项目 `agent-desk` (Apache-2.0) 进行工程级深度魔改与业务落地  
> **交付日期**：2026-10-15  
> **本审查包用途**：供网页端 AI（如 Claude / ChatGPT / Kimi / 评委审查终端）进行全仓代码审计、架构审查与业务逻辑评估。

---

## 〇、三分钟快速审查结论

1. **业务站位与核心创新**：
   - 摒弃传统机械应答机器人，定位为千牛客服的“即时决策外挂”（递小纸条，而非替代发声）；
   - 在坐席工作台（`/workbench`）右侧辅助区实现**四大智能抽屉**（会话洞察、客户全景、回复建议、会话信息），全景打通对话、订单、工单三向上下文；
   - 建立客服风险监控看板（`/dashboard/risk-board`），实现最高危（critical）会话秒级预警与主管接管拦截。
2. **核心算法与评测指标**：
   - 构建了涵盖美妆客服全流程的 **10 大类、41 小类** 细粒度场景体系与风险矩阵；
   - 离线意图识别与风险研判准确率达到 **93.13%**，macro-F1 达到 **0.937**；
   - 官方数据集全量覆盖检验（138 个闭环会话、998 条消息、113 个订单、80 张工单），精准捕获 10 例最高危风险会话。
3. **架构解耦与存算分离（应对“离线数据 vs 实时接收”审查问答）**：
   - 采用适配器模式（Adapter Pattern），将最上层的“数据接入”与底层的“业务与智能决策”彻底解耦；
   - **离线管道（ETL Pipeline）**：用于离线数据清洗、三向主键关联（零 AI 成本，纯代码毫秒级 Join）、构建 Golden Benchmark；
   - **在线接入（Production Serving）**：只需将 ETL 数据源替换为消息网关（千牛 JS-API / 企微 Webhook / MQ），下层会话上下文聚合模型与大模型决策链**无需修改一行代码**。

---

## 一、项目核心目录结构与审查导航

```text
├── 00-项目审查指引与架构总览.md      <-- 本文件（审查总纲与导读）
├── PRD-数据共情者-千牛插件-V1.0.md      <-- 详细产品需求文档（业务背景、交互规范、三栏设计）
├── README-CAU.md                       <-- 项目总体入口与导航
├── CHANGELOG-CAU.md                    <-- 完整的 18 轮实战 Bug 修复与迭代演进记录
├── 千牛插件形态与数据通路说明.md        <-- 答辩与审查必读（形态对照、千牛资格门槛分析、数据解耦通路）
├── 架构与迭代维护规划.md                <-- 软件架构设计与可维护性设计
├── 数据共情者-完赛设计思路-V1.0.md      <-- 整体算法思路、场景体系设计
├── 开发准则与协作规范.md                <-- 团队代码准则与交付标准
├── 运行手册.md                          <-- 运行指导手册
├── 教育文件/                            <-- 面向零基础的产品通俗白话解读
├── scripts/                            <-- 数据工程管道与自检脚本
│   ├── pipeline.py                     <-- 核心清洗与三向关联流水线
│   ├── seed_risk_db.py                 <-- 风险模型评估与数据库注入
│   ├── export_eval_set.py              <-- 评估集抽取
│   └── selfcheck.sh                    <-- 一键全流程自检脚本
├── agent-desk/                         <-- 核心工程源码仓库
│   ├── cmd/server/main.go              <-- 服务端启动入口
│   ├── config/config.yaml              <-- 核心配置文件
│   ├── data/agentdesk.db               <-- 内置 SQLite 种子数据库（含全量清洗与标注数据）
│   ├── internal/                       <-- Go 后端核心逻辑
│   │   ├── models/cs_models.go         <-- [AgentDesk] 客服会话上下文、风险事件、研判模型 DTO
│   │   ├── services/cs_risk_service.go <-- [AgentDesk] 风险研判与预警引擎核心服务
│   │   ├── services/cs_evaluation_service.go <-- [AgentDesk] 评估推理与指标度量服务
│   │   ├── services/cs_reply_service.go<-- [AgentDesk] 话术建议与策略生成服务
│   │   ├── services/cs_scene_classifier.go<-- [AgentDesk] 10大类41小类场景意图分类器
│   │   ├── handlers/dashboard/cs_risk_handler.go <-- [AgentDesk] 风险看板与抽屉接口 Handler
│   │   ├── pkg/ginx/static.go          <-- [AgentDesk] 静态单文件路由回退与 Next.js 切片容错
│   │   └── handlers/api/auth_handler.go<-- [AgentDesk] 开放注册与权限扩展
│   └── web/                            <-- Next.js 16 + React 19 前端工程
│       ├── app/workbench/page.tsx      <-- 坐席工作台三栏形态入口（对应千牛）
│       ├── app/dashboard/risk-board/   <-- 客服风险监控与预警大屏看板
│       ├── components/conversation-workbench.tsx <-- 工作台三栏布局 + 右侧四大智能抽屉核心组件
│       ├── components/login-form.tsx   <-- 登录组件（硬导航与静态切片防死锁）
│       ├── components/register-form.tsx<-- 注册组件（双向互通与即时校验）
│       └── messages/zh-CN.json         <-- i18n 国际化语言包（含全套客服业务词条）
```

---

## 二、关键技术点与代码位置索引

### 1. 业务上下文聚合与三向 Join（零成本确定性计算）
- **文件**：`agent-desk/internal/models/cs_models.go`
- **解析**：定义 `CSSessionContext`，聚合了当前买家在聊天流中的上下文、关联订单及历史售后工单，由确定性代码秒级拼装，不浪费昂贵的大模型 Token。

### 2. 风险识别与场景意图分类引擎
- **文件**：`agent-desk/internal/services/cs_risk_service.go` 与 `cs_scene_classifier.go`
- **解析**：针对美妆领域特有的“使用后过敏、泵头损坏、物流停滞、虚假发货、差评投诉”等场景，建立分级规则树与动态提示词链路。

### 3. 千牛工作台右侧四抽屉交互
- **文件**：`agent-desk/web/components/conversation-workbench.tsx`
- **解析**：通过 `ResizablePanelGroup` 弹性分栏，在客服工作台右侧提供 `会话洞察`、`客户全景`、`回复建议`、`会话信息` 四大折叠抽屉，即时辅助客服决策。

### 4. 单文件全嵌入式交付与 Next.js 静态切片优化
- **文件**：`agent-desk/internal/pkg/ginx/static.go`
- **解析**：解决 Next.js 16 静态导出在 SPA 模式下因 RSC `*.txt` 切片拦截引起的路由死锁，实现了 Go 服务单二进制内嵌（86MB），双击即可运行。

---

## 三、审查重点关注建议

1. **业务契合度**：请对比官方赛题文档（`交接包-欧莱雅黑客松/07-赛题说明原文.txt`），审查四抽屉功能是否严丝合缝地击中“不干扰主聊天、给客服递纸条”的定位要求。
2. **架构可扩展性**：请审查 `千牛插件形态与数据通路说明.md` 第四节，验证从当前离线 Demo 演进到千牛正式插件的数据解耦设计。
3. **工程质量与防御性编程**：请查阅 `CHANGELOG-CAU.md`，全过程真实记录了 18 轮实战中遇到的 MySQL/SQLite 方言兼容、GORM 错误检查、并发锁竞争、Next.js 客户端路由切片解析等关键工程排错过程。
