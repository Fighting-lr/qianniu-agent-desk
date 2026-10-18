import { clearSession, writeSession, type AuthSession } from "@/lib/auth"
import { request } from "@/lib/api/client"

export type LoginRequest = {
  username: string
  password: string
}

export async function loginWithPassword(payload: LoginRequest) {
  const data = await request<AuthSession>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

// [AgentDesk-BEGIN] 开放注册
// 后端 /api/auth/register 的返回结构与 /api/auth/login 完全一致（注册即登录），
// 所以这里拿到就直接写会话，前端注册成功后无需再走一遍登录。
export type RegisterRequest = {
  username: string
  nickname: string
  password: string
}

export async function registerWithPassword(payload: RegisterRequest) {
  const data = await request<AuthSession>("/api/auth/register", {
    method: "POST",
    body: JSON.stringify(payload),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

// [AgentDesk-END]

export async function exchangeWxWorkTicket(ticket: string) {
  const data = await request<AuthSession>("/api/auth/wxwork_exchange", {
    method: "POST",
    body: JSON.stringify({ ticket }),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function exchangeOIDCTicket(ticket: string) {
  const data = await request<AuthSession>("/api/auth/oidc_exchange", {
    method: "POST",
    body: JSON.stringify({ ticket }),
    skipAuth: true,
  })
  writeSession(data)
  return data
}

export async function fetchProfile() {
  return request<AuthSession>("/api/auth/profile")
}

export async function logout() {
  try {
    await request("/api/auth/logout", {
      method: "POST",
    })
  } finally {
    clearSession()
  }
}
