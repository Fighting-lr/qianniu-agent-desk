"use client";

/**
 * ============================================================================
 * [AgentDesk] CustomerPanorama —— 右侧「客户全景」Tab
 * ----------------------------------------------------------------------------
 * 产品主张是「让客服在买家开口之前，先看见这个人」。这个面板就是那句话的
 * 视觉落点 —— 它回答三个问题：
 *
 *   1. 这个人值多少？      → 历史会话数、累计实付
 *   2. 这个人以前出过什么事？ → 敏感标签、上次进线结论
 *   3. 这次涉及哪些单据？   → 关联订单、关联工单
 *
 * 「上次进线结论」是整个面板里最关键的一格：它把一次性客服变成连续性服务。
 * 官方数据里同一个买家昵称可能出现在多个会话里，这就是它的数据基础。
 * ============================================================================
 */

import {
  BadgeAlert,
  CalendarClock,
  Coins,
  History,
  Package,
  ShieldAlert,
  StickyNote,
  Ticket,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";
import type { CustomerPanorama as CustomerPanoramaType } from "./types";

type CustomerPanoramaProps = {
  data: CustomerPanoramaType;
  className?: string;
};

export function CustomerPanorama({ data, className }: CustomerPanoramaProps) {
  const t = useI18n();

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {/* ---------- 身份与价值 ---------- */}
      <div className="rounded-md border border-border bg-card px-3 py-2.5">
        <div className="text-sm font-medium text-foreground">{data.name}</div>
        <div className="mt-2 grid grid-cols-2 gap-2">
          <Metric
            icon={<History className="size-3.5" />}
            label={t("csRisk.panorama.visitCount")}
            value={String(data.visitCount)}
          />
          <Metric
            icon={<Coins className="size-3.5" />}
            label={t("csRisk.panorama.totalPaid")}
            value={`¥${formatMoney(data.totalPaid)}`}
          />
        </div>
      </div>

      {/* ---------- 上次进线结论 —— 最关键的一格 ---------- */}
      {data.lastConclusion ? (
        <div className="relative overflow-hidden rounded-md border border-border bg-muted/40">
          <span className="absolute inset-y-0 left-0 w-1 bg-primary/60" />
          <div className="flex items-start gap-2 py-2 pr-2.5 pl-3.5">
            <StickyNote className="mt-px size-3.5 shrink-0 text-primary" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-[11px] font-medium text-foreground">
                  {t("csRisk.insight.lastAt")}
                </span>
                {data.lastVisitAt ? (
                  <span className="inline-flex items-center gap-1 text-[10px] text-muted-foreground">
                    <CalendarClock className="size-2.5" />
                    {data.lastVisitAt}
                  </span>
                ) : null}
              </div>
              <p className="mt-1 text-[11px] leading-snug text-foreground/80">
                {data.lastConclusion}
              </p>
            </div>
          </div>
        </div>
      ) : null}

      {/* ---------- 敏感标签 ---------- */}
      {data.sensitiveTags.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          <SectionLabel icon={<ShieldAlert className="size-3.5" />}>
            {t("csRisk.panorama.sceneHistory")}
          </SectionLabel>
          <div className="flex flex-wrap gap-1">
            {data.sensitiveTags.map((tag) => (
              <span
                key={tag}
                className="inline-flex items-center gap-1 rounded-md border border-risk-high/50 bg-risk-high-bg px-1.5 py-0.5 text-[10px] leading-none text-risk-high"
              >
                <BadgeAlert className="size-2.5" />
                {tag}
              </span>
            ))}
          </div>
        </div>
      ) : null}

      {/* ---------- 关联订单 ---------- */}
      <div className="flex flex-col gap-1.5">
        <SectionLabel icon={<Package className="size-3.5" />}>
          {t("csRisk.panorama.orderInfo")}
        </SectionLabel>
        {data.orders.length > 0 ? (
          <div className="flex flex-col gap-1.5">
            {data.orders.map((o) => (
              <div
                key={o.orderNo}
                className="rounded-md border border-border bg-card px-2.5 py-1.5"
              >
                <div className="flex items-center gap-2">
                  <code className="text-[10px] text-muted-foreground">{o.orderNo}</code>
                  <span className="ml-auto text-[11px] font-medium tabular-nums text-foreground">
                    ¥{formatMoney(o.amount)}
                  </span>
                </div>
                <div className="mt-0.5 truncate text-[11px] text-foreground/80">{o.product}</div>
                <div className="mt-0.5 flex items-center gap-2 text-[10px] text-muted-foreground">
                  <span>{o.status}</span>
                  <span>·</span>
                  <span>{o.createdAt}</span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <EmptyLine text={t("csRisk.panorama.ticketNone")} />
        )}
      </div>

      {/* ---------- 关联工单 ---------- */}
      <div className="flex flex-col gap-1.5">
        <SectionLabel icon={<Ticket className="size-3.5" />}>
          {t("csRisk.panorama.ticketInfo")}
        </SectionLabel>
        {data.tickets.length > 0 ? (
          <div className="flex flex-col gap-1.5">
            {data.tickets.map((tk) => (
              <div
                key={tk.ticketNo}
                className="flex items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1.5"
              >
                <code className="text-[10px] text-muted-foreground">{tk.ticketNo}</code>
                <div className="flex flex-col flex-1 min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="text-[11px] text-foreground/80">{tk.type}</span>
                    {tk.batchNo ? (
                      <span className="rounded border border-amber-500/30 bg-amber-500/10 px-1 py-px text-[9px] text-amber-600 dark:text-amber-400">
                        批次 {tk.batchNo}
                      </span>
                    ) : null}
                  </div>
                  {tk.summary ? (
                    <span className="truncate text-[10px] text-muted-foreground">{tk.summary}</span>
                  ) : null}
                </div>
                <span className="ml-auto shrink-0 rounded bg-muted px-1.5 py-px text-[10px] text-muted-foreground">
                  {tk.status}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <EmptyLine text={t("csRisk.panorama.ticketNone")} />
        )}
      </div>
    </div>
  );
}

// ------------------------------------------------------------------ 小组件 ---

function Metric({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-md bg-muted/50 px-2 py-1.5">
      <div className="flex items-center gap-1 text-muted-foreground">{icon}</div>
      <div className="mt-0.5 text-sm font-semibold tabular-nums text-foreground">{value}</div>
      <div className="text-[10px] text-muted-foreground">{label}</div>
    </div>
  );
}

function SectionLabel({
  icon,
  children,
}: {
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
      {icon}
      {children}
    </div>
  );
}

function EmptyLine({ text }: { text: string }) {
  return (
    <div className="rounded-md border border-dashed border-border px-2.5 py-1.5 text-[10px] text-muted-foreground">
      {text}
    </div>
  );
}

/** 金额格式化：整数不显示小数，避免「¥398.00」的冗余。 */
function formatMoney(v: number): string {
  if (!Number.isFinite(v)) return "0";
  return Number.isInteger(v) ? String(v) : v.toFixed(2);
}
