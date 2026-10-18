package middleware

import (
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(ctx *gin.Context) {
	if !authenticateRequest(ctx) {
		return
	}
	ctx.Next()
}

func authenticateRequest(ctx *gin.Context) bool {
	if _, err := services.AuthService.Authenticate(ctx); err != nil {
		// 用 httpx.WriteJSON 而非 web.JsonError，确保 I18nError 的
		// Code（如 3000）被正确传递到前端，否则前端 parseResult
		// 收到 errorCode=0，不会触发 expireSession()，导致页面
		// 在 session 已过期时仍继续渲染，引发 client-side exception。
		httpx.WriteJSON(ctx, err)
		ctx.Abort()
		return false
	}
	return true
}
