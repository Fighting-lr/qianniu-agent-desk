package services

import (
	"math"
	"testing"
)

// ============================================================================
// [AgentDesk-BEGIN] 评估体系测试
//
// 评估器本身必须被验证 —— 否则它会给出错误结论，且没人发现。
// 这里的重点是**用已知答案的算例反推**，而不是只看"跑通了没"。
// ============================================================================

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestEvaluatePairsPerfect 全对时所有指标应为 1。
func TestEvaluatePairsPerfect(t *testing.T) {
	pairs := []Pair{
		{"A", "A"}, {"A", "A"}, {"B", "B"}, {"C", "C"}, {"C", "C"},
	}
	r := EvaluatePairs(pairs)

	if r.Total != 5 || r.Correct != 5 {
		t.Fatalf("总数/正确数错：total=%d correct=%d", r.Total, r.Correct)
	}
	if !almostEqual(r.Accuracy, 1.0) {
		t.Errorf("准确率应为 1，实际 %.6f", r.Accuracy)
	}
	if !almostEqual(r.MacroF1, 1.0) {
		t.Errorf("宏 F1 应为 1，实际 %.6f", r.MacroF1)
	}
	if !almostEqual(r.WeightedF1, 1.0) {
		t.Errorf("加权 F1 应为 1，实际 %.6f", r.WeightedF1)
	}
	if len(r.TopConfusions) != 0 {
		t.Errorf("全对时不应有错判，实际 %v", r.TopConfusions)
	}
}

// TestEvaluatePairsKnownAnswer 用**手算得出的答案**校验，这是最关键的一条。
//
// 构造算例（3 个类，6 条样本）：
//
//	期望: A A A B B C
//	实际: A A B B C C
//
// 混淆矩阵：
//
//	     A  B  C
//	A  [ 2  1  0 ]   support(A)=3
//	B  [ 0  1  1 ]   support(B)=2
//	C  [ 0  0  1 ]   support(C)=1
//
// 逐类手算：
//
//	A: TP=2 FP=0 FN=1 → P=1.0    R=2/3      F1=2*(1*2/3)/(1+2/3)=0.8
//	B: TP=1 FP=1 FN=1 → P=0.5    R=0.5      F1=0.5
//	C: TP=1 FP=1 FN=0 → P=0.5    R=1.0      F1=2*(0.5*1)/(1.5)=0.6666...
//
// 准确率 = 4/6 = 0.6666...
// macroF1 = (0.8 + 0.5 + 0.66666...)/3 = 1.966666.../3 = 0.65555...
// weightedF1 = (0.8*3 + 0.5*2 + 0.66666*1)/6 = (2.4+1+0.66666)/6 = 4.06666/6 = 0.67777...
func TestEvaluatePairsKnownAnswer(t *testing.T) {
	pairs := []Pair{
		{"A", "A"}, {"A", "A"}, {"A", "B"},
		{"B", "B"}, {"B", "C"},
		{"C", "C"},
	}
	r := EvaluatePairs(pairs)

	if r.Total != 6 || r.Correct != 4 {
		t.Fatalf("total=%d correct=%d（应为 6/4）", r.Total, r.Correct)
	}
	if !almostEqual(r.Accuracy, 4.0/6.0) {
		t.Errorf("准确率应为 %.6f，实际 %.6f", 4.0/6.0, r.Accuracy)
	}

	// 逐类核对
	want := map[string]ClassMetric{
		"A": {Label: "A", Support: 3, Predicted: 2, TP: 2, FP: 0, FN: 1, Precision: 1.0, Recall: 2.0 / 3.0, F1: 0.8},
		"B": {Label: "B", Support: 2, Predicted: 2, TP: 1, FP: 1, FN: 1, Precision: 0.5, Recall: 0.5, F1: 0.5},
		"C": {Label: "C", Support: 1, Predicted: 2, TP: 1, FP: 1, FN: 0, Precision: 0.5, Recall: 1.0, F1: 2.0 / 3.0},
	}
	for _, m := range r.Labels {
		w := want[m.Label]
		if m.Support != w.Support || m.Predicted != w.Predicted || m.TP != w.TP || m.FP != w.FP || m.FN != w.FN {
			t.Errorf("%s 计数不符：got support=%d pred=%d TP=%d FP=%d FN=%d；want %d/%d/%d/%d/%d",
				m.Label, m.Support, m.Predicted, m.TP, m.FP, m.FN,
				w.Support, w.Predicted, w.TP, w.FP, w.FN)
		}
		if !almostEqual(m.Precision, w.Precision) {
			t.Errorf("%s Precision 应为 %.6f，实际 %.6f", m.Label, w.Precision, m.Precision)
		}
		if !almostEqual(m.Recall, w.Recall) {
			t.Errorf("%s Recall 应为 %.6f，实际 %.6f", m.Label, w.Recall, m.Recall)
		}
		if !almostEqual(m.F1, w.F1) {
			t.Errorf("%s F1 应为 %.6f，实际 %.6f", m.Label, w.F1, m.F1)
		}
	}

	if !almostEqual(r.MacroF1, (0.8+0.5+2.0/3.0)/3.0) {
		t.Errorf("宏 F1 应为 %.6f，实际 %.6f", (0.8+0.5+2.0/3.0)/3.0, r.MacroF1)
	}
	if !almostEqual(r.WeightedF1, (0.8*3+0.5*2+(2.0/3.0)*1)/6.0) {
		t.Errorf("加权 F1 应为 %.6f，实际 %.6f", (0.8*3+0.5*2+(2.0/3.0)*1)/6.0, r.WeightedF1)
	}

	// 混淆矩阵方向：行是期望，列是实际
	if got := r.Confusion["A"]["B"]; got != 1 {
		t.Errorf("Confusion[期望A][实际B] 应为 1，实际 %d", got)
	}
	if got := r.Confusion["B"]["A"]; got != 0 {
		t.Errorf("Confusion[期望B][实际A] 应为 0，实际 %d", got)
	}

	// 最典型错判：非对角元只有 A→B 与 B→C 两组（各 1 次）
	// 注意：C 行没有非对角元（C 全判对），B→C 只有一条，别数成两条。
	if len(r.TopConfusions) != 2 {
		t.Fatalf("应有 2 组错判，实际 %d 组：%v", len(r.TopConfusions), r.TopConfusions)
	}
	if r.TopConfusions[0].Expected != "A" || r.TopConfusions[0].Actual != "B" {
		t.Errorf("首条错判应为 A→B，实际 %v", r.TopConfusions[0])
	}
	if r.TopConfusions[1].Expected != "B" || r.TopConfusions[1].Actual != "C" {
		t.Errorf("次条错判应为 B→C，实际 %v", r.TopConfusions[1])
	}
}

