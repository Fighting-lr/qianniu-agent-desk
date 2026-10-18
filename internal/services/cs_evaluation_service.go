package services

import (
	"sort"
	"strings"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 评估体系（混淆矩阵 / Precision / Recall / F1）
//
// 为什么需要这个文件：
//   上游 `AgentEvaluationService` 只有 Total/Passed 两个标量 —— 它回答的是
//   "跑了几条、过了几条"，**回答不了赛题真正关心的"这个分类器准不准"**。
//   准确率之外还必须知道：哪些类被漏判（Recall）、哪些类被误判（Precision）、
//   错判具体错到哪去（混淆矩阵）。这三者是评分类任务的标准交付物。
//
// 设计原则：
//   - **纯函数、零依赖** —— 输入 [(期望, 实际)] 对，输出指标。不碰 DB、不调 LLM。
//     这样它既能评估场景分类器，也能评估风险等级判定，还能被单测直接覆盖。
//   - **不做宏平均的粉饰** —— 同时给 macro 与 weighted，因为 41 类严重不均衡
//     （产品咨询 180 条 vs 会员服务 17 条），只看 macro 会被小类拖低、
//     只看 weighted 会掩盖小类全错。
//   - **小类单独列出** —— 样本数低于阈值的类单独标出，避免"F1=0.9"的假象
//     （实际上 30 个类各只有 2 条样本）。
// ============================================================================

// ClassMetric 单个类别的评估指标。
type ClassMetric struct {
	Label     string  `json:"label"`
	Support   int     `json:"support"`   // 该类真实样本数
	Predicted int     `json:"predicted"` // 该类被预测出的次数
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

// EvaluationReport 完整评估报告。
type EvaluationReport struct {
	Total      int     `json:"total"`      // 样本总数
	Correct    int     `json:"correct"`    // 预测正确数
	Accuracy   float64 `json:"accuracy"`   // 总体准确率
	MacroF1    float64 `json:"macroF1"`    // 宏平均 F1（各类等权，暴露小类问题）
	WeightedF1 float64 `json:"weightedF1"` // 加权平均 F1（按样本数加权，反映大局）

	MacroPrecision    float64 `json:"macroPrecision"`
	MacroRecall       float64 `json:"macroRecall"`
	WeightedPrecision float64 `json:"weightedPrecision"`
	WeightedRecall    float64 `json:"weightedRecall"`

	Labels []ClassMetric `json:"labels"` // 每类明细，按 F1 升序（问题类排前面）

	// Confusion 混淆矩阵：Confusion[期望][实际] = 次数。
	// 行是真实标签，列是预测标签，符合教科书惯例。
	Confusion map[string]map[string]int `json:"confusion"`

	// TopConfusions 最典型的错判方向，方便快速定位问题。
	TopConfusions []ConfusionPair `json:"topConfusions"`

	// LowSupportLabels 样本过少的类（结论不可靠，需谨慎解读）。
	LowSupportLabels []string `json:"lowSupportLabels"`
}

// ConfusionPair 一对错判及其次数。
type ConfusionPair struct {
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Count    int    `json:"count"`
}

// Pair 一条 (期望, 实际) 配对。
type Pair struct {
	Expected string
	Actual   string
}

// lowSupportThreshold 样本数低于此值的类会被标记为"结论不可靠"。
const lowSupportThreshold = 5

// EvaluatePairs 对一组 (期望, 实际) 配对做完整评估。
//
// 空输入返回空报告（不 panic，不返回 nil），方便调用方直接序列化。
func EvaluatePairs(pairs []Pair) *EvaluationReport {
	report := &EvaluationReport{
		Confusion: map[string]map[string]int{},
	}
	if len(pairs) == 0 {
		return report
	}

	report.Total = len(pairs)

	// 收集全部标签（期望与实际都算，这样"凭空造出的类"也能被看见）
	labelSet := map[string]bool{}
	support := map[string]int{}
	predicted := map[string]int{}
	for _, p := range pairs {
		exp := normalizeLabel(p.Expected)
		act := normalizeLabel(p.Actual)
		labelSet[exp] = true
		labelSet[act] = true
		support[exp]++
		predicted[act]++

		if report.Confusion[exp] == nil {
			report.Confusion[exp] = map[string]int{}
		}
		report.Confusion[exp][act]++

		if exp == act {
			report.Correct++
		}
	}

	if report.Total > 0 {
		report.Accuracy = float64(report.Correct) / float64(report.Total)
	}

	// ---- 逐类计算 ----
	// 注意：这里遍历 labelSet 的顺序是不确定的（map），
	// 所以最终必须显式排序 —— 否则报告在不同次运行间会抖动。
	labels := make([]string, 0, len(labelSet))
	for l := range labelSet {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	metrics := make([]ClassMetric, 0, len(labels))
	for _, l := range labels {
		tp := report.Confusion[l][l]
		m := ClassMetric{
			Label:     l,
			Support:   support[l],
			Predicted: predicted[l],
			TP:        tp,
			FP:        predicted[l] - tp,
			FN:        support[l] - tp,
		}
		m.Precision = safeDivide(float64(m.TP), float64(m.TP+m.FP))
		m.Recall = safeDivide(float64(m.TP), float64(m.TP+m.FN))
		m.F1 = harmonicMean(m.Precision, m.Recall)
		metrics = append(metrics, m)

		if m.Support > 0 && m.Support < lowSupportThreshold {
			report.LowSupportLabels = append(report.LowSupportLabels, l)
		}
	}

	// ---- 平均 ----
	// macro：注意要跳过 support==0 的类（它们只因"被误判成它"而出现在标签集里，
	// 不是真实存在的类，计入平均会稀释指标）。
	var macroP, macroR, macroF1, sumSupport float64
	var counted int
	var wP, wR, wF1 float64
	for _, m := range metrics {
		if m.Support == 0 {
			continue
		}
		macroP += m.Precision
		macroR += m.Recall
		macroF1 += m.F1
		counted++
		w := float64(m.Support)
		sumSupport += w
		wP += m.Precision * w
		wR += m.Recall * w
		wF1 += m.F1 * w
	}
	if counted > 0 {
		report.MacroPrecision = macroP / float64(counted)
		report.MacroRecall = macroR / float64(counted)
		report.MacroF1 = macroF1 / float64(counted)
	}
	if sumSupport > 0 {
		report.WeightedPrecision = wP / sumSupport
		report.WeightedRecall = wR / sumSupport
		report.WeightedF1 = wF1 / sumSupport
	}

	// 明细按 F1 升序 —— 最差的排最前面，报告是"待办清单"而非"成绩单"
	sort.SliceStable(metrics, func(i, j int) bool {
		if metrics[i].F1 != metrics[j].F1 {
			return metrics[i].F1 < metrics[j].F1
		}
		return metrics[i].Label < metrics[j].Label
	})
	report.Labels = metrics

	// ---- 典型错判（只看 exp != act）----
	var pairsList []ConfusionPair
	for exp, row := range report.Confusion {
		for act, cnt := range row {
			if exp == act || cnt == 0 {
				continue
			}
			pairsList = append(pairsList, ConfusionPair{Expected: exp, Actual: act, Count: cnt})
		}
	}
	sort.SliceStable(pairsList, func(i, j int) bool {
		if pairsList[i].Count != pairsList[j].Count {
			return pairsList[i].Count > pairsList[j].Count
		}
		if pairsList[i].Expected != pairsList[j].Expected {
			return pairsList[i].Expected < pairsList[j].Expected
		}
		return pairsList[i].Actual < pairsList[j].Actual
	})
	if len(pairsList) > 10 {
		pairsList = pairsList[:10]
	}
	report.TopConfusions = pairsList

	return report
}

// normalizeLabel 统一标签格式（去空格），避免"泛红刺痒 "与"泛红刺痒"被当成两类。
func normalizeLabel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(空)"
	}
	return s
}

// safeDivide 除零返回 0（而非 NaN/Inf，保证可序列化）。
//
// 注意语义：分母为 0 表示"该类从未出现也从未被预测"，
// 返回 0 而非 1 —— 宁可保守，不给不存在的类送分。
func safeDivide(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den
}

// harmonicMean 调和平均。任一为 0 则结果为 0（F1 的标准定义）。
func harmonicMean(a, b float64) float64 {
	if a+b == 0 {
		return 0
	}
	return 2 * a * b / (a + b)
}

// ============================================================================
// 面向本项目的便捷封装
// ============================================================================

// EvaluationCase 一条评估样本：买家原声 + 官方标注场景。
type EvaluationCase struct {
	SessionNo  string
	BuyerTexts []string
	ExpectMj   string
	ExpectMi   string
}

// EvaluateSceneClassifier 用官方标注评估 41 类场景分类器的表现。
//
// 这是「评估体系」的核心交付物：直接回答"分类器在 41 类上有多准"。
// 输入是官方 ground truth（scene_major/scene_minor），
// 输出是两级混淆矩阵（大类 + 子类）与 F1。
func EvaluateSceneClassifier(cases []EvaluationCase) (major *EvaluationReport, minor *EvaluationReport, skipped int) {
	var majorPairs, minorPairs []Pair
	for _, c := range cases {
		got := ClassifyScene(c.BuyerTexts)
		if got.Source != "rule" {
			// fallback 不参与评估 —— 分类器没给出判断，
			// 把它算成"错"会低估真实能力，算成"对"会高估。
			// 单独计数让调用方知道覆盖面。
			skipped++
			continue
		}
		majorPairs = append(majorPairs, Pair{Expected: c.ExpectMj, Actual: got.SceneMajor})
		minorPairs = append(minorPairs, Pair{Expected: c.ExpectMi, Actual: got.SceneMinor})
	}
	return EvaluatePairs(majorPairs), EvaluatePairs(minorPairs), skipped
}

// EvaluateRiskLevel 评估风险等级判定的表现。
//
// 与场景分类不同，风险等级是**有序标签**（none < low < medium < high < critical），
// 严格相等会过于苛刻（把 critical 判成 high 是"差一档"而非"全错"）。
// 因此额外提供「相邻容差」口径：偏差 ≤1 档算作可接受。
func EvaluateRiskLevel(pairs []Pair) *EvaluationReport {
	return EvaluatePairs(pairs)
}

// AdjacentToleranceAccuracy 计算"允许错一档"的准确率。
//
// 用于风险等级这种有序标签：critical↔high 的偏差远比 critical↔none 轻微。
// 但对本项目有个**重要例外**：安全类风险（safety）错判不可容忍，
// 因为漏报 critical 的后果是法律风险，不是体验问题。
// 所以这个指标只用于观察，不能替代严格准确率做验收。
func AdjacentToleranceAccuracy(pairs []Pair, rankOf func(string) int) float64 {
	if len(pairs) == 0 {
		return 0
	}
	ok := 0
	for _, p := range pairs {
		if absInt(rankOf(p.Actual)-rankOf(p.Expected)) <= 1 {
			ok++
		}
	}
	return float64(ok) / float64(len(pairs))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// [AgentDesk-END]
