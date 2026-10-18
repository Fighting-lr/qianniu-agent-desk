import { RegisterForm } from "@/components/register-form"

// [AgentDesk] 注册页 —— 与登录页（/dashboard/login）同一视觉外壳。
// 上游预留了 auth.noAccount / auth.signUp 文案但从未实现本页，这里补齐。
export default function RegisterPage() {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center bg-muted p-6 md:p-10">
      <div className="w-full max-w-sm md:max-w-4xl">
        <RegisterForm />
      </div>
    </div>
  )
}