// TestEvaluatePairsEmpty 空输入必须安全返回空报告，不能 panic、不能 nil。
func TestEvaluatePairsEmpty(t *testing.T) {
	r := EvaluatePairs(nil)
	if r == nil {
		t.Fatal("空输入不应返回 nil")
	}
	if r.Total != 0 || r.Accuracy != 0 {
		t.Errorf("空输入应全零，实际 total=%d acc=%.3f", r.Total, r.Accuracy)
	}
	if r.Confusion == nil {
		t.Error("Confusion 不应为 nil（下游要直接序列化）")
	}
}

// TestEvaluatePairsNeverPredictsClass 真实场景：某类样本存在但从没被预测出来。
//
// 这正是 41 类分类器最可能的失败模式 —— 稀有类（如「积分兑换咨询」）
// 因为关键词太少而永不命中。必须能在报告里一眼看出来。
func TestEvaluatePairsNeverPredictsClass(t *testing.T) {
	pairs := []Pair{
		{"rare", "common"}, // 稀有类被误判成常见类
		{"common", "common"},
		{"common", "common"},
	}
	r := EvaluatePairs(pairs)

	var rare ClassMetric
	found := false
	for _, m := range r.Labels {
		if m.Label == "rare" {
			rare = m
			found = true
		}
	}
	if !found {
		t.Fatal("报告里应包含「rare」类（哪怕它从未被预测出）")
	}
	if rare.Predicted != 0 {
		t.Errorf("rare 从未被预测，Predicted 应为 0，实际 %d", rare.Predicted)
	}
	if rare.Recall != 0 {
		t.Errorf("rare 全部漏判，Recall 应为 0，实际 %.6f", rare.Recall)
	}
	if rare.F1 != 0 {
		t.Errorf("rare 的 F1 应为 0，实际 %.6f", rare.F1)
	}
	if rare.FN != 1 {
		t.Errorf("rare 应有 1 个漏判，实际 %d", rare.FN)
	}
}

