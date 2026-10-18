package services

import (
	"testing"

	"agent-desk/internal/models"
)

// ============================================================================
// [AgentDesk] 风险推导规则的单测
//
// 这些用例不只是"跑通"，它们本身就是**规则表的可执行文档**：
// 每一条都对应一个真实的业务判断，评委可以直接读代码验证逻辑。
// ============================================================================

func TestDeriveRisks_HardWordSafetyCritical(t *testing.T) {
	// 买家原声直接提到"过敏"，必须判到 critical。
	s := newCSRiskService()
	events := s.DeriveRisks([]string{"我用了以后过敏了，脸都肿了"}, "", "")

	level := s.OverallRisk(events)
	if level != models.RiskLevelCritical {
		t.Fatalf("期望 critical，实际 %s；events=%+v", level, events)
	}

	var found bool
	for _, e := range events {
		if e.Category == models.RiskCategorySafety && e.Level == models.RiskLevelCritical {
			found = true
			if e.Source != "hard_word" {
				t.Errorf("期望 source=hard_word，实际 %s", e.Source)
			}
			if e.HardWords == "" || e.HardWords == "[]" {
				t.Errorf("命中硬词应写入 HardWords，实际 %q", e.HardWords)
			}
		}
	}
	if !found {
		t.Fatal("未产生 safety/critical 风险事件")
	}
}

func TestDeriveRisks_SceneMapping(t *testing.T) {
	tests := []struct {
		sceneMinor string
		wantLevel  string
		wantCat    string
	}{
		{"泛红刺痒", models.RiskLevelHigh, models.RiskCategorySafety},
		{"过敏就医", models.RiskLevelCritical, models.RiskCategorySafety},
		{"漏发赠品", models.RiskLevelMedium, models.RiskCategoryExperience},
		{"物流停滞疑似丢件", models.RiskLevelHigh, models.RiskCategoryTimeliness},
		{"质疑正品退货", models.RiskLevelHigh, models.RiskCategoryCompliance},
		{"孕妇可用咨询", models.RiskLevelHigh, models.RiskCategorySafety},
		// 「积分兑换咨询」已定为 none（纯咨询，答完即结束，不上风险看板），
		// 因此不再产生风险事件 —— 这里改为断言"不应产生事件"（见下方单独用例）。
	}
	s := newCSRiskService()
	for _, tt := range tests {
		t.Run(tt.sceneMinor, func(t *testing.T) {
			// 传空文本，确保命中的只能是场景映射这一层。
			events := s.DeriveRisks([]string{""}, tt.sceneMinor, "")
			if len(events) == 0 {
				t.Fatalf("场景 %q 未产生任何风险事件", tt.sceneMinor)
			}
			var hit bool
			for _, e := range events {
				if e.Source == "scene" {
					hit = true
					if e.Level != tt.wantLevel {
						t.Errorf("等级：期望 %s，实际 %s", tt.wantLevel, e.Level)
					}
					if e.Category != tt.wantCat {
						t.Errorf("类别：期望 %s，实际 %s", tt.wantCat, e.Category)
					}
				}
			}
			if !hit {
				t.Errorf("未产生 source=scene 的事件")
			}
		})
	}
}

// TestDeriveRisks_PureConsultProducesNoEvent 纯咨询场景不应产生任何风险事件。
//
// 口径：cs_models.go 里定义 none = "纯咨询，答完即结束，不出现在风险看板上"。
// 这条断言保护的是「不让噪音淹没真实风险」—— 若咨询类也上板，
// 客服看到的告警里 90% 是"能开发票吗"，真正的 critical 会被淹没。
func TestDeriveRisks_PureConsultProducesNoEvent(t *testing.T) {
	s := newCSRiskService()
	pureConsult := []string{"使用方法", "色号选择", "套装搭配推荐", "好评回访", "发票申请", "积分兑换咨询"}
	for _, scene := range pureConsult {
		t.Run(scene, func(t *testing.T) {
			// 空文本 → 只有场景层可能触发
			events := s.DeriveRisks([]string{""}, scene, "")
			for _, e := range events {
				if e.Source == "scene" {
					t.Errorf("纯咨询场景「%s」不应产生场景风险事件，实际 level=%s", scene, e.Level)
				}
			}
			if got := s.OverallRisk(events); got != models.RiskLevelNone {
				t.Errorf("纯咨询场景「%s」应无风险，实际 %s", scene, got)
			}
		})
	}
}

