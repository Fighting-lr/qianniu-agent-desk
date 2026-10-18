package request

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// [AgentDesk-BEGIN] 开放注册
// 上游只预留了 i18n 文案（auth.noAccount / auth.signUp）和一处被注释掉的入口链接，
// 注册能力一直没落地。这里补齐请求体。
type RegisterRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// [AgentDesk-END]

type WxWorkExchangeRequest struct {
	Ticket string `json:"ticket"`
}

type OIDCExchangeRequest struct {
	Ticket string `json:"ticket"`
}