// TestEvaluatePairsPhantomClass 凭空造出的类（实际预测有、期望从未出现）。
//
// 对应真实缺陷：分类器自创官方 41 类之外的场景。
// 这类情况必须能被看见 —— 它的 Precision 应为 0 而不是被忽略。
func TestEvaluatePairsPhantomClass(t *testing.T) {
	pairs := []Pair{
		{"A", "phantom"},
		{"A", "A"},
	}
	r := EvaluatePairs(pairs)

	var ph ClassMetric
	for _, m := range r.Labels {
		if m.Label == "phantom" {
			ph = m
		}
	}
	if ph.Label == "" {
		t.Fatal("凭空类应出现在报告里")
	}
	if ph.Support != 0 {
		t.Errorf("phantom 无真实样本，Support 应为 0，实际 %d", ph.Support)
	}
	if ph.FP != 1 {
		t.Errorf("phantom 应有 1 个误判，实际 %d", ph.FP)
	}
	if ph.Precision != 0 {
		t.Errorf("phantom 全部预测都是错的，Precision 应为 0，实际 %.6f", ph.Precision)
	}
	// 关键：support=0 的类不能被算进宏平均（会稀释指标）
	if r.MacroF1 >= 1.0 {
		t.Errorf("宏平均不应被 support=0 的类拉高，实际 %.6f", r.MacroF1)
	}
}

// TestEvaluatePairsDeterministic 同输入必须同输出。
//
// 因为内部遍历 map，如果忘了排序，Labels 顺序会在多次运行间抖动 ——
// 报告抖动会导致"昨天 F1 排第一的问题类今天不见了"。
func TestEvaluatePairsDeterministic(t *testing.T) {
	pairs := []Pair{
		{"C", "A"}, {"A", "B"}, {"B", "C"}, {"D", "D"}, {"A", "A"},
	}
	first := EvaluatePairs(pairs)

	for i := 0; i < 30; i++ {
		again := EvaluatePairs(pairs)
		if len(again.Labels) != len(first.Labels) {
			t.Fatalf("第 %d 次 Labels 数量不一致", i)
		}
		for j := range first.Labels {
			if again.Labels[j].Label != first.Labels[j].Label {
				t.Fatalf("第 %d 次 Labels[%d] 顺序不一致：%s vs %s",
					i, j, again.Labels[j].Label, first.Labels[j].Label)
			}
		}
		if len(again.TopConfusions) != len(first.TopConfusions) {
			t.Fatalf("第 %d 次 TopConfusions 数量不一致", i)
		}
		for j := range first.TopConfusions {
			if again.TopConfusions[j] != first.TopConfusions[j] {
				t.Fatalf("第 %d 次 TopConfusions[%d] 不一致：%v vs %v",
					i, j, again.TopConfusions[j], first.TopConfusions[j])
			}
		}
	}
}

// TestEvaluatePairsLowSupport 样本过少的类必须被标记出来。
func TestEvaluatePairsLowSupport(t *testing.T) {
	var pairs []Pair
	// big 类 10 条全对
	for i := 0; i < 10; i++ {
		pairs = append(pairs, Pair{"big", "big"})
	}
	// tiny 类 2 条（低于阈值 5）
	pairs = append(pairs, Pair{"tiny", "tiny"}, Pair{"tiny", "big"})

	r := EvaluatePairs(pairs)

	sawTiny := false
	for _, l := range r.LowSupportLabels {
		if l == "tiny" {
			sawTiny = true
		}
		if l == "big" {
			t.Errorf("big 有 10 条样本，不应被标为低样本")
		}
	}
	if !sawTiny {
		t.Errorf("tiny 只有 2 条样本，应被标记，实际标记了 %v", r.LowSupportLabels)
	}
}

// TestEvaluatePairsLabelNormalization 标签首尾空格必须归一，否则同义类被拆成两个。
func TestEvaluatePairsLabelNormalization(t *testing.T) {
	pairs := []Pair{
		{"泛红刺痒", "泛红刺痒 "},
		{" 泛红刺痒", "泛红刺痒"},
	}
	r := EvaluatePairs(pairs)

	if r.Correct != 2 {
		t.Errorf("去空格后应全对，实际正确 %d 条（Labels=%v）", r.Correct, r.Labels)
	}
	// 只应有一个标签
	distinct := map[string]bool{}
	for _, m := range r.Labels {
		distinct[m.Label] = true
	}
	if len(distinct) != 1 {
		t.Errorf("应归并为 1 个标签，实际 %d 个：%v", len(distinct), distinct)
	}
}

