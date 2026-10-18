package dashboard

import (
	"strings"

	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 风险看板接口
//
// 路由挂载（见 bootstrap/routes.go 的 registerDashboardCSRoutes）：
//   GET  /api/dashboard/cs-risk/board          风险看板：统计 + 列表
//   GET  /api/dashboard/cs-risk/session/:no    单会话全景 + 风险明细
//   POST /api/dashboard/cs-risk/handle         标记风险已处置（闭环）
//
// 设计取向：
//   board 一次性返回统计与列表 —— 看板页首屏只发一个请求，
//   Demo 现场不会因为网络/并发出现半屏空白。
// ============================================================================

// CSRiskGetBoard 风险看板数据。
//
// query:
//
//	levels  风险等级过滤，逗号分隔，如 "critical,high"
//	scene   大类场景过滤，如 "不良反应"
//	keyword 买家昵称 / 会话号模糊搜索
//	limit   返回条数，默认 200
func CSRiskGetBoard(ctx *gin.Context) {
	levelsRaw, _ := params.Get(ctx, "levels")
	scene, _ := params.Get(ctx, "scene")
	keyword, _ := params.Get(ctx, "keyword")
	limit, _ := params.GetInt(ctx, "limit")

	var levels []string
	for _, s := range strings.Split(levelsRaw, ",") {
		if v := strings.TrimSpace(s); v != "" {
			levels = append(levels, v)
		}
	}

	stats := services.CSRiskService.BuildRiskBoardStats()
	list := services.CSRiskService.ListContexts(levels, scene, keyword, limit)

	httpx.WriteJSON(ctx, gin.H{
		"stats": stats,
		"list":  list,
	})
}

// CSRiskGetSession 单会话全景 + 风险明细（按官方 session_no）。
func CSRiskGetSession(ctx *gin.Context) {
	sessionNo := strings.TrimSpace(ctx.Param("no"))
	if sessionNo == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	overview := services.CSRiskService.GetContextBySessionNo(sessionNo)
	if overview == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0116"))
		return
	}
	httpx.WriteJSON(ctx, gin.H{
		"context": overview,
		"risks":   services.CSRiskService.ListEvents(sessionNo),
	})
}

// CSRiskGetByConversation 按站内会话ID取全景 + 风险明细。
//
// 这是工作台右侧辅佐面板调用的主接口 —— 选中会话即刷新一次。
//
// 重要：查不到数据时返回 200 + context=null，而不是错误。
// 原因：agent-desk 里绝大多数会话并没有预置的风险数据（只有赛题导入的
// 138 条有），如果这里回错误码，前端每次点开会话都会弹 toast，
// 演示现场会满屏报错。前端约定 context 为 null 时渲染「暂无洞察」空态。
func CSRiskGetByConversation(ctx *gin.Context) {
	conversationID, _ := params.GetInt64(ctx, "conversationId")
	if conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}
	overview := services.CSRiskService.GetContextByConversationID(conversationID)
	if overview == nil {
		httpx.WriteJSON(ctx, gin.H{
			"context": nil,
			"risks":   nil,
		})
		return
	}
	httpx.WriteJSON(ctx, gin.H{
		"context": overview,
		"risks":   services.CSRiskService.ListEventsByConversationID(conversationID),
	})
}

// CSRiskPostHandle 标记风险事件已处置。
//
// body: { "sessionNo": "S00010" }  —— 该会话下全部风险置为已处理
// 或    { "conversationId": 123 }   —— 站内会话处置全部风险
// 或    { "eventId": 12 }           —— 只处理单条
func CSRiskPostHandle(ctx *gin.Context) {
	var req struct {
		SessionNo      string `json:"sessionNo"`
		ConversationID int64  `json:"conversationId"`
		EventID        int64  `json:"eventId"`
	}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.SessionNo == "" && req.ConversationID > 0 {
		if c := services.CSRiskService.GetContextByConversationID(req.ConversationID); c != nil {
			req.SessionNo = c.SessionNo
		}
	}
	var handled int64
	if req.SessionNo != "" || req.EventID > 0 {
		handled = services.CSRiskService.MarkHandled(req.SessionNo, req.EventID)
	} else if req.ConversationID > 0 {
		handled = services.CSRiskService.MarkHandledByConversationID(req.ConversationID)
	}
	httpx.WriteJSON(ctx, gin.H{"handled": handled})
}

// CSRiskGetReplySuggestions 生成式回复建议 —— 右侧「回复建议」Tab 的数据源。
//
// query:
//
//	conversationId 站内会话ID（必填）
//
// 语义：
//   - 中低风险 → 返回 3 档语气候选（professional / warm / concise），客服可一键插入输入框
//   - critical / high → locked=true，**不返回自由话术**，改为返回 AI 生成的
//     「处置要点」放在 evidence 里。理由见 cs_reply_service.go 的产品原则注释。
//
// 降级约定：模型未配置 / 调用失败 / 返回非法 JSON 时，**不返回错误码**，
// 而是 200 + fallback=true + 规则证据，避免客服工作台弹错误、白屏。
func CSRiskGetReplySuggestions(ctx *gin.Context) {
	conversationID, _ := params.GetInt64(ctx, "conversationId")
	if conversationID <= 0 {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0064"))
		return
	}

	result, err := services.CSReplyService.Generate(ctx.Request.Context(), conversationID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}
