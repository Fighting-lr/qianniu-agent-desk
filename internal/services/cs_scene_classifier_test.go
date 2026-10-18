package services

import (
	"testing"

	"agent-desk/internal/models"
)

// TestAllSceneMinorsCover41 断言 41 类场景全覆盖。
//
// ★ 这条断言是「完整性断言」，不是「结果断言」——
// 它直接比对官方 41 类清单，而不是间接数"某等级有几个"。
// 后者在映射表残缺时仍可能通过（血泪教训）。
func TestAllSceneMinorsCover41(t *testing.T) {
	// 官方 41 个 scene_minor —— 以 cs_risk_service.go 的 sceneMinorToRisk 为权威
	official := make([]string, 0, 41)
	for k := range sceneMinorToRisk {
		official = append(official, k)
	}
	if len(official) != 41 {
		t.Fatalf("权威表 sceneMinorToRisk 应有 41 项，实际 %d", len(official))
	}

	// 分类器覆盖的子类
	covered := make(map[string]bool)
	for _, s := range AllSceneMinors() {
		covered[s] = true
	}

	var missing []string
	for _, s := range official {
		if !covered[s] {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("分类器未覆盖 %d 个子类：%v", len(missing), missing)
	}

	// 反向：分类器不得自创官方没有的子类
	var extra []string
	for s := range covered {
		if _, ok := sceneMinorToRisk[s]; !ok {
			extra = append(extra, s)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("分类器自创了 %d 个非官方子类：%v", len(extra), extra)
	}
}

// TestSceneMajorConsistency 断言每个子类的大类归属与权威表一致。
//
// ★ 这是一条「防错位」断言，价值极高。
//
// 背景：初版把 41 条规则按「业务直觉」自行分组，结果 33 条大类归错：
//
//	物流停滞疑似丢件 → 误归「物流」，官方是「物流异常」
//	色号选择        → 误归「咨询」，官方是「产品咨询」
//	发票申请        → 误归「咨询」，官方是「订单服务」
//
// 后果极其隐蔽：子类判对了，大类却报出官方不存在的名字。
// 实测大类准确率 16.9%、子类 67.7%，落差全部来自这个错位。
//
// 现在强制：规则表里写的 SceneMajor 必须等于权威映射表的值。
// 规则表的字段只是调试用，任何对外输出都必须经 sceneMajorOf() 查表。
func TestSceneMajorConsistency(t *testing.T) {
	for _, minor := range AllSceneMinors() {
		got := SceneMajorOf(minor)
		if got == "" {
			t.Errorf("子类「%s」没有大类归属", minor)
			continue
		}
		// 必须落在官方 10 大类之内
		found := false
		for _, mj := range officialSceneMajors {
			if mj == got {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("子类「%s」的大类「%s」不在官方 10 大类中（凭空类）", minor, got)
		}
	}

	// 反向：规则表字段与权威表必须一致，否则维护者会被误导
	for _, r := range sceneCategoryRules {
		want := sceneMinorToMajor[r.SceneMinor]
		if want == "" {
			t.Errorf("子类「%s」未出现在权威映射表中", r.SceneMinor)
			continue
		}
		if r.SceneMajor != want {
			t.Errorf("规则表「%s」写的大类是「%s」，权威表是「%s」—— 必须对齐，否则维护者会被误导",
				r.SceneMinor, r.SceneMajor, want)
		}
	}
}

// TestOfficialMajorsCoverage 断言官方 10 大类恰好 10 个且不是凭空造的。
func TestOfficialMajorsCoverage(t *testing.T) {
	if len(officialSceneMajors) != 10 {
		t.Fatalf("官方应为 10 大类，实际 %d：%v", len(officialSceneMajors), officialSceneMajors)
	}
	// 去重校验
	seen := map[string]bool{}
	for _, mj := range officialSceneMajors {
		if seen[mj] {
			t.Errorf("大类「%s」重复", mj)
		}
		seen[mj] = true
	}
	// 41 个子类必须全部有归属，且归属只能落在这 10 个里
	count := 0
	for _, r := range sceneCategoryRules {
		mj, ok := sceneMinorToMajor[r.SceneMinor]
		if !ok {
			t.Errorf("子类「%s」无权威归属", r.SceneMinor)
			continue
		}
		if !seen[mj] {
			t.Errorf("子类「%s」归属到非官方大类「%s」", r.SceneMinor, mj)
		}
		count++
	}
	if count != 41 {
		t.Errorf("应有 41 个子类，实际 %d", count)
	}
}

// TestClassifySceneTypical 覆盖典型场景的端到端分类。
func TestClassifySceneTypical(t *testing.T) {
	cases := []struct {
		name   string
		texts  []string
		wantMi string
		wantMj string
	}{
		{"过敏就医", []string{"用了之后脸过敏了，已经去医院看了"}, "过敏就医", "不良反应"},
		{"泛红刺痒", []string{"用完脸有点泛红刺痒"}, "泛红刺痒", "不良反应"},
		{"闷痘爆痘", []string{"用了两天就开始爆痘长闭口"}, "闷痘爆痘", "不良反应"},
		{"丢件", []string{"我的包裹物流一直不动，是不是丢件了"}, "物流停滞疑似丢件", "物流异常"},
		{"签收未收", []string{"显示签收了我根本没收收到"}, "显示签收未收到", "物流异常"},
		{"催发货", []string{"下单三天了什么时候发货"}, "催发货", "物流服务"},
		{"退款没到", []string{"退款迟迟不到账，钱一直没到"}, "退款迟迟不到账", "订单服务"},
		{"假货", []string{"这个是不是假货，怎么验真"}, "", ""}, // 见下方单独断言
		{"孕妇咨询", []string{"孕妇可以用吗"}, "孕妇可用咨询", "产品咨询"},
		{"色号选择", []string{"黄皮适合什么色号"}, "色号选择", "产品咨询"},
		{"成分适配", []string{"敏感肌能用吗，含不含酒精"}, "成分与肤质适配", "产品咨询"},
		{"漏发赠品", []string{"说好的赠品小样没送"}, "漏发赠品", "补发换货"},
		{"价保", []string{"刚买就降价了能保价吗"}, "大促价保补差", "退款打款"},
		{"运费", []string{"退货的运费谁承担，能报销吗"}, "退货运费报销", "退款打款"},
		{"发票", []string{"能开发票吗"}, "发票申请", "订单服务"},
		{"好评回访", []string{"评价晒图有返现吗"}, "好评回访", "其他服务"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyScene(c.texts)
			if c.wantMi == "" {
				// 只断言命中"质疑正品"相关的安全类场景之一
				if got.SceneMinor != "质疑正品退货" && got.SceneMinor != "正品鉴别咨询" {
					t.Fatalf("「假货/验真」应归入正品相关场景，实际「%s」", got.SceneMinor)
				}
				return
			}
			if got.SceneMinor != c.wantMi {
				t.Fatalf("期望子类「%s」，实际「%s」（命中=%v 置信=%.2f）",
					c.wantMi, got.SceneMinor, got.Matched, got.Confidence)
			}
			if got.SceneMajor != c.wantMj {
				t.Fatalf("期望大类「%s」，实际「%s」", c.wantMj, got.SceneMajor)
			}
			if got.Source != "rule" {
				t.Fatalf("期望 source=rule，实际 %s", got.Source)
			}
			if got.Confidence <= 0 || got.Confidence > 0.95 {
				t.Fatalf("置信度应在 (0, 0.95]，实际 %.2f", got.Confidence)
			}
		})
	}
}

// TestClassifySceneFallback 空输入与无关键词时应稳定回落，不得 panic。
func TestClassifySceneFallback(t *testing.T) {
	for _, texts := range [][]string{
		nil,
		{""},
		{"   "},
		{"嗯"},
	} {
		got := ClassifyScene(texts)
		if got.SceneMinor == "" {
			t.Fatalf("回落结果不应为空（输入=%v）", texts)
		}
		if got.Source != "fallback" {
			t.Fatalf("无关键词时应回落，实际 source=%s scene=%s", got.Source, got.SceneMinor)
		}
	}
}

// TestClassifyScenePriority 高优先级场景应压过低优先级。
//
// ★ 这是一条「防退化」断言。初版打分用「关键词长度 × Priority」，
// 两个量纲相乘后不可比，`用了过敏`(4字) 会压过 `过敏`(2字)，
// 导致安全场景被降级成普通退货。现将安全场景设为过敏就医，锁死行为。
func TestClassifyScenePriority(t *testing.T) {
	cases := []struct {
		name  string
		texts []string
	}{
		{"过敏+退货", []string{"用了过敏了，我要退货"}},
		{"过敏+退货变体", []string{"用了之后过敏了，想退货"}},
		{"过敏+退货极简", []string{"过敏了退货"}},
		{"就医+退货", []string{"已经去医院了，这个必须退"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyScene(c.texts)
			if got.SceneMajor != "不良反应" {
				t.Fatalf("同句含过敏时，安全类场景必须压过普通退货，实际「%s/%s」（命中=%v）",
					got.SceneMajor, got.SceneMinor, got.Matched)
			}
			// 进一步：用药后过敏属于「过敏就医」而非泛泛的「使用不适退货」
			if got.SceneMinor != "过敏就医" {
				t.Errorf("期望「过敏就医」，实际「%s」", got.SceneMinor)
			}
		})
	}
}

// TestClassifySceneDeterministic 同输入必须同输出（可复现，可审计）。
func TestClassifySceneDeterministic(t *testing.T) {
	texts := []string{"东西有质量问题，而且物流很慢，客服也不理我"}
	first := ClassifyScene(texts)
	for i := 0; i < 20; i++ {
		again := ClassifyScene(texts)
		if again.SceneMinor != first.SceneMinor || again.SceneMajor != first.SceneMajor {
			t.Fatalf("第 %d 次结果不一致：%s/%s vs %s/%s",
				i, again.SceneMajor, again.SceneMinor, first.SceneMajor, first.SceneMinor)
		}
	}
}

// TestClassifySceneFeedsRiskLayer 端到端：分类结果能真正驱动风险判定第 2 层。
//
// 这是「分类器有用」的证明 —— 不是孤立单测，而是接入真实链路。
func TestClassifySceneFeedsRiskLayer(t *testing.T) {
	svc := &csRiskService{}

	// 场景：买家提到过敏就医
	texts := []string{"用完过敏了，已经去医院开了药"}
	scene := ClassifyScene(texts)
	if scene.SceneMinor == "" {
		t.Fatal("分类结果为空")
	}

	events := svc.DeriveRisks(texts, scene.SceneMinor, "")
	overall := svc.OverallRisk(events)

	if overall != "critical" {
		t.Fatalf("「过敏就医」应判为 critical，实际 %s（事件=%+v）", overall, events)
	}

	// 必须至少有一条来自场景层的证据
	hasSceneEvidence := false
	for _, e := range events {
		if e.Source == "scene" {
			hasSceneEvidence = true
		}
	}
	if !hasSceneEvidence {
		t.Fatalf("应产生 scene 来源的风险事件，实际 %+v", events)
	}
}

// TestSceneRiskLevelSemantics 断言场景等级符合语义口径，且同类场景不打架。
//
// ★ 这是「一致性断言」。曾经 `发票申请` 定为 low+30（→ low，上板）
// 而 `大促价保补差` 定为 medium+60（→ medium，上板），
// 但 `色号选择` low+10（→ none，不上板）—— 三个都是"答完即结束"的咨询，
// 却有两个上板一个不上。等级必须按 cs_models.go 的口径表来。
func TestSceneRiskLevelSemantics(t *testing.T) {
	// 口径：纯咨询、答完即结束 → 必须 none，绝不能出现在风险看板上
	pureConsult := []string{
		"使用方法", "色号选择", "套装搭配推荐", "好评回访",
		"发票申请", "积分兑换咨询",
	}
	for _, minor := range pureConsult {
		r, ok := sceneMinorToRisk[minor]
		if !ok {
			t.Errorf("场景「%s」不在权威表中", minor)
			continue
		}
		if r.Level != models.RiskLevelNone {
			t.Errorf("纯咨询场景「%s」应为 none（答完即结束，不上风险看板），实际 %s/%d",
				minor, r.Level, r.Score)
		}
	}

	// 口径：需要留痕但不主动介入 → low
	lowTouch := []string{"比价与决策", "物流查询", "优惠与赠品咨询"}
	for _, minor := range lowTouch {
		r := sceneMinorToRisk[minor]
		if r.Level != models.RiskLevelLow {
			t.Errorf("低接触场景「%s」应为 low，实际 %s/%d", minor, r.Level, r.Score)
		}
	}

	// 口径：score 与 level 必须自洽（scoreToLevel 是唯一裁判）
	for minor, r := range sceneMinorToRisk {
		if want := scoreToLevel(r.Score); want != r.Level {
			t.Errorf("场景「%s」的 level(%s) 与 score(%d→%s) 不自洽",
				minor, r.Level, r.Score, want)
		}
	}
}

// TestClassifySceneLowRisk 普通咨询不应误报高风险 —— 也不应上风险看板。
func TestClassifySceneLowRisk(t *testing.T) {
	svc := &csRiskService{}
	for _, c := range []struct {
		texts []string
		want  string
	}{
		{[]string{"这个怎么用呀"}, "none"},
		{[]string{"能开发票吗"}, "none"},
		{[]string{"黄皮选什么色号"}, "none"},
		{[]string{"积分怎么兑换"}, "none"},
	} {
		scene := ClassifyScene(c.texts)
		events := svc.DeriveRisks(c.texts, scene.SceneMinor, "")
		overall := svc.OverallRisk(events)
		if overall != c.want {
			t.Errorf("咨询类「%v」不应产生风险，期望 %s 实际 %s（场景=%s）",
				c.texts, c.want, overall, scene.SceneMinor)
		}
	}
}