// TestEvaluatePairsWeightedVsMacro 验证两个平均口径确实不同，且各有用途。
//
// 构造极端不均衡：大类 100 条全对，小类 1 条判错。
//
// 混淆矩阵：
//
//	          major  minority
//	major    [  100      0   ]
//	minority [    1      0   ]
//
//	major:    TP=100 FP=1 FN=0 → P=100/101 R=1  F1=0.995025
//	minority: TP=0   FP=0 FN=1 → P=0       R=0  F1=0
//
//	macroF1    = (0.995025 + 0)/2 = 0.497512   ← 暴露小类全错
//	weightedF1 = (0.995025*100 + 0*1)/101 ≈ 0.9851 ← 掩盖小类全错
//
// 这正是本项目必须同时报两个数的原因：
// 官方数据里「产品咨询」180 条 vs「会员服务」17 条，差距 10 倍以上。
func TestEvaluatePairsWeightedVsMacro(t *testing.T) {
	var pairs []Pair
	for i := 0; i < 100; i++ {
		pairs = append(pairs, Pair{"major", "major"})
	}
	pairs = append(pairs, Pair{"minority", "major"})

	r := EvaluatePairs(pairs)

	// 注意：minority 的 F1 为 0，但 major 的 F1 并非 1（它有 1 个 FP）
	// 所以宏平均是 0.497512，不是 0.5。这类"看起来该是整数"的地方
	// 最容易写错期望值 —— 记录下来避免下次再踩。
	if !almostEqual(r.MacroF1, 0.49751243781094523) {
		t.Errorf("宏 F1 应为 %.6f，实际 %.6f", 0.49751243781094523, r.MacroF1)
	}
	if r.WeightedF1 < 0.98 {
		t.Errorf("加权 F1 应接近 1（小类被淹没），实际 %.6f", r.WeightedF1)
	}
	if r.MacroF1 >= r.WeightedF1 {
		t.Errorf("不均衡数据下宏 F1(%f) 应低于加权 F1(%f)", r.MacroF1, r.WeightedF1)
	}
}

// TestEvaluateSceneClassifierSkipsFallback fallback 样本不应污染指标，但要单独计数。
func TestEvaluateSceneClassifierSkipsFallback(t *testing.T) {
	cases := []EvaluationCase{
		{SessionNo: "S1", BuyerTexts: []string{"用了过敏了，已经去医院了"}, ExpectMj: "不良反应", ExpectMi: "过敏就医"},
		// 注意：发票申请的官方大类是「订单服务」，不是「咨询」—— 别按直觉填。
		{SessionNo: "S2", BuyerTexts: []string{"能开发票吗"}, ExpectMj: "订单服务", ExpectMi: "发票申请"},
		{SessionNo: "S3", BuyerTexts: []string{"嗯嗯好的"}, ExpectMj: "产品咨询", ExpectMi: "使用方法"}, // fallback
	}

	major, minor, skipped := EvaluateSceneClassifier(cases)

	if skipped != 1 {
		t.Errorf("应有 1 条 fallback 被跳过，实际 %d", skipped)
	}
	if major.Total != 2 || minor.Total != 2 {
		t.Errorf("评估样本应为 2（不含 fallback），实际 major=%d minor=%d", major.Total, minor.Total)
	}
	// 前两条应判对
	if major.Correct != 2 {
		t.Errorf("两条规则命中样本应全对，实际正确 %d（labels=%v）", major.Correct, major.Labels)
	}
	if minor.Correct != 2 {
		t.Errorf("子类也应全对，实际正确 %d（labels=%v）", minor.Correct, minor.Labels)
	}
}

// TestAdjacentToleranceAccuracy 相邻容差只放宽一档，不能变成"随便判都算对"。
func TestAdjacentToleranceAccuracy(t *testing.T) {
	rank := func(s string) int {
		switch s {
		case "none":
			return 1
		case "low":
			return 2
		case "medium":
			return 3
		case "high":
			return 4
		case "critical":
			return 5
		}
		return 0
	}

	pairs := []Pair{
		{"critical", "high"},   // 差 1 档 → 容差内
		{"critical", "medium"}, // 差 2 档 → 容差外
		{"low", "low"},         // 完全对 → 容差内
		{"none", "critical"},   // 差 4 档 → 容差外
	}
	got := AdjacentToleranceAccuracy(pairs, rank)
	if !almostEqual(got, 0.5) {
		t.Errorf("相邻容差准确率应为 0.5，实际 %.6f", got)
	}

	// 空输入
	if AdjacentToleranceAccuracy(nil, rank) != 0 {
		t.Error("空输入应返回 0")
	}
}
