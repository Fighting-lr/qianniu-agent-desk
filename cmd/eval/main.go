// Package main —— 41 类场景分类器 + 风险等级的离线评估。
//
// 目的：用官方 ground truth（138 条会话的 scene_major/scene_minor 标注）
// 评估分类器的真实表现，产出混淆矩阵与 F1。
//
// 这是赛题里「评估体系」的核心交付物。上游 `AgentEvaluationService`
// 只有 Total/Passed 两个标量，回答不了"准不准、错在哪"。
//
// 用法：
//
//	go run ./cmd/eval -set ../build/eval_scene_set.json
//	go run ./cmd/eval -set ../build/eval_scene_set.json -json out.json
//	go run ./cmd/eval -set ... -show-confusion   # 打印完整混淆矩阵
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"agent-desk/internal/services"
)

// evalCase 对齐 export_eval_set.py 的输出结构。
type evalCase struct {
	SessionNo   string   `json:"sessionNo"`
	BuyerTexts  []string `json:"buyerTexts"`
	ExpectMajor string   `json:"expectMajor"`
	ExpectMinor string   `json:"expectMinor"`
}

type evalSet struct {
	Source            string     `json:"source"`
	TotalSessions     int        `json:"totalSessions"`
	DistinctMajor     int        `json:"distinctMajor"`
	DistinctMinor     int        `json:"distinctMinor"`
	MajorDistribution map[string]int `json:"majorDistribution"`
	MinorDistribution map[string]int `json:"minorDistribution"`
	Cases             []evalCase `json:"cases"`
}

func main() {
	defaultSet := "scripts/eval_scene_set.json"
	if _, err := os.Stat(defaultSet); err != nil {
		defaultSet = "../build/eval_scene_set.json"
	}
	setPath := flag.String("set", defaultSet, "评估集 JSON 路径")
	jsonOut := flag.String("json", "", "把完整报告额外写入此文件（可选）")
	showConfusion := flag.Bool("show-confusion", false, "打印完整混淆矩阵")
	topN := flag.Int("top", 10, "打印最差 N 个类")
	flag.Parse()

	if err := run(*setPath, *jsonOut, *showConfusion, *topN); err != nil {
		fmt.Fprintf(os.Stderr, "\n[FAIL] %v\n", err)
		os.Exit(1)
	}
}

