package repositories

import (
	"log/slog"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"

	"gorm.io/gorm"
)

var DashboardRepository = newDashboardRepository()

func newDashboardRepository() *dashboardRepository {
	return &dashboardRepository{}
}

type dashboardRepository struct {
}

func (r *dashboardRepository) CountConversations(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) int64 {
	var count int64
	tx := db.Model(&models.Conversation{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Count(&count).Error; err != nil {
		slog.Error("dashboardRepository.CountConversations failed", "err", err)
		return 0
	}
	return count
}

func (r *dashboardRepository) ListConversations(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) []models.Conversation {
	var list []models.Conversation
	tx := db.Model(&models.Conversation{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Find(&list).Error; err != nil {
		slog.Error("dashboardRepository.ListConversations failed", "err", err)
		return []models.Conversation{}
	}
	return list
}

func (r *dashboardRepository) ListEnabledAgentProfiles(db *gorm.DB) []models.AgentProfile {
	var list []models.AgentProfile
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := db.Model(&models.AgentProfile{}).Where("status = ?", enums.StatusOk).Find(&list).Error; err != nil {
		slog.Error("dashboardRepository.ListEnabledAgentProfiles failed", "err", err)
		return []models.AgentProfile{}
	}
	return list
}

func (r *dashboardRepository) ListEnabledAgentTeams(db *gorm.DB) []models.AgentTeam {
	var list []models.AgentTeam
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := db.Model(&models.AgentTeam{}).Where("status = ?", enums.StatusOk).Order("id asc").Find(&list).Error; err != nil {
		slog.Error("dashboardRepository.ListEnabledAgentTeams failed", "err", err)
		return []models.AgentTeam{}
	}
	return list
}

func (r *dashboardRepository) ListActiveTeamSchedules(db *gorm.DB, startAt, endAt time.Time) []models.AgentTeamSchedule {
	var list []models.AgentTeamSchedule
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := db.Model(&models.AgentTeamSchedule{}).
		Where("status = ? AND start_at <= ? AND end_at >= ?", enums.StatusOk, endAt, startAt).
		Find(&list).Error; err != nil {
		slog.Error("dashboardRepository.ListActiveTeamSchedules failed", "err", err)
		return []models.AgentTeamSchedule{}
	}
	return list
}

func (r *dashboardRepository) CountAIAgents(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) int64 {
	var count int64
	tx := db.Model(&models.AIAgent{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Count(&count).Error; err != nil {
		slog.Error("dashboardRepository.CountAIAgents failed", "err", err)
		return 0
	}
	return count
}

func (r *dashboardRepository) ListAIAgents(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) []models.AIAgent {
	var list []models.AIAgent
	tx := db.Model(&models.AIAgent{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Find(&list).Error; err != nil {
		slog.Error("dashboardRepository.ListAIAgents failed", "err", err)
		return []models.AIAgent{}
	}
	return list
}

func (r *dashboardRepository) CountChannels(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) int64 {
	var count int64
	tx := db.Model(&models.Channel{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Count(&count).Error; err != nil {
		slog.Error("dashboardRepository.CountChannels failed", "err", err)
		return 0
	}
	return count
}

func (r *dashboardRepository) CountKnowledgeRetrieveLogs(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) int64 {
	var count int64
	tx := db.Model(&models.KnowledgeRetrieveLog{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Count(&count).Error; err != nil {
		slog.Error("dashboardRepository.CountKnowledgeRetrieveLogs failed", "err", err)
		return 0
	}
	return count
}

func (r *dashboardRepository) CountAgentRuns(db *gorm.DB, query func(tx *gorm.DB) *gorm.DB) int64 {
	var count int64
	tx := db.Model(&models.AgentRun{})
	if query != nil {
		tx = query(tx)
	}
	// [AgentDesk] 检查并记录查询错误，杜绝静默失败
	if err := tx.Count(&count).Error; err != nil {
		slog.Error("dashboardRepository.CountAgentRuns failed", "err", err)
		return 0
	}
	return count
}
