package workflow

import (
	"testing"

	"agent-desk/internal/ai/workflow/dsl"
	"agent-desk/internal/models"
)

// ============================================================================
// [AgentDesk-BEGIN] risk_guard 节点 · 场景自动推断的接入测试
//
// 这组测试证明的是**链路价值**，不是单个函数：
//   实时聊天时上游不提供 sceneMinor，
//   节点必须自己从买家原声推断出来，否则风险判定第 2 层永远是空的。
// ============================================================================

// newRiskGuardState 构造一个只含 risk_guard 节点、输入全靠常量注入的 runState。
//
// 注意：必须用 dsl.ConstantValue() 构造，不能直接写 dsl.Value{ConstantContent: v}——
// 后者 Type 是零值 ""，resolveValue 会走 default 分支返回 nil，输入静默变空。
func newRiskGuardState(t *testing.T, inputs map[string]string) (*runState, dsl.Node) {
	t.Helper()

	values := make(map[string]dsl.Value, len(inputs))
	for k, v := range inputs {
		values[k] = dsl.ConstantValue(v)
	}
	return newRiskGuardStateWithValues(t, values)
}

func newRiskGuardStateWithValues(t *testing.T, values map[string]dsl.Value) (*runState, dsl.Node) {
	t.Helper()

	node := dsl.Node{
		ID:   "guard_1",
		Type: "risk_guard",
		Meta: dsl.NodeMeta{Position: dsl.Position{X: 0, Y: 0}},
		Data: dsl.NodeData{
			Title:        "Risk Guard",
			InputsValues: values,
		},
	}

	state := newRunState(Input{
		Definition: dsl.Definition{
			Nodes: []dsl.Node{node},
		},
		UserMessage: models.Message{},
	})
	return state, node
}

// TestRiskGuardDerivesSceneWhenUpstreamOmits 上游不给 sceneMinor 时必须自动推断。
//
// 这是整条改动的**核心断言**：官方数据里 sceneMinor 只在离线导入时存在，
// 线上实时聊天没有。如果节点不推断，第 2 层（41 类场景映射）在线上等于白写。
func TestRiskGuardDerivesSceneWhenUpstreamOmits(t *testing.T) {
	e := NewExecutor()

	cases := []struct {
		name       string
		msg        string
		wantLevel  string
		wantMinor  string
		wantMajor  string
		wantSource string
	}{
		{
			name:       "过敏就医",
			msg:        "用了之后脸过敏了，已经去医院看过了",
			wantLevel:  models.RiskLevelCritical,
			wantMinor:  "过敏就医",
			wantMajor:  "不良反应",
			wantSource: "rule",
		},
		{
			name:       "物流停滞疑似丢件",
			msg:        "我包裹物流一直不动，是不是丢件了",
			wantMinor:  "物流停滞疑似丢件",
			wantMajor:  "物流异常",
			wantSource: "rule",
		},
		{
			name:       "退款迟迟不到账",
			msg:        "退款迟迟不到账，钱一直没到我卡上",
			wantMinor:  "退款迟迟不到账",
			wantMajor:  "订单服务",
			wantSource: "rule",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 刻意**不传** sceneMinor —— 模拟实时聊天
			state, node := newRiskGuardState(t, map[string]string{
				"userMessage": c.msg,
			})

			if err := e.executeRiskGuard(state, node); err != nil {
				t.Fatalf("节点执行失败：%v", err)
			}

			vars := state.vars[node.ID]
			if vars == nil {
				t.Fatal("节点未写入任何变量")
			}

			if got := vars["sceneMinor"]; got != c.wantMinor {
				t.Errorf("sceneMinor 期望「%s」，实际「%v」", c.wantMinor, got)
			}
			if got := vars["sceneMajor"]; got != c.wantMajor {
				t.Errorf("sceneMajor 期望「%s」，实际「%v」", c.wantMajor, got)
			}
			if got := vars["sceneSource"]; got != c.wantSource {
				t.Errorf("sceneSource 期望「%s」，实际「%v」", c.wantSource, got)
			}
			if c.wantLevel != "" {
				if got := vars["riskLevel"]; got != c.wantLevel {
					t.Errorf("riskLevel 期望「%s」，实际「%v」", c.wantLevel, got)
				}
			}
			// fallback 一律不采信
			if vars["sceneSource"] == "fallback" {
				t.Error("fallback 场景不应被采信")
			}
		})
	}
}

