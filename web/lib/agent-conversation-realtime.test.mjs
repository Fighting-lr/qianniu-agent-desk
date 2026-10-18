import assert from "node:assert/strict"
import test from "node:test"

import { shouldReloadConversationListForRealtimePatch } from "./agent-conversation-realtime.ts"

test("reloads conversation list when realtime patch changes list membership fields", () => {
  assert.equal(
    shouldReloadConversationListForRealtimePatch({
      conversationId: 1,
      status: 3,
      currentAssigneeId: 101,
    }),
    true
  )
})

test("keeps local patching for message summary and unread-only changes", () => {
  assert.equal(
    shouldReloadConversationListForRealtimePatch({
      conversationId: 1,
      lastMessageId: 9,
      lastMessageSummary: "hello",
      agentUnreadCount: 1,
    }),
    false
  )
})
