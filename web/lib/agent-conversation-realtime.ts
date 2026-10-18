import type { RealtimeConversationPatch } from "@/lib/im-realtime-state"

const listMembershipFields = new Set<keyof RealtimeConversationPatch>([
  "status",
  "currentAssigneeId",
  "currentTeamId",
])

export function shouldReloadConversationListForRealtimePatch(
  patch: RealtimeConversationPatch | null | undefined
) {
  if (!patch) {
    return false
  }
  return Object.keys(patch).some((key) =>
    listMembershipFields.has(key as keyof RealtimeConversationPatch)
  )
}
