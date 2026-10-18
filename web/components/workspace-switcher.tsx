"use client"

import { CheckIcon, ChevronsUpDownIcon } from "lucide-react"
import Link from "next/link"
import type { ReactElement } from "react"

import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

export type WorkspaceKey = "dashboard" | "workbench"

export type WorkspaceOption = {
  key: WorkspaceKey
  href: string
  labelKey: string
}

export const workspaceOptions: WorkspaceOption[] = [
  {
    key: "dashboard",
    href: "/dashboard",
    labelKey: "workspace.dashboard",
  },
  {
    key: "workbench",
    href: "/workbench",
    labelKey: "workspace.workbench",
  },
]

type WorkspaceSwitcherProps = {
  currentWorkspace: WorkspaceKey
  variant?: "sidebar" | "header" | "rail"
  className?: string
  trigger?: ReactElement
}

export function WorkspaceSwitcher({
  currentWorkspace,
  variant = "header",
  className,
  trigger,
}: WorkspaceSwitcherProps) {
  const t = useI18n()
  const currentOption =
    workspaceOptions.find((item) => item.key === currentWorkspace) ?? workspaceOptions[0]
  const switchIndicatorClassName =
    "absolute bottom-0.5 right-0.5 size-2.5 rounded-full bg-sidebar text-sidebar-foreground/70"

  const triggerClassName = cn(
    "gap-2 text-left",
    variant === "header" &&
      "h-9 rounded-md border border-border/70 bg-background px-2.5 shadow-xs hover:bg-muted",
    variant === "sidebar" &&
      "relative data-[slot=sidebar-menu-button]:p-1.5! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0! group-data-[collapsible=icon]:data-[slot=sidebar-menu-button]:p-0!",
    variant === "rail" &&
      "relative size-8 rounded-md border-0 bg-transparent p-0 shadow-none hover:bg-sidebar-accent",
    className
  )
  const triggerContent =
    variant === "rail" ? (
      <>
        <img
          src="/images/logo.svg"
          alt={t("app.brand")}
          width="32"
          height="32"
          className="size-7 shrink-0 object-contain"
        />
        <span className="sr-only">
          {t("workspace.switchWorkspace")} - {t(currentOption.labelKey)}
        </span>
        <ChevronsUpDownIcon className={switchIndicatorClassName} />
      </>
    ) : (
      <>
        <img
          src="/images/logo.svg"
          alt={t("app.brand")}
          width="32"
          height="32"
          className="size-7 shrink-0 object-contain"
        />
        <div className="grid min-w-0 flex-1 text-left leading-tight">
          <span className="truncate text-sm font-semibold">{t("app.brand")}</span>
          <span className="truncate text-xs text-muted-foreground">
            {t(currentOption.labelKey)}
          </span>
        </div>
        <ChevronsUpDownIcon className="ml-auto size-4 shrink-0 text-muted-foreground group-data-[collapsible=icon]:hidden" />
        {variant === "sidebar" ? (
          <ChevronsUpDownIcon
            className={cn(switchIndicatorClassName, "hidden group-data-[collapsible=icon]:block")}
          />
        ) : null}
      </>
    )

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          trigger ?? <Button variant="ghost" className={triggerClassName} />
        }
      >
        {triggerContent}
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        side={variant === "sidebar" || variant === "rail" ? "right" : "bottom"}
        sideOffset={8}
        className="w-60 min-w-60"
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel>{t("workspace.switchWorkspace")}</DropdownMenuLabel>
          {workspaceOptions.map((item) => (
            <DropdownMenuItem
              key={item.key}
              render={<Link href={item.href} />}
              className="cursor-pointer gap-2"
            >
              <span className="flex-1 truncate">{t(item.labelKey)}</span>
              {item.key === currentWorkspace ? (
                <CheckIcon className="size-4 text-primary" />
              ) : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
