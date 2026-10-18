"use client"

import { UserIcon } from "lucide-react";
import { useEffect } from "react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { ScrollArea } from "@/components/ui/scroll-area";
// [AgentDesk-BEGIN] 数据共情者：会话列表风险色条
import { RISK_STYLE, needsImmediateAttention } from "@/components/workbench/risk-style";
import { useCsRiskMapStore } from "@/lib/stores/cs-risk-map";
// [AgentDesk-END]
import { IMConversationStatus } from "@/lib/generated/enums";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { cn, formatDateTime } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";

type ConversationListProps = {
  onAfterSelect?: () => void
}

export function ConversationList({ onAfterSelect }: ConversationListProps) {
  const t = useI18n()
  const conversations = useAgentConversationsStore((state) => state.conversations)
  const loading = useAgentConversationsStore((state) => state.conversationsLoading)
  const selectedId = useAgentConversationsStore((state) => state.selectedConversationId)
  const selectConversation = useAgentConversationsStore((state) => state.selectConversation)

  // [AgentDesk-BEGIN] 数据共情者：列表加载后一次性拉取风险等级映射
  const riskLevelByConversation = useCsRiskMapStore((s) => s.levelByConversation)
  const loadRiskMap = useCsRiskMapStore((s) => s.load)

  useEffect(() => {
    if (conversations.length > 0) {
      void loadRiskMap()
    }
  }, [conversations.length, loadRiskMap])
  // [AgentDesk-END]

  return (
    <ScrollArea className="overflow-auto">
      {loading ? (
        <div className="p-6 text-center text-sm text-muted-foreground">
          {t("conversation.loading")}
        </div>
      ) : conversations.length > 0 ? (
        conversations.map((conversation) => {
          const isSelected = selectedId === conversation.id
          // [AgentDesk-BEGIN] 风险色条数据
          const riskLevel = riskLevelByConversation[conversation.id] ?? "none"
          const riskStyle = RISK_STYLE[riskLevel]
          const showRiskBar = needsImmediateAttention(riskLevel)
          // [AgentDesk-END]
          return (
            <div
              key={conversation.id}
              className={`relative cursor-pointer border-b border-border/80 px-3 py-2 transition-colors hover:bg-muted/40 ${
                isSelected ? "bg-accent/70" : ""
              }`}
              onClick={() => {
                void selectConversation(conversation.id).then(
                  () => {
                    onAfterSelect?.()
                  },
                  () => {},
                )
              }}
            >
              {/* [AgentDesk-BEGIN] 高危会话左侧色条 —— 扫一眼列表就知道先处理谁 */}
              {showRiskBar ? (
                <span
                  className={cn(
                    "absolute inset-y-0 left-0 w-1",
                    riskStyle.bar,
                    riskLevel === "critical" && "risk-pulse",
                  )}
                  aria-hidden
                />
              ) : null}
              {/* [AgentDesk-END] */}
              <div className="overflow-hidden">
                <div className="flex items-center gap-2">
                  <Avatar className="size-7 shrink-0">
                    <AvatarImage src="" />
                    <AvatarFallback className="bg-primary/10 text-primary">
                      <UserIcon className="size-3.5 text-primary" />
                    </AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="min-w-0 flex-1 truncate font-medium text-sm leading-4">
                        {conversation.customerName ||
                          t("conversation.customerFallback", {
                            id: conversation.customerId || conversation.id,
                          })}
                      </span>
                      {conversation.agentUnreadCount > 0 ? (
                        <div className="flex size-4.5 shrink-0 items-center justify-center rounded-full bg-primary text-[10px] text-primary-foreground">
                          {conversation.agentUnreadCount > 99
                            ? "99+"
                            : conversation.agentUnreadCount}
                        </div>
                      ) : null}
                    </div>
                    <div className="mt-0.5 text-[11px] text-muted-foreground">
                      {conversation.lastMessageAt
                        ? formatDateTime(conversation.lastMessageAt)
                        : t("conversation.noTime")}
                    </div>
                  </div>
                </div>
                <div className="mt-0.5 truncate text-xs leading-4 text-muted-foreground">
                  {conversation.lastMessageSummary || t("conversation.noLatestMessage")}
                </div>
                {conversation.status === IMConversationStatus.Pending &&
                conversation.currentTeamName ? (
                  <div className="mt-1 flex items-center gap-1 text-[10px] text-muted-foreground">
                    <span className="rounded-md bg-muted px-1.5 py-0.5">
                      {t("conversation.teamOnDuty", {
                        name: conversation.currentTeamName,
                      })}
                    </span>
                  </div>
                ) : null}
              </div>
            </div>
          )
        })
      ) : (
        <div className="p-6 text-center text-sm text-muted-foreground">
          {t("conversation.empty")}
        </div>
      )}
    </ScrollArea>
  )
}