// TestRiskGuardPrefersUpstreamScene 上游显式给定 sceneMinor 时，必须优先采信。
//
// 理由：离线导入场景下官方标注一定比规则准，规则不能覆盖事实。
func TestRiskGuardPrefersUpstreamScene(t *testing.T) {
	e := NewExecutor()

	state, node := newRiskGuardState(t, map[string]string{
		"userMessage": "随便说点什么",
		"sceneMinor":  "直播间承诺纠纷", // 上游给定，且原声里毫无线索
	})

	if err := e.executeRiskGuard(state, node); err != nil {
		t.Fatalf("节点执行失败：%v", err)
	}

	vars := state.vars[node.ID]
	if got := vars["sceneMinor"]; got != "直播间承诺纠纷" {
		t.Errorf("应采信上游 sceneMinor，实际「%v」", got)
	}
	if got := vars["sceneSource"]; got != "provided" {
		t.Errorf("sceneSource 应为 provided，实际「%v」", got)
	}
	if got := vars["sceneMajor"]; got != "订单服务" {
		t.Errorf("应由子类补出大类「订单服务」，实际「%v」", got)
	}
	if got := vars["sceneConf"]; got != 1.0 {
		t.Errorf("上游给定的置信度应为 1.0，实际「%v」", got)
	}
}

// TestRiskGuardIgnoresFallbackScene 无关键词时必须**不判**，而不是拿兜底场景污染链路。
func TestRiskGuardIgnoresFallbackScene(t *testing.T) {
	e := NewExecutor()

	state, node := newRiskGuardState(t, map[string]string{
		"userMessage": "嗯嗯好的",
	})

	if err := e.executeRiskGuard(state, node); err != nil {
		t.Fatalf("节点执行失败：%v", err)
	}

	vars := state.vars[node.ID]
	if got := vars["sceneMinor"]; got != "" {
		t.Errorf("无信号时不应给出场景，实际「%v」", got)
	}
	if got := vars["riskLevel"]; got != models.RiskLevelNone {
		t.Errorf("无信号时应为 none，实际「%v」", got)
	}
}

// TestRiskGuardSceneDrivesNoFalsePositive 场景推断不得把普通咨询推成高风险。
//
// 反向保险：分类器 + 风险链路合起来，必须对"能开发票吗"这类问题保持冷静。
func TestRiskGuardSceneDrivesNoFalsePositive(t *testing.T) {
	e := NewExecutor()

	for _, msg := range []string{
		"能开发票吗",
		"这个怎么用呀",
		"黄皮选什么色号",
		"积分怎么兑换",
	} {
		t.Run(msg, func(t *testing.T) {
			state, node := newRiskGuardState(t, map[string]string{
				"userMessage": msg,
			})
			if err := e.executeRiskGuard(state, node); err != nil {
				t.Fatalf("节点执行失败：%v", err)
			}
			vars := state.vars[node.ID]
			if got := vars["riskLevel"]; got != models.RiskLevelNone {
				t.Errorf("普通咨询「%s」不应产生风险，实际 %v（场景=%v/%v）",
					msg, got, vars["sceneMajor"], vars["sceneMinor"])
			}
			if got := vars["needHuman"]; got != false {
				t.Errorf("普通咨询「%s」不应转人工", msg)
			}
		})
	}
}

// TestRiskGuardSceneUpgradesRisk 场景推断必须真的把等级"抬"起来。
//
// 同一个问题，有场景 vs 无场景，风险等级应当不同 ——
// 这是第 2 层确实生效的证明。
func TestRiskGuardSceneUpgradesRisk(t *testing.T) {
	e := NewExecutor()

	// 「孕妇可以用吗」本身没有任何硬风险词，但场景是安全类（high）
	msg := "我是孕妇，这个能用吗"

	// 有场景（自动推断）
	state, node := newRiskGuardState(t, map[string]string{"userMessage": msg})
	if err := e.executeRiskGuard(state, node); err != nil {
		t.Fatalf("节点执行失败：%v", err)
	}
	vars := state.vars[node.ID]

	if got := vars["sceneMinor"]; got != "孕妇可用咨询" {
		t.Fatalf("应推断出「孕妇可用咨询」，实际「%v」", got)
	}
	rank := models.RiskLevelRank(vars["riskLevel"].(string))
	if rank < models.RiskLevelRank(models.RiskLevelHigh) {
		t.Errorf("孕妇咨询属安全口径问答，应至少 high，实际「%v」", vars["riskLevel"])
	}

	// 对照组：同一节点配置下，把原声换成完全无信号的文本，
	// 等级必须掉下来 —— 证明上面那条 high 确实来自场景层，而非别的通路。
	state2, node2 := newRiskGuardState(t, map[string]string{
		"userMessage": "嗯",
	})
	if err := e.executeRiskGuard(state2, node2); err != nil {
		t.Fatalf("节点执行失败：%v", err)
	}
	vars2 := state2.vars[node2.ID]
	if models.RiskLevelRank(vars2["riskLevel"].(string)) >= models.RiskLevelRank(models.RiskLevelHigh) {
		t.Errorf("无信号文本不应达到 high，实际「%v」", vars2["riskLevel"])
	}
}
