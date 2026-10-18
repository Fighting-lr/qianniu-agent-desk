// Package main —— 豆包（火山方舟）结构化输出 PoC。
//
// 目的：验证方舟能否稳定返回**纯 JSON**（无 markdown 围栏、无前后缀废话），
// 这是风险判定链路能否落地的最高风险假设。
//
// 用法：
//
//	go run ./cmd/arkpoc -apikey <KEY> -model <ENDPOINT_ID>
//	go run ./cmd/arkpoc -apikey <KEY> -model <ENDPOINT_ID> -n 20   # 压稳定性
//
// 若不传 -apikey，则从环境变量 ARK_API_KEY 读取。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// 与项目内 cs_risk_service.go 的场景口径保持一致：scene_minor 恰好 41 类。
// 这里只取一个子集做 PoC，验证模型能否在受控枚举里选择。
var sceneMinorEnum = []string{
	"商品质量问题", "商品与描述不符", "发错货", "漏发少发", "包装破损",
	"物流延迟", "物流丢件", "物流破损", "快递态度差", "虚假发货",
	"退换货申请", "退款延迟", "退款金额争议", "运费争议", "仅退款",
	"优惠券问题", "价格保护", "活动规则争议", "赠品缺失", "虚假宣传",
	"客服态度差", "客服响应慢", "客服答非所问", "承诺未兑现", "推诿不处理",
	"发票问题", "订单信息错误", "系统故障", "账号异常", "支付失败",
	"商品咨询", "物流咨询", "售后咨询", "活动咨询", "发票咨询",
	"会员权益", "积分问题", "投诉举报", "威胁曝光", "要求赔偿", "其他",
}

// 期望的输出结构 —— 这是风险判定链路真正需要的东西。
type riskAssessment struct {
	SceneMinor     string   `json:"scene_minor"`
	RiskLevel      string   `json:"risk_level"`
	RiskSignals    []string `json:"risk_signals"`
	SuggestedReply string   `json:"suggested_reply"`
	Confidence     float64  `json:"confidence"`
	Reason         string   `json:"reason"`
}

const systemPromptTemplate = `你是电商客服风险识别助手。请分析客户消息，输出严格 JSON。

可用场景分类（scene_minor 必须从中选一个，不得自创）：
%s

risk_level 取值：low / medium / high / critical

只输出 JSON 对象，不要 markdown 围栏，不要任何解释文字。
JSON 结构：
{"scene_minor":"字符串","risk_level":"字符串","risk_signals":["字符串"],"suggested_reply":"字符串","confidence":0.0-1.0,"reason":"字符串"}`

// 覆盖典型与边界场景的测试用例
var cases = []struct {
	name string
	msg  string
}{
	{"低风险咨询", "请问这个面膜敏感肌能用吗？"},
	{"中风险物流", "我的快递已经五天了还没到，能帮我查一下吗"},
	{"高风险质量", "你们发的货跟图片完全不一样，这是欺骗消费者"},
	{"极高风险威胁", "再不退款我就去12315投诉，还要发微博曝光你们"},
	{"边界-情绪激烈", "气死我了！！！这什么破东西！！！"},
	{"边界-极短", "退款"},
	{"边界-中英混杂", "这个 order 的 refund 什么时候到账"},
	{"边界-多诉求", "东西质量有问题，而且客服一直不理我，我要退货还要赔偿"},
}

