package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	ai "agent-desk/internal/ai"
	"agent-desk/internal/models"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
)

// einoAgentLoop is the production model/tool loop. AgentDesk still owns tool
// authorization, business execution, interrupts, idempotency, and auditing.
func einoAgentLoop(
	ctx context.Context,
	config models.AIConfig,
	systemPrompt string,
	userPrompt string,
	definitions []ai.ToolDefinition,
	maxSteps int,
	execute ai.ToolCallExecutor,
) (*ai.ToolLoopResult, error) {
	model, err := newEinoChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	if maxSteps <= 0 {
		maxSteps = 6
	}
	tools := make([]einotool.BaseTool, 0, len(definitions))
	for _, definition := range definitions {
		tool, buildErr := newEinoFunctionTool(definition, execute)
		if buildErr != nil {
			return nil, buildErr
		}
		tools = append(tools, tool)
	}
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: tools},
		MaxStep:          maxSteps,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino agent loop: %w", err)
	}
	messages := make([]*schema.Message, 0, 2)
	if value := strings.TrimSpace(systemPrompt); value != "" {
		messages = append(messages, schema.SystemMessage(value))
	}
	messages = append(messages, schema.UserMessage(strings.TrimSpace(userPrompt)))
	result, err := agent.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("Eino agent loop returned no result")
	}
	ret := &ai.ToolLoopResult{ChatCompletionResult: ai.ChatCompletionResult{
		Content:   strings.TrimSpace(result.Content),
		ModelName: config.ModelName,
	}}
	if result.ResponseMeta != nil && result.ResponseMeta.Usage != nil {
		ret.PromptTokens = result.ResponseMeta.Usage.PromptTokens
		ret.CompletionTokens = result.ResponseMeta.Usage.CompletionTokens
	}
	return ret, nil
}

func newEinoChatModel(ctx context.Context, config models.AIConfig) (einomodel.ToolCallingChatModel, error) {
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.ModelName) == "" {
		return nil, fmt.Errorf("ai config base URL, API key, and model name are required")
	}
	modelConfig := &einoopenai.ChatModelConfig{
		APIKey:  strings.TrimSpace(config.APIKey),
		BaseURL: strings.TrimSpace(config.BaseURL),
		Model:   strings.TrimSpace(config.ModelName),
	}
	if config.TimeoutMS > 0 {
		modelConfig.Timeout = time.Duration(config.TimeoutMS) * time.Millisecond
	}
	if config.MaxOutputTokens > 0 {
		maxTokens := config.MaxOutputTokens
		modelConfig.MaxCompletionTokens = &maxTokens
	}
	if isThinkingModelRequiringDisable(config) {
		// [AgentDesk-FIX 2026-09-27] 见 thinkingDisableExtraFields 的说明：
		// 火山方舟 doubao-seed-2.x 需要新写法，老的 enable_thinking 已实测失效。
		modelConfig.ExtraFields = thinkingDisableExtraFields(config)
	}
	model, err := einoopenai.NewChatModel(ctx, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("create Eino OpenAI-compatible model: %w", err)
	}
	return model, nil
}

func isDashScopeQwenThinkingModel(config models.AIConfig) bool {
	baseURL := strings.ToLower(strings.TrimSpace(config.BaseURL))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	return strings.Contains(baseURL, "dashscope.aliyuncs.com") && strings.HasPrefix(modelName, "qwen3")
}

// [AgentDesk-BEGIN] 数据共情者 · 豆包（火山方舟）思考模式关闭
//
// 为什么必须关？
//   doubao-seed-* 系列默认开启深度思考，返回体里会带 reasoning_content，
//   而 agent-desk 走的是 OpenAI 兼容的 content 字段。不关的话表现为：
//     · 首字延迟从 ~1s 涨到 5-15s（客服场景无法接受）
//     · 部分模型 content 为空，回复直接丢失
//     · token 成本翻 3-5 倍
//
// 判定条件刻意收窄到 volces.com 域名，避免误伤其他 OpenAI 兼容服务。
func isVolcengineArkThinkingModel(config models.AIConfig) bool {
	baseURL := strings.ToLower(strings.TrimSpace(config.BaseURL))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	if !strings.Contains(baseURL, "volces.com") && !strings.Contains(baseURL, "volcengineapi.com") {
		return false
	}
	return strings.HasPrefix(modelName, "doubao-seed") ||
		strings.HasPrefix(modelName, "doubao-1.5-thinking") ||
		strings.Contains(modelName, "thinking")
}

// isThinkingModelRequiringDisable 汇总所有"需要显式关闭思考"的模型。
// 新增厂商时只改这一个函数，调用点不用动。
func isThinkingModelRequiringDisable(config models.AIConfig) bool {
	return isDashScopeQwenThinkingModel(config) || isVolcengineArkThinkingModel(config)
}

// thinkingDisableExtraFields 返回厂商专属的「关闭思考」参数。
//
// [AgentDesk-FIX 2026-09-27] 为什么不能再用统一的 enable_thinking=false：
// 对 doubao-seed-2-1-turbo-260628（火山方舟）实测对比 ——
//
//	{"enable_thinking": false}          → 返回体仍带 reasoning_content，说明未生效
//	{"thinking": {"type": "disabled"}}  → 无 reasoning_content，输出 token 从 16 降到 1
//
// 而 DashScope 的 qwen3 系列仍在用 enable_thinking，故按厂商分流。
// 与 internal/ai/llm.go 的同名函数必须保持完全一致（两处走不同 SDK，无法合并）。
func thinkingDisableExtraFields(config models.AIConfig) map[string]any {
	if isVolcengineArkThinkingModel(config) {
		return map[string]any{"thinking": map[string]any{"type": "disabled"}}
	}
	return map[string]any{"enable_thinking": false}
}

// [AgentDesk-END]

type einoFunctionTool struct {
	info    *schema.ToolInfo
	execute ai.ToolCallExecutor
}

var _ einotool.InvokableTool = (*einoFunctionTool)(nil)

func newEinoFunctionTool(definition ai.ToolDefinition, execute ai.ToolCallExecutor) (*einoFunctionTool, error) {
	if strings.TrimSpace(definition.Name) == "" || execute == nil {
		return nil, fmt.Errorf("Eino tool name and executor are required")
	}
	info := &schema.ToolInfo{
		Name: strings.TrimSpace(definition.Name),
		Desc: strings.TrimSpace(definition.Description),
	}
	if len(definition.Parameters) > 0 {
		data, err := json.Marshal(definition.Parameters)
		if err != nil {
			return nil, fmt.Errorf("encode Eino tool schema: %w", err)
		}
		var params einojsonschema.Schema
		if err := json.Unmarshal(data, &params); err != nil {
			return nil, fmt.Errorf("decode Eino tool schema: %w", err)
		}
		info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&params)
	}
	return &einoFunctionTool{info: info, execute: execute}, nil
}

func (t *einoFunctionTool) Info(context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *einoFunctionTool) InvokableRun(ctx context.Context, arguments string, _ ...einotool.Option) (string, error) {
	result, err := t.execute(ctx, ai.ToolCall{Name: t.info.Name, Arguments: arguments})
	if err == nil {
		return result, nil
	}
	var interrupt *agentLoopInterruptError
	if errors.As(err, &interrupt) {
		return "", err
	}
	observation, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		return "", err
	}
	return string(observation), nil
}
