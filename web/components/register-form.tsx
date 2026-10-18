"use client"

// [AgentDesk] 注册表单 —— 上游预留了 i18n 文案（auth.noAccount / auth.signUp）
// 和登录页一处被注释掉的入口链接，但注册页一直没做，这里补齐。
// 结构刻意与 login-form.tsx 保持一致（同样的卡片布局与视觉语言），
// 但不拉取 PublicConfig / 不渲染企业微信与 OIDC 入口 —— 注册只走账密。

import { useEffect, useState } from "react"
import { toast } from "sonner"

import { useAuth } from "@/components/auth-provider"
import { registerWithPassword } from "@/lib/api/auth"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"

// 与后端 normalizeSQLiteDSN 无关 —— 这里是用户名规则，和
// internal/services/user_service.go 的 registerUsernamePattern 保持一致
const USERNAME_PATTERN = /^[a-zA-Z0-9_]{3,32}$/
const MIN_PASSWORD_LENGTH = 6

export function RegisterForm({
  className,
  ...props
}: React.ComponentProps<"div">) {
  const t = useI18n()
  const { session } = useAuth()
  const [isPending, setIsPending] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const redirectPath = "/dashboard/"

  // 已登录的用户不需要再注册
  useEffect(() => {
    if (session) {
      window.location.assign(redirectPath)
    }
  }, [redirectPath, session])

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)
    const formData = new FormData(event.currentTarget)
    const username = formData.get("username")?.toString().trim() ?? ""
    const nickname = formData.get("nickname")?.toString().trim() ?? ""
    const password = formData.get("password")?.toString() ?? ""
    const confirmPassword = formData.get("confirmPassword")?.toString() ?? ""

    if (!USERNAME_PATTERN.test(username)) {
      const err = t("auth.usernameRule")
      setFormError(err)
      toast.error(err)
      return
    }
    if (password.length < MIN_PASSWORD_LENGTH) {
      const err = t("auth.passwordRule")
      setFormError(err)
      toast.error(err)
      return
    }
    if (password !== confirmPassword) {
      const err = t("auth.passwordMismatch")
      setFormError(err)
      toast.error(err)
      return
    }

    setIsPending(true)

    try {
      // 后端注册即登录：返回结构与 /api/auth/login 一致，registerWithPassword 内部已写会话
      await registerWithPassword({ username, nickname, password })
      toast.success(t("auth.registerSuccess"))
      window.location.assign(redirectPath)
    } catch (error) {
      const msg = error instanceof Error ? error.message : t("auth.registerFailed")
      setFormError(msg)
      toast.error(msg)
    } finally {
      setIsPending(false)
    }
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card className="overflow-hidden p-0">
        <CardContent className="grid p-0 md:grid-cols-2">
          <form className="p-6 md:p-8" onSubmit={handleSubmit}>
            <FieldGroup>
              <div className="flex flex-col items-center gap-2 text-center">
                <h1 className="text-2xl font-bold">{t("auth.registerTitle")}</h1>
                <p className="text-balance text-muted-foreground">
                  {t("auth.registerDescription", { brand: t("app.brand") === "贝壳AGENT" ? "AgentDesk" : (t("app.brand") || "AgentDesk") })}
                </p>
              </div>
              <Field>
                <FieldLabel htmlFor="register-username">{t("auth.username")}</FieldLabel>
                <Input
                  id="register-username"
                  name="username"
                  placeholder={t("auth.usernamePlaceholder")}
                  autoComplete="username"
                  required
                />
                <p className="text-xs text-muted-foreground">{t("auth.usernameRule")}</p>
              </Field>
              <Field>
                <FieldLabel htmlFor="register-nickname">{t("auth.nickname")}</FieldLabel>
                <Input
                  id="register-nickname"
                  name="nickname"
                  placeholder={t("auth.nicknamePlaceholder")}
                  autoComplete="nickname"
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="register-password">{t("auth.password")}</FieldLabel>
                <Input
                  id="register-password"
                  name="password"
                  type="password"
                  placeholder={t("auth.passwordPlaceholder")}
                  autoComplete="new-password"
                  required
                />
                <p className="text-xs text-muted-foreground">{t("auth.passwordRule")}</p>
              </Field>
              <Field>
                <FieldLabel htmlFor="register-confirm-password">
                  {t("auth.confirmPassword")}
                </FieldLabel>
                <Input
                  id="register-confirm-password"
                  name="confirmPassword"
                  type="password"
                  placeholder={t("auth.confirmPasswordPlaceholder")}
                  autoComplete="new-password"
                  required
                />
              </Field>
              {formError ? (
                <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive font-medium text-center">
                  {formError}
                </div>
              ) : null}
              <Field>
                <Button type="submit" disabled={isPending}>
                  {isPending ? t("auth.creatingAccount") : t("auth.registerNow")}
                </Button>
              </Field>
              <p className="text-center text-sm text-muted-foreground">
                {t("auth.haveAccount")}{" "}
                <a
                  href="/dashboard/login/"
                  onClick={(e) => {
                    e.preventDefault()
                    window.location.assign("/dashboard/login/")
                  }}
                  className="underline underline-offset-2 hover:text-foreground cursor-pointer"
                >
                  {t("auth.goSignIn")}
                </a>
              </p>
            </FieldGroup>
          </form>
          <div className="relative hidden bg-muted md:block">
            {/* 与登录页同一张装饰图，视觉上保持一套语言 */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/images/login-illustration.svg?v=20261017-shield"
              alt={t("auth.imageAlt")}
              className="absolute inset-0 h-full w-full object-cover select-none pointer-events-none"
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
