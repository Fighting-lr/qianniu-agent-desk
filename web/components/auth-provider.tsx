"use client"

import {
  createContext,
  startTransition,
  useContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react"
import { usePathname, useRouter } from "next/navigation"

import { fetchProfile, logout } from "@/lib/api/auth"
import {
  AUTH_SESSION_EXPIRED_EVENT,
  clearSession,
  readSession,
  writeSession,
  type AuthSession,
} from "@/lib/auth"

type AuthContextValue = {
  session: AuthSession | null
  ready: boolean
  refreshProfile: () => Promise<void>
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const [session, setSession] = useState<AuthSession | null>(null)
  const [ready, setReady] = useState(false)
  // [AgentDesk-FIX] 认证白名单：登录页与注册页均无需登录态。
  // 此前白名单只含 /dashboard/login，导致未登录访问 /dashboard/register 时
  // requiresAuth=true，AuthProvider 直接弹回登录页（第二层守卫，与
  // app/dashboard/layout.tsx 的守卫是两个独立实现点，须同步修改）。
  const isAuthRoute =
    (pathname?.startsWith("/dashboard/login") ?? false) ||
    (pathname?.startsWith("/dashboard/register") ?? false)
  const requiresAuth =
    ((pathname?.startsWith("/dashboard") ?? false) ||
      (pathname?.startsWith("/workbench") ?? false)) &&
    !isAuthRoute

  const refreshProfile = useCallback(async () => {
    const stored = readSession()
    if (!stored) {
      setSession(null)
      setReady(true)
      return
    }

    try {
      const profile = await fetchProfile()
      const nextSession: AuthSession = {
        ...stored,
        user: profile.user,
        permissions: profile.permissions,
        roles: profile.roles,
        accessToken: profile.accessToken || stored.accessToken,
        expiresAt: profile.expiresAt || stored.expiresAt,
      }
      writeSession(nextSession)
      setSession(nextSession)
    } catch (error) {
      const errorCode = (error as Error & { errorCode?: number }).errorCode
      if (errorCode === 3000 || errorCode === 3002) {
        clearSession()
        setSession(null)
        if (requiresAuth) {
          startTransition(() => {
            router.replace("/dashboard/login")
          })
        }
      }
    } finally {
      setReady(true)
    }
  }, [requiresAuth, router])

  async function signOut() {
    try {
      await logout()
    } finally {
      setSession(null)
      startTransition(() => {
        router.replace("/dashboard/login")
      })
    }
  }

  useEffect(() => {
    function handleAuthExpired() {
      setSession(null)
      if (requiresAuth) {
        startTransition(() => {
          router.replace("/dashboard/login")
        })
      }
    }

    window.addEventListener(AUTH_SESSION_EXPIRED_EVENT, handleAuthExpired)
    return () => {
      window.removeEventListener(AUTH_SESSION_EXPIRED_EVENT, handleAuthExpired)
    }
  }, [requiresAuth, router])

  useEffect(() => {
    const stored = readSession()
    setSession(stored)
    if (stored) {
      void refreshProfile()
      return
    }

    setReady(true)
    if (requiresAuth) {
      startTransition(() => {
        router.replace("/dashboard/login")
      })
    }
  }, [requiresAuth, refreshProfile, router])

  return (
    <AuthContext.Provider value={{ session, ready, refreshProfile, signOut }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}
