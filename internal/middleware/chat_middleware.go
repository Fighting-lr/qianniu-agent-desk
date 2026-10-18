package middleware

import (
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

func ExternalUserMiddleware(ctx *gin.Context) {
	channel := services.ChannelService.GetEnabledChannel(ctx)
	if channel == nil {
		ctx.JSON(200, httpx.JsonErrorMsg(ctx, "error.e0210"))
		ctx.Abort()
		return
	}
	result, err := services.CustomerSessionService.VerifyRequest(ctx, channel)
	if err != nil {
		// 用 httpx.WriteJSON 确保 I18nError 的 Code 被正确传递
		httpx.WriteJSON(ctx, err)
		ctx.Abort()
		return
	}
	services.CustomerSessionService.SetRefreshHeaders(ctx, result)
	httpx.SetExternalUser(ctx, result.ExternalUser)
	ctx.Next()
}
