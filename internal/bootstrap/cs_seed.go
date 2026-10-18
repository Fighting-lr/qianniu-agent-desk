package bootstrap

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"time"

	"agent-desk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

//go:embed seed_data/cs_session_context.json
var embeddedCSSessionData []byte

type seedRiskItem struct {
	Category        string   `json:"category"`
	Level           string   `json:"level"`
	Score           int      `json:"score"`
	Reason          string   `json:"reason"`
	HardWords       []string `json:"hard_words"`
	SuggestedAction string   `json:"suggested_action"`
	Source          string   `json:"source"`
	EvidenceRef     string   `json:"evidence_ref"`
}

type seedSessionItem struct {
	SessionID         string         `json:"session_id"`
	ConversationID    int64          `json:"conversation_id"`
	BuyerNickname     string         `json:"buyer_nickname"`
	SceneMajor        string         `json:"scene_major"`
	SceneMinor        string         `json:"scene_minor"`
	OrderNo           string         `json:"order_no"`
	TicketNo          string         `json:"ticket_no"`
	TicketType        string         `json:"ticket_type"`
	OrderProduct      string         `json:"order_product"`
	OrderAmount       float64        `json:"order_amount"`
	OrderStatus       string         `json:"order_status"`
	TicketSummary     string         `json:"ticket_summary"`
	BatchNo           string         `json:"batch_no"`
	MessageCount      int            `json:"message_count"`
	BuyerMessageCount int            `json:"buyer_message_count"`
	AgentMessageCount int            `json:"agent_message_count"`
	FirstMessageAt    *string        `json:"first_message_at"`
	LastMessageAt     *string        `json:"last_message_at"`
	Risks             []seedRiskItem `json:"risks"`
	OverallRisk       string         `json:"overall_risk"`
	EvidenceMismatch  any            `json:"evidence_mismatch"`
	ImageTypes        any            `json:"image_types"`
	ImagePaths        []string       `json:"image_paths"`
	BuyerVisitCount   int            `json:"buyer_visit_count"`
	BuyerTotalPaid    float64        `json:"buyer_total_paid"`
	DemoPinned        bool           `json:"demo_pinned"`
	DemoNote          string         `json:"demo_note"`
}

// SeedCSSessionsIfEmpty 在没有演示数据时自动注入脱敏的 138 条会话全景与风险事件
func SeedCSSessionsIfEmpty() (int, error) {
	var count int64
	if err := sqls.DB().Model(&models.CSSessionContext{}).Count(&count).Error; err != nil {
		return 0, err
	}
	if count > 0 {
		return int(count), nil
	}
	return SeedCSSessions(false)
}

// SeedCSSessions 执行数据注入，force=true 时清空后重新灌入
func SeedCSSessions(force bool) (int, error) {
	var items []seedSessionItem
	if err := json.Unmarshal(embeddedCSSessionData, &items); err != nil {
		slog.Error("failed to unmarshal embedded CS session data", "err", err)
		return 0, err
	}

	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if force {
			if err := tx.Where("1 = 1").Delete(&models.CSRiskEvent{}).Error; err != nil {
				return err
			}
			if err := tx.Where("1 = 1").Delete(&models.CSSessionContext{}).Error; err != nil {
				return err
			}
		}

		timeFormat := "2006-01-02 15:04:05"
		now := time.Now()

		for _, item := range items {
			var firstAt, lastAt *time.Time
			if item.FirstMessageAt != nil && *item.FirstMessageAt != "" {
				if t, err := time.ParseInLocation(timeFormat, *item.FirstMessageAt, time.Local); err == nil {
					firstAt = &t
				}
			}
			if item.LastMessageAt != nil && *item.LastMessageAt != "" {
				if t, err := time.ParseInLocation(timeFormat, *item.LastMessageAt, time.Local); err == nil {
					lastAt = &t
				}
			}

			imgTypesJSON, _ := json.Marshal(item.ImageTypes)
			evMismatchJSON, _ := json.Marshal(item.EvidenceMismatch)

			orderProd := item.OrderProduct
			if orderProd == "" {
				orderProd = "美妆护肤品（演示预设）"
			}
			orderStat := item.OrderStatus
			if orderStat == "" {
				orderStat = "已签收"
			}
			orderAmt := item.OrderAmount
			if orderAmt == 0 {
				orderAmt = item.BuyerTotalPaid
			}

			// 计算风险类别数
			catSet := make(map[string]struct{})
			for _, r := range item.Risks {
				if r.Category != "" {
					catSet[r.Category] = struct{}{}
				}
			}

			ctxRow := models.CSSessionContext{
				SessionNo:         item.SessionID,
				ConversationID:    item.ConversationID,
				OrderNo:           item.OrderNo,
				TicketNo:          item.TicketNo,
				TicketType:        item.TicketType,
				OrderProduct:      orderProd,
				OrderAmount:       orderAmt,
				OrderStatus:       orderStat,
				TicketSummary:     item.TicketSummary,
				BatchNo:           item.BatchNo,
				BuyerNickname:     item.BuyerNickname,
				SceneMajor:        item.SceneMajor,
				SceneMinor:        item.SceneMinor,
				OverallRisk:       item.OverallRisk,
				RiskCount:         len(catSet),
				ImageCount:        len(item.ImagePaths),
				ImageTypes:        string(imgTypesJSON),
				EvidenceMismatch:  string(evMismatchJSON),
				BuyerVisitCount:   item.BuyerVisitCount,
				BuyerTotalPaid:    item.BuyerTotalPaid,
				MessageCount:      item.MessageCount,
				BuyerMessageCount: item.BuyerMessageCount,
				AgentMessageCount: item.AgentMessageCount,
				FirstMessageAt:    firstAt,
				LastMessageAt:     lastAt,
				DemoPinned:        item.DemoPinned,
				DemoNote:          item.DemoNote,
			}
			ctxRow.CreatedAt = now
			ctxRow.UpdatedAt = now

			if err := tx.Create(&ctxRow).Error; err != nil {
				return err
			}

			for _, r := range item.Risks {
				hardWordsJSON, _ := json.Marshal(r.HardWords)
				ev := models.CSRiskEvent{
					ConversationID:  item.ConversationID,
					SessionNo:       item.SessionID,
					Category:        r.Category,
					Level:           r.Level,
					Score:           r.Score,
					Source:          r.Source,
					Reason:          r.Reason,
					HardWords:       string(hardWordsJSON),
					EvidenceRef:     r.EvidenceRef,
					SuggestedAction: r.SuggestedAction,
					Handled:         false,
				}
				ev.CreatedAt = now
				ev.UpdatedAt = now
				if err := tx.Create(&ev).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("SeedCSSessions failed", "err", err)
		return 0, err
	}
	slog.Info("SeedCSSessions succeeded", "seededCount", len(items))
	return len(items), nil
}
