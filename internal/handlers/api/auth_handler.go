package api

import (
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func Login(ctx *gin.Context) {
	cfg := config.Current()
	req := request.LoginRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ret, err := services.AuthService.Login(req, cfg.Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

// [AgentDesk-BEGIN] 开放注册（注册即登录）
//
// 流程：校验 + 建用户（默认 cs_user 角色 + 自动客服档案）→ 复用完整登录流程发 token。
// 复用 Login 而不是手工签发，是为了不绕过它的失败锁定与审计日志逻辑。
// 返回结构与 /api/auth/login 完全一致，前端注册成功后可直接写入会话进入系统。
func Register(ctx *gin.Context) {
	cfg := config.Current()
	req := request.RegisterRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	user, err := services.UserService.Register(req)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ret, err := services.AuthService.Login(request.LoginRequest{
		Username: user.Username,
		Password: req.Password,
	}, cfg.Auth, ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

// [AgentDesk-END]

func PublicConfig(ctx *gin.Context) {
	cfg := config.Current()
	httpx.WriteJSON(ctx, &response.PublicConfigResponse{
		Language:      cfg.LanguageOrDefault(),
		WxWorkEnabled: cfg.WxWork.Enabled,
		OIDCEnabled:   cfg.OIDC.Enabled,
	})
}

func WxWorkLogin(ctx *gin.Context) {
	loginURL, err := services.WxWorkLoginService.BuildWxWorkLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func WxWorkQRLogin(ctx *gin.Context) {
	loginURL, err := services.WxWorkLoginService.BuildWxWorkQRCodeLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func WxWorkCallback(ctx *gin.Context) {
	cfg := config.Current()
	ticket, next, err := services.WxWorkLoginService.LoginByWxWork(
		ctx.Query("code"),
		ctx.Query("state"),
		cfg.Auth,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
	)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/login?wxworkError="+url.QueryEscape(wxWorkErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, "/dashboard/login/wxwork/callback?ticket="+url.QueryEscape(ticket)+"&next="+url.QueryEscape(next))
}

func WxWorkExchange(ctx *gin.Context) {
	req := request.WxWorkExchangeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.WxWorkLoginService.ExchangeWxWorkLoginTicket(req.Ticket)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func OIDCLogin(ctx *gin.Context) {
	loginURL, err := services.OIDCLoginService.BuildOIDCLoginURL(ctx.Query("next"))
	if err != nil {
		ctx.Redirect(http.StatusFound, "/dashboard/login?oidcError="+url.QueryEscape(loginErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, loginURL)
}

func OIDCCallback(ctx *gin.Context) {
	cfg := config.Current()
	ticket, next, err := services.OIDCLoginService.LoginByOIDC(
		ctx.Request.Context(),
		ctx.Query("code"),
		ctx.Query("state"),
		cfg.Auth,
		ctx.ClientIP(),
		ctx.GetHeader("User-Agent"),
	)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/dashboard/login?oidcError="+url.QueryEscape(loginErrorMessage(err.Error())))
		return
	}
	ctx.Redirect(http.StatusFound, "/dashboard/login/oidc/callback?ticket="+url.QueryEscape(ticket)+"&next="+url.QueryEscape(next))
}

func OIDCExchange(ctx *gin.Context) {
	req := request.OIDCExchangeRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret, err := services.OIDCLoginService.ExchangeOIDCLoginTicket(req.Ticket)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func Logout(ctx *gin.Context) {
	if err := services.AuthService.Logout(ctx.GetHeader("Authorization")); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func Profile(ctx *gin.Context) {
	ret, err := services.AuthService.CurrentProfile(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, ret)
}

func wxWorkErrorMessage(message string) string {
	return loginErrorMessage(message)
}

func loginErrorMessage(message string) string {
	if idx := strings.Index(message, ": "); idx >= 0 {
		message = message[idx+2:]
	}
	return message
}
