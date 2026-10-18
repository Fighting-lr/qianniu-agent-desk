"use client"

import { useSyncExternalStore } from "react"
import { LaptopIcon, MoonIcon, SunIcon } from "lucide-react"
import { useTheme } from "next-themes"

import { useI18n } from "@/i18n/provider"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

type ThemeMode = "light" | "dark" | "system"

const themeOptions: Array<{
  value: ThemeMode
  labelKey: string
  icon: typeof SunIcon
}> = [
  { value: "light", labelKey: "theme.light", icon: SunIcon },
  { value: "dark", labelKey: "theme.dark", icon: MoonIcon },
  { value: "system", labelKey: "theme.system", icon: LaptopIcon },
]

export function ThemeToggle() {
  const t = useI18n()
  const { theme, setTheme } = useTheme()
  const mounted = useSyncExternalStore(
    () => () => {},
    () => true,
    () => false
  )

  const activeTheme = mounted ? ((theme as ThemeMode | undefined) ?? "system") : "system"
  const ActiveIcon =
    themeOptions.find((option) => option.value === activeTheme)?.icon ?? LaptopIcon

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="outline" size="sm" />}
        aria-label={t("theme.toggle")}
      >
        <ActiveIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40 min-w-40">
        <DropdownMenuRadioGroup
          value={activeTheme}
          onValueChange={(value) => setTheme(value as ThemeMode)}
        >
          {themeOptions.map((option) => {
            const Icon = option.icon
            return (
              <DropdownMenuRadioItem key={option.value} value={option.value}>
                <Icon />
                {t(option.labelKey)}
              </DropdownMenuRadioItem>
            )
          })}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
