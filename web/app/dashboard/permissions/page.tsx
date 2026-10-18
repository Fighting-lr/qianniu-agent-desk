"use client"

import { KeyRoundIcon, RouteIcon, SearchIcon } from "lucide-react"

import { DashboardListPage } from "@/components/dashboard/list"
import { Badge } from "@/components/ui/badge"
import { fetchPermissions, type AdminPermission } from "@/lib/api/admin"
import { Status } from "@/lib/generated/enums"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { getPermissionDisplayName, getPermissionGroupName } from "@/lib/permission-i18n"

export default function DashboardPermissionsPage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const listStatusOptions = [
    { value: "all", label: t("status.all") },
    { value: String(Status.Ok), label: t("status.ok") },
    { value: String(Status.Disabled), label: t("status.disabled") },
    { value: String(Status.Deleted), label: t("status.deleted") },
  ]

  return (
    <DashboardListPage<AdminPermission>
      filters={[
        {
          name: "keyword",
          label: t("permission.filterKeyword"),
          placeholder: t("permission.filterKeyword"),
          defaultValue: "",
          trim: true,
          className: "w-full sm:w-72",
          inputClassName: "pl-9",
          icon: <SearchIcon className="size-4" />,
        },
        {
          name: "groupName",
          label: t("permission.filterGroup"),
          placeholder: t("permission.filterGroup"),
          defaultValue: "",
          trim: true,
          className: "w-full sm:w-44",
        },
        {
          name: "status",
          label: t("status.all"),
          type: "select",
          defaultValue: "all",
          allValue: "all",
          options: listStatusOptions,
          className: "w-full sm:w-36",
        },
      ]}
      fetchList={fetchPermissions}
      getItemId={(item) => item.id}
      columns={[
        {
          key: "permission",
          label: t("permission.columnPermission"),
          render: (item) => (
            <div className="flex items-center gap-3">
              <div className="flex size-10 items-center justify-center rounded-md bg-muted text-muted-foreground">
                <KeyRoundIcon className="size-4" />
              </div>
              <div>
                <div className="font-medium">
                  {getPermissionDisplayName(item.code, item.name, locale)}
                </div>
                <div className="text-xs text-muted-foreground">{item.type}</div>
              </div>
            </div>
          ),
        },
        {
          key: "code",
          label: t("permission.columnCode"),
          render: (item) => <Badge variant="outline">{item.code}</Badge>,
        },
        {
          key: "group",
          label: t("permission.columnGroup"),
          render: (item) => getPermissionGroupName(item.groupName, locale),
        },
        {
          key: "api",
          label: t("permission.columnApi"),
          render: (item) => (
            <div className="flex items-start gap-2">
              <Badge variant="secondary">{item.method || "ANY"}</Badge>
              <div className="text-sm text-muted-foreground">
                <div className="flex items-center gap-1">
                  <RouteIcon className="size-3.5" />
                  {item.apiPath || "-"}
                </div>
              </div>
            </div>
          ),
        },
        {
          key: "status",
          label: t("permission.columnStatus"),
          render: (item) => (
            <Badge variant={item.status === Status.Ok ? "secondary" : "outline"}>
              {getStatusLabel(item.status, t)}
            </Badge>
          ),
        },
      ]}
      labels={{
        refresh: t("permission.refresh"),
        query: t("permission.query"),
        loading: t("permission.loading"),
        empty: t("permission.empty"),
        loadFailed: t("permission.loadFailed"),
      }}
    />
  )
}

function getStatusLabel(
  status: number,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  if (status === Status.Ok) {
    return t("status.ok")
  }
  if (status === Status.Disabled) {
    return t("status.disabled")
  }
  if (status === Status.Deleted) {
    return t("status.deleted")
  }
  return String(status)
}
