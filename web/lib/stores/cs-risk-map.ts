"use client";

/**
 * ============================================================================
 * [AgentDesk] 风险等级映射 Store —— 会话列表角标 / 色条的数据源
 * ----------------------------------------------------------------------------
 * 会话列表可能有几十上百行，逐行请求风险数据会产生 N 次网络往返。
 * 这里改成：一次性拉取全量看板（/api/dashboard/cs-risk/board），
 * 在内存里建 conversationId → riskLevel 的映射，供列表 O(1) 查询。
 *
 * 规模评估：赛题数据 138 条会话、107 条风险事件，
 * 单次响应在 100KB 量级 —— 一次性拉取比 N 次点查更划算，也更稳。
 *
 * 刷新时机：进入会话列表时拉一次；有新会话风险变化时手动 invalidate。
 * 不做轮询 —— 客服工作台里后台轮询会干扰消息区滚动与输入焦点。
 * ============================================================================
 */

import { create } from "zustand";

import { fetchRiskBoard, type CSSessionContextDTO } from "@/lib/api/cs-risk";
import type { RiskLevel } from "@/components/workbench/types";

type CsRiskMapState = {
  /** conversationId → 风险等级。 */
  levelByConversation: Record<number, RiskLevel>;
  /** conversationId → 全景数据，供悬停卡片等场景复用。 */
  contextByConversation: Record<number, CSSessionContextDTO>;
  loaded: boolean;
  loading: boolean;
  /** 拉取一次全量风险映射。已加载过则直接返回（除非 force）。 */
  load: (force?: boolean) => Promise<void>;
  /** 取单个会话的风险等级，未命中返回 "none"。 */
  levelOf: (conversationId: number) => RiskLevel;
};

export const useCsRiskMapStore = create<CsRiskMapState>((set, get) => ({
  levelByConversation: {},
  contextByConversation: {},
  loaded: false,
  loading: false,

  load: async (force = false) => {
    const { loaded, loading } = get();
    if (loading || (loaded && !force)) return;

    set({ loading: true });
    try {
      const resp = await fetchRiskBoard({ limit: 500 });
      const levelMap: Record<number, RiskLevel> = {};
      const ctxMap: Record<number, CSSessionContextDTO> = {};

      for (const row of resp?.list ?? []) {
        if (!row.conversationId) continue;
        levelMap[row.conversationId] = row.overallRisk as RiskLevel;
        ctxMap[row.conversationId] = row;
      }

      set({
        levelByConversation: levelMap,
        contextByConversation: ctxMap,
        loaded: true,
      });
    } catch {
      // 拉取失败不阻塞会话列表渲染 —— 降级为"没有角标"
      set({ loaded: true });
    } finally {
      set({ loading: false });
    }
  },

  levelOf: (conversationId: number) => {
    return get().levelByConversation[conversationId] ?? "none";
  },
}));