func run(setPath, jsonOut string, showConfusion bool, topN int) error {
	raw, err := os.ReadFile(setPath)
	if err != nil {
		return fmt.Errorf("读取评估集失败: %w", err)
	}
	var es evalSet
	if err := json.Unmarshal(raw, &es); err != nil {
		return fmt.Errorf("解析评估集失败: %w", err)
	}
	if len(es.Cases) == 0 {
		return fmt.Errorf("评估集为空")
	}

	printHeader(&es)

	// ---- 转成评估器输入 ----
	cases := make([]services.EvaluationCase, 0, len(es.Cases))
	for _, c := range es.Cases {
		cases = append(cases, services.EvaluationCase{
			SessionNo:  c.SessionNo,
			BuyerTexts: c.BuyerTexts,
			ExpectMj:   c.ExpectMajor,
			ExpectMi:   c.ExpectMinor,
		})
	}

	major, minor, skipped := services.EvaluateSceneClassifier(cases)

	printReport("大类（10 类）", major, topN)
	printReport("子类（41 类）", minor, topN)

	if showConfusion {
		printConfusion("子类混淆矩阵", minor)
	}

	// ---- 覆盖率 ----
	fmt.Printf("\n%s\n", strings.Repeat("=", 72))
	fmt.Printf("覆盖率\n")
	fmt.Printf("%s\n", strings.Repeat("=", 72))
	evaluated := len(cases) - skipped
	fmt.Printf("  参与评估   : %d / %d 会话\n", evaluated, len(cases))
	fmt.Printf("  被跳过     : %d 会话（分类器返回 fallback，无判断）\n", skipped)
	if len(cases) > 0 {
		fmt.Printf("  规则命中率 : %.1f%%\n", float64(evaluated)/float64(len(cases))*100)
	}

	// ---- 风险等级评估（用预测出的场景喂入三层判定）----
	printRiskSection(cases)

	// ---- 导出 ----
	if jsonOut != "" {
		out := map[string]any{
			"major":   major,
			"minor":   minor,
			"skipped": skipped,
			"total":   len(cases),
		}
		buf, _ := json.MarshalIndent(out, "", "  ")
		if err := os.WriteFile(jsonOut, buf, 0o644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
		fmt.Printf("\n完整报告已写入 %s\n", jsonOut)
	}

	return nil
}

func printHeader(es *evalSet) {
	fmt.Printf("%s\n", strings.Repeat("=", 72))
	fmt.Printf("场景分类器评估 · 官方 ground truth\n")
	fmt.Printf("%s\n", strings.Repeat("=", 72))
	fmt.Printf("  数据来源   : %s\n", es.Source)
	fmt.Printf("  会话总数   : %d\n", es.TotalSessions)
	fmt.Printf("  大类覆盖   : %d / 10\n", es.DistinctMajor)
	fmt.Printf("  子类覆盖   : %d / 41\n", es.DistinctMinor)
}

func printReport(title string, r *services.EvaluationReport, topN int) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 72))
	fmt.Printf("%s\n", title)
	fmt.Printf("%s\n", strings.Repeat("=", 72))

	if r.Total == 0 {
		fmt.Printf("  无有效样本\n")
		return
	}

	fmt.Printf("  准确率          : %.4f  (%d/%d)\n", r.Accuracy, r.Correct, r.Total)
	fmt.Printf("  宏平均   P/R/F1 : %.4f / %.4f / %.4f\n",
		r.MacroPrecision, r.MacroRecall, r.MacroF1)
	fmt.Printf("  加权平均 P/R/F1 : %.4f / %.4f / %.4f\n",
		r.WeightedPrecision, r.WeightedRecall, r.WeightedF1)

	// 宏观 vs 微观落差提醒 —— 这是不均衡数据的核心信号
	gap := r.WeightedF1 - r.MacroF1
	if gap > 0.15 {
		fmt.Printf("  ⚠ 加权 F1 比宏 F1 高 %.4f —— 小类表现明显弱于大类（类别不均衡）\n", gap)
	}

	// 问题类（F1 升序，最差在前）
	fmt.Printf("\n  按 F1 升序（最需改进的排前面）:\n")
	fmt.Printf("    %-18s %6s %6s %6s %6s %6s %6s %6s\n",
		"类别", "样本", "预测", "TP", "FP", "FN", "P", "R")
	shown := 0
	for _, m := range r.Labels {
		if m.Support == 0 {
			continue // 凭空类单独在下面说
		}
		if shown >= topN {
			break
		}
		fmt.Printf("    %-18s %6d %6d %6d %6d %6d %6.3f %6.3f\n",
			truncate(m.Label, 18), m.Support, m.Predicted, m.TP, m.FP, m.FN, m.Precision, m.Recall)
		shown++
	}

	// 凭空类（预测出但官方没有）
	var phantom []services.ClassMetric
	for _, m := range r.Labels {
		if m.Support == 0 {
			phantom = append(phantom, m)
		}
	}
	if len(phantom) > 0 {
		fmt.Printf("\n  ⚠ 凭空类（官方无此生样本，但被预测出）:\n")
		for _, m := range phantom {
			fmt.Printf("    %-18s 误判 %d 次\n", truncate(m.Label, 18), m.FP)
		}
	}

	// 零召回类（全漏）
	var missed []string
	for _, m := range r.Labels {
		if m.Support > 0 && m.Recall == 0 {
			missed = append(missed, fmt.Sprintf("%s(%d条)", m.Label, m.Support))
		}
	}
	if len(missed) > 0 {
		fmt.Printf("\n  ⚠ 全部漏判的类: %s\n", strings.Join(missed, "、"))
	}

	// 低样本提醒
	if len(r.LowSupportLabels) > 0 {
		sort.Strings(r.LowSupportLabels)
		fmt.Printf("\n  ℹ 样本不足 %d 条的类（结论不可靠）: %d 个\n", 5, len(r.LowSupportLabels))
	}

	// 典型错判
	if len(r.TopConfusions) > 0 {
		fmt.Printf("\n  最典型错判方向:\n")
		for i, c := range r.TopConfusions {
			if i >= 8 {
				break
			}
			fmt.Printf("    %-16s → %-16s %d 次\n", truncate(c.Expected, 16), truncate(c.Actual, 16), c.Count)
		}
	}
}