func main() {
	apiKey := flag.String("apikey", "", "ARK API Key（缺省读环境变量 ARK_API_KEY）")
	model := flag.String("model", "", "接入点 ID，形如 ep-2026xxxx-xxxxx")
	baseURL := flag.String("baseurl", "https://ark.cn-beijing.volces.com/api/v3", "方舟 Base URL")
	timeout := flag.Int("timeout", 60, "单次请求超时（秒）")
	jsonMode := flag.Bool("jsonmode", true, "是否启用 response_format=json_object")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("ARK_API_KEY")
	}
	if *apiKey == "" {
		fmt.Println("❌ 缺少 API Key。用法：-apikey <KEY>  或  设置环境变量 ARK_API_KEY")
		os.Exit(2)
	}
	if *model == "" {
		fmt.Println("❌ 缺少接入点 ID（-model ep-xxxx）。方舟的 model 字段填的是接入点 ID，不是模型名。")
		os.Exit(2)
	}

	client := openai.NewClient(
		option.WithAPIKey(*apiKey),
		option.WithBaseURL(*baseURL),
		option.WithRequestTimeout(time.Duration(*timeout)*time.Second),
	)

	systemPrompt := fmt.Sprintf(systemPromptTemplate, strings.Join(sceneMinorEnum, "、"))

	fmt.Println("========================================")
	fmt.Println("豆包（火山方舟）结构化输出 PoC")
	fmt.Println("========================================")
	fmt.Printf("  BaseURL   : %s\n", *baseURL)
	fmt.Printf("  Model     : %s\n", *model)
	fmt.Printf("  JSON Mode : %v\n", *jsonMode)
	fmt.Printf("  测试用例  : %d 条\n", len(cases))
	fmt.Println()

	pass, total := 0, 0
	var failReasons []string

	for i, c := range cases {
		total++
		fmt.Printf("[%d/%d] %s\n", i+1, len(cases), c.name)
		fmt.Printf("  输入: %s\n", c.msg)

		start := time.Now()
		params := openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				{OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String(systemPrompt),
					},
				}},
				{OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(c.msg),
					},
				}},
			},
			Model: shared.ChatModel(*model),
		}
		if *jsonMode {
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
		resp, err := client.Chat.Completions.New(ctx, params)
		cancel()

		if err != nil {
			fmt.Printf("  ❌ 调用失败 (%v): %v\n\n", time.Since(start).Round(time.Millisecond), err)
			failReasons = append(failReasons, c.name+": 调用失败 "+err.Error())
			continue
		}
		if len(resp.Choices) == 0 {
			fmt.Printf("  ❌ 无 choices\n\n")
			failReasons = append(failReasons, c.name+": 无 choices")
			continue
		}

		raw := strings.TrimSpace(resp.Choices[0].Message.Content)
		elapsed := time.Since(start).Round(time.Millisecond)

		// ── 关键校验：能否直接 json.Unmarshal（不经任何清洗） ──
		var parsed riskAssessment
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			fmt.Printf("  ❌ JSON 解析失败 (%v)\n", elapsed)
			fmt.Printf("     原始内容: %q\n\n", truncate(raw, 300))
			failReasons = append(failReasons, c.name+": JSON 解析失败")
			continue
		}

		// ── 校验枚举合规性 ──
		if !contains(sceneMinorEnum, parsed.SceneMinor) {
			fmt.Printf("  ⚠️  scene_minor 越界: %q (%v)\n", parsed.SceneMinor, elapsed)
			failReasons = append(failReasons, c.name+": scene_minor 越界 "+parsed.SceneMinor)
			continue
		}
		if !inSet(parsed.RiskLevel, "low", "medium", "high", "critical") {
			fmt.Printf("  ⚠️  risk_level 越界: %q (%v)\n", parsed.RiskLevel, elapsed)
			failReasons = append(failReasons, c.name+": risk_level 越界 "+parsed.RiskLevel)
			continue
		}

		pass++
		fmt.Printf("  ✅ %v | 场景=%s 等级=%s 置信=%.2f\n",
			elapsed, parsed.SceneMinor, parsed.RiskLevel, parsed.Confidence)
		fmt.Printf("     信号: %v\n", parsed.RiskSignals)
		fmt.Printf("     理由: %s\n\n", truncate(parsed.Reason, 100))
	}

	fmt.Println("========================================")
	fmt.Printf("结果: %d/%d 通过（%.0f%%）\n", pass, total, float64(pass)/float64(total)*100)
	fmt.Println("========================================")
	if len(failReasons) > 0 {
		fmt.Println("失败明细:")
		for _, r := range failReasons {
			fmt.Println("  - " + r)
		}
		os.Exit(1)
	}
	fmt.Println("🎉 纯 JSON 输出稳定，结构化链路可落地。")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func inSet(v string, set ...string) bool {
	for _, x := range set {
		if x == v {
			return true
		}
	}
	return false
}