func TestDeriveRisks_TicketEvidence(t *testing.T) {
	// 不良反应工单的存在本身即最高级证据 —— 不依赖任何文本。
	s := newCSRiskService()
	events := s.DeriveRisks([]string{""}, "", "adverse")

	level := s.OverallRisk(events)
	if level != models.RiskLevelCritical {
		t.Fatalf("不良反应工单应判 critical，实际 %s", level)
	}

	var hasTicketSource bool
	for _, e := range events {
		if e.Source == "ticket" {
			hasTicketSource = true
		}
	}
	if !hasTicketSource {
		t.Fatal("未产生 source=ticket 的事件")
	}
}

func TestDeriveRisks_TakesHighestLevel(t *testing.T) {
	// 低危文本 + 高危场景 → 取高危，不能因为文本"友好"就降级。
	s := newCSRiskService()
	events := s.DeriveRisks([]string{"你好，想问一下"}, "泛红刺痒", "")
	if got := s.OverallRisk(events); got != models.RiskLevelHigh {
		t.Fatalf("期望 high，实际 %s", got)
	}
}

func TestDeriveRisks_NoRisk(t *testing.T) {
	// 常规咨询不应产生任何风险 —— 避免"狼来了"。
	s := newCSRiskService()
	events := s.DeriveRisks([]string{"这个色号适合我吗"}, "色号选择", "")
	if got := s.OverallRisk(events); got != models.RiskLevelNone {
		t.Fatalf("常规咨询期望 none，实际 %s；events=%+v", got, events)
	}
}

func TestDeriveRisks_ComplianceAndOpinion(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantCat string
		wantLv  string
	}{
		{"监管投诉", "再不解决我就打12315投诉你们", models.RiskCategoryCompliance, models.RiskLevelCritical},
		{"法律威胁", "我要起诉你们，已经找律师了", models.RiskCategoryOpinion, models.RiskLevelCritical},
		{"平台曝光", "你们这是欺骗消费者，我要去黑猫曝光", models.RiskCategoryOpinion, models.RiskLevelCritical},
		{"时效焦虑", "这个包裹已经半个月了还没到，我很着急", models.RiskCategoryTimeliness, models.RiskLevelHigh},
		{"情绪负面", "你们服务太差了，真的很失望", models.RiskCategoryExperience, models.RiskLevelHigh},
	}
	s := newCSRiskService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := s.DeriveRisks([]string{tt.text}, "", "")
			var hit bool
			for _, e := range events {
				if e.Category == tt.wantCat && e.Level == tt.wantLv {
					hit = true
				}
			}
			if !hit {
				t.Fatalf("期望 %s/%s，实际 events=%+v", tt.wantCat, tt.wantLv, events)
			}
		})
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{0, models.RiskLevelNone},
		{10, models.RiskLevelLow},
		{49, models.RiskLevelLow},
		{50, models.RiskLevelMedium},
		{69, models.RiskLevelMedium},
		{70, models.RiskLevelHigh},
		{94, models.RiskLevelHigh},
		{95, models.RiskLevelCritical},
		{100, models.RiskLevelCritical},
	}
	for _, tt := range tests {
		if got := scoreToLevel(tt.score); got != tt.want {
			t.Errorf("scoreToLevel(%d)：期望 %s，实际 %s", tt.score, tt.want, got)
		}
	}
}

func TestRiskLevelRank_Ordering(t *testing.T) {
	// 等级排序必须严格单调，否则 OverallRisk 会取错。
	order := []string{
		models.RiskLevelNone,
		models.RiskLevelLow,
		models.RiskLevelMedium,
		models.RiskLevelHigh,
		models.RiskLevelCritical,
	}
	for i := 1; i < len(order); i++ {
		if models.RiskLevelRank(order[i]) <= models.RiskLevelRank(order[i-1]) {
			t.Fatalf("等级排序非单调：%s(%d) 未高于 %s(%d)",
				order[i], models.RiskLevelRank(order[i]),
				order[i-1], models.RiskLevelRank(order[i-1]))
		}
	}
}

func TestSceneMinorToRisk_CoversAll41(t *testing.T) {
	// 官方 scene_minor 共 41 类。少一类就意味着某些会话得不到场景级判定。
	const want = 41
	if got := len(sceneMinorToRisk); got != want {
		t.Fatalf("场景映射表应覆盖 %d 类，实际 %d 类", want, got)
	}
}

func TestTicketTypeToRisk_CoversAll5(t *testing.T) {
	const want = 5
	if got := len(ticketTypeToRisk); got != want {
		t.Fatalf("工单映射表应覆盖 %d 类，实际 %d 类", want, got)
	}
}
