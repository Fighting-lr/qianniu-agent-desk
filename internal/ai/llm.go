package ai

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mlogclub/simple/common/strs"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
)

type ChatCompletionResult struct {
	Content          string
	ModelName        string
	PromptTokens     int
	CompletionTokens int
}

type llm struct{}

var LLM = &llm{}

func (s *llm) Chat(ctx context.Context, systemPrompt string, userPrompt string) (*ChatCompletionResult, error) {
	config, err := GetEnabledAIConfig(enums.AIModelTypeLLM)
	if err != nil {
		return nil, err
	}
	return s.ChatWithConfig(ctx, *config, systemPrompt, userPrompt)
}

func (s *llm) ChatWithConfig(ctx context.Context, config models.AIConfig, systemPrompt string, userPrompt string) (*ChatCompletionResult, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if strs.IsNotBlank(systemPrompt) {
		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(systemPrompt),
				},
			},
		})
	}
	messages = append(messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(userPrompt),
			},
		},
	})

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    shared.ChatModel(config.ModelName),
	}
	if config.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(config.MaxOutputTokens))
	}
	applyProviderSpecificChatParams(&params, config)

	client := newOpenAIClient(config)
	chatResp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to call llm api (model=%s provider=%s system_chars=%d user_chars=%d max_output_tokens=%d): %w",
			config.ModelName, config.Provider, utf8.RuneCountInString(systemPrompt), utf8.RuneCountInString(userPrompt), config.MaxOutputTokens, err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no llm choices in response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return &ChatCompletionResult{
		Content:          content,
		ModelName:        config.ModelName,
		PromptTokens:     int(chatResp.Usage.PromptTokens),
		CompletionTokens: int(chatResp.Usage.CompletionTokens),
	}, nil
}

func applyProviderSpecificChatParams(params *openai.ChatCompletionNewParams, config models.AIConfig) {
	if params == nil {
		return
	}
	if isThinkingModelRequiringDisable(config) {
		// [AgentDesk-FIX 2026-09-27] 见 thinkingDisableExtraFields 的说明
		params.SetExtraFields(thinkingDisableExtraFields(config))
	}
}

func isDashScopeQwenThinkingModel(config models.AIConfig) bool {
	baseURL := strings.ToLower(strings.TrimSpace(config.BaseURL))
	modelName := strings.ToLower(strings.TrimSpace(config.ModelName))
	return strings.Contains(baseURL, "dashscope.aliyuncs.com") && strings.HasPrefix(modelName, "qwen3")
}

// [AgentDesk-BEGIN] 数据共情者 · 豆包（火山方舟）思考模式关闭
// 与 eino_agent_loop.go 的同名逻辑保持一致；两处调用点走不同 SDK，
// 无法合并，但判定条件必须完全相同。
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

func isThinkingModelRequiringDisable(config models.AIConfig) bool {
	return isDashScopeQwenThinkingModel(config) || isVolcengineArkThinkingModel(config)
}

// thinkingDisableExtraFields 返回厂商专属的「关闭思考」参数。
//
// [AgentDesk-FIX 2026-09-27] 与 internal/ai/application/runtime/eino_agent_loop.go
// 的同名函数必须保持完全一致（两处走不同 SDK，无法合并）。
// 实测 doubao-seed-2-1-turbo-260628：
//
//	{"enable_thinking": false}          → 仍带 reasoning_content，未生效
//	{"thinking": {"type": "disabled"}}  → 无 reasoning_content，输出 token 16 → 1
func thinkingDisableExtraFields(config models.AIConfig) map[string]any {
	if isVolcengineArkThinkingModel(config) {
		return map[string]any{"thinking": map[string]any{"type": "disabled"}}
	}
	return map[string]any{"enable_thinking": false}
}

// [AgentDesk-END]