func printConfusion(title string, r *services.EvaluationReport) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 72))
	fmt.Printf("%s（行=真实，列=预测，只显示有错判的行）\n", title)
	fmt.Printf("%s\n", strings.Repeat("=", 72))

	// 收集所有出现的标签
	labelSet := map[string]bool{}
	for exp, row := range r.Confusion {
		labelSet[exp] = true
		for act := range row {
			labelSet[act] = true
		}
	}
	var labels []string
	for l := range labelSet {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	// 只打印"本行有错判"的
	var rowsToShow []string
	for _, exp := range labels {
		hasErr := false
		for act, c := range r.Confusion[exp] {
			if act != exp && c > 0 {
				hasErr = true
				break
			}
		}
		if hasErr {
			rowsToShow = append(rowsToShow, exp)
		}
	}
	if len(rowsToShow) == 0 {
		fmt.Printf("  （无错判）\n")
		return
	}

	for _, exp := range rowsToShow {
		fmt.Printf("\n  【%s】被误判为:\n", exp)
		var acts []string
		for act := range r.Confusion[exp] {
			acts = append(acts, act)
		}
		sort.Slice(acts, func(i, j int) bool {
			return r.Confusion[exp][acts[i]] > r.Confusion[exp][acts[j]]
		})
		for _, act := range acts {
			if act == exp {
				continue
			}
			c := r.Confusion[exp][act]
			if c == 0 {
				continue
			}
			fmt.Printf("      %-20s %d 次\n", act, c)
		}
	}
}

// printRiskSection 用「预测出的场景」喂入三层判定，评估最终风险等级。
//
// 注意这评估的是**端到端风险链路**：分类器判错场景 → 风险等级也跟着错。
// 所以这里的结果通常比场景分类本身更差，这是符合预期的 ——
// 它衡量的是"最终交付给客服的东西有多准"。
func printRiskSection(cases []services.EvaluationCase) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 72))
	fmt.Printf("端到端风险等级（分类器 → 三层判定）\n")
	fmt.Printf("%s\n", strings.Repeat("=", 72))
	fmt.Printf("  说明：用「官方标注场景」与「分类器预测场景」分别跑一遍三层判定，\n")
	fmt.Printf("        对比两者给出的最终风险等级 —— 衡量场景误差如何传导到最终结论。\n\n")

	svc := services.CSRiskService
	var pairs []services.Pair
	var agree, total int

	levelCountsPred := map[string]int{}
	levelCountsTruth := map[string]int{}

	for _, c := range cases {
		// ground truth 场景 → 风险
		truthEvents := svc.DeriveRisks(c.BuyerTexts, c.ExpectMi, "")
		truthLevel := svc.OverallRisk(truthEvents)

		// 分类器预测场景 → 风险
		got := services.ClassifyScene(c.BuyerTexts)
		predEvents := svc.DeriveRisks(c.BuyerTexts, got.SceneMinor, "")
		predLevel := svc.OverallRisk(predEvents)

		pairs = append(pairs, services.Pair{Expected: truthLevel, Actual: predLevel})
		levelCountsTruth[truthLevel]++
		levelCountsPred[predLevel]++
		total++
		if truthLevel == predLevel {
			agree++
		}
	}

	report := services.EvaluateRiskLevel(pairs)
	fmt.Printf("  风险等级完全一致 : %d / %d  (%.1f%%)\n", agree, total,
		float64(agree)/float64(total)*100)

	rankOf := func(s string) int {
		switch s {
		case "critical":
			return 5
		case "high":
			return 4
		case "medium":
			return 3
		case "low":
			return 2
		case "none":
			return 1
		}
		return 0
	}
	adj := services.AdjacentToleranceAccuracy(pairs, rankOf)
	fmt.Printf("  允许差一档      : %.1f%%\n", adj*100)

	// ★ 关键安全指标：真值 critical 被降级的条数（漏报，最危险）
	downgraded := 0
	upgraded := 0
	for _, p := range pairs {
		if rankOf(p.Expected) > rankOf(p.Actual) {
			downgraded++
		} else if rankOf(p.Expected) < rankOf(p.Actual) {
			upgraded++
		}
	}
	fmt.Printf("\n  风险等级偏差方向:\n")
	fmt.Printf("    降级（漏报）: %d 条  ← 最危险，尤其 safety 类\n", downgraded)
	fmt.Printf("    升级（误报）: %d 条  ← 增加客服负担但不会漏\n", upgraded)

	fmt.Printf("\n  等级分布对比（真值 vs 预测）:\n")
	fmt.Printf("    %-10s %8s %8s\n", "等级", "真值", "预测")
	for _, lv := range []string{"critical", "high", "medium", "low", "none"} {
		fmt.Printf("    %-10s %8d %8d\n", lv, levelCountsTruth[lv], levelCountsPred[lv])
	}

	if report.TopConfusions != nil && len(report.TopConfusions) > 0 {
		fmt.Printf("\n  典型等级错判:\n")
		for i, c := range report.TopConfusions {
			if i >= 6 {
				break
			}
			fmt.Printf("    %-10s → %-10s %d 次\n", c.Expected, c.Actual, c.Count)
		}
	}
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}

var _ = io.Discard
