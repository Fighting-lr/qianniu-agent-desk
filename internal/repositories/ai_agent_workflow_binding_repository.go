package repositories

import (
	"log/slog"

	"agent-desk/internal/models"

	"gorm.io/gorm"
)

var AIAgentWorkflowBindingRepository = newAIAgentWorkflowBindingRepository()

func newAIAgentWorkflowBindingRepository() *aiAgentWorkflowBindingRepository {
	return &aiAgentWorkflowBindingRepository{}
}

type aiAgentWorkflowBindingRepository struct{}

func (r *aiAgentWorkflowBindingRepository) FindByAgentID(db *gorm.DB, agentID int64) []models.AIAgentWorkflowBinding {
	ret := make([]models.AIAgentWorkflowBinding, 0)
	if agentID > 0 {
		// [AgentDesk] 检查并记录查询错误，杜绝静默失败
		if err := db.Where("ai_agent_id = ?", agentID).Order("priority ASC, id ASC").Find(&ret).Error; err != nil {
			slog.Error("aiAgentWorkflowBindingRepository.FindByAgentID failed", "agentID", agentID, "err", err)
			return []models.AIAgentWorkflowBinding{}
		}
	}
	return ret
}

func (r *aiAgentWorkflowBindingRepository) FindEnabledByAgentID(db *gorm.DB, agentID int64) []models.AIAgentWorkflowBinding {
	ret := make([]models.AIAgentWorkflowBinding, 0)
	if agentID > 0 {
		// [AgentDesk] 检查并记录查询错误，杜绝静默失败
		if err := db.Where("ai_agent_id = ? AND enabled = ?", agentID, true).Order("priority ASC, id ASC").Find(&ret).Error; err != nil {
			slog.Error("aiAgentWorkflowBindingRepository.FindEnabledByAgentID failed", "agentID", agentID, "err", err)
			return []models.AIAgentWorkflowBinding{}
		}
	}
	return ret
}

func (r *aiAgentWorkflowBindingRepository) FindByWorkflowID(db *gorm.DB, workflowID int64) []models.AIAgentWorkflowBinding {
	ret := make([]models.AIAgentWorkflowBinding, 0)
	if workflowID > 0 {
		// [AgentDesk] 检查并记录查询错误，杜绝静默失败
		if err := db.Where("workflow_id = ?", workflowID).Order("priority ASC, id ASC").Find(&ret).Error; err != nil {
			slog.Error("aiAgentWorkflowBindingRepository.FindByWorkflowID failed", "workflowID", workflowID, "err", err)
			return []models.AIAgentWorkflowBinding{}
		}
	}
	return ret
}

func (r *aiAgentWorkflowBindingRepository) CountByWorkflowID(db *gorm.DB, workflowID int64) int64 {
	var count int64
	if workflowID > 0 {
		// [AgentDesk] 检查并记录查询错误，杜绝静默失败
		if err := db.Model(&models.AIAgentWorkflowBinding{}).Where("workflow_id = ?", workflowID).Count(&count).Error; err != nil {
			slog.Error("aiAgentWorkflowBindingRepository.CountByWorkflowID failed", "workflowID", workflowID, "err", err)
			return 0
		}
	}
	return count
}

func (r *aiAgentWorkflowBindingRepository) ReplaceByAgentID(db *gorm.DB, agentID int64, items []models.AIAgentWorkflowBinding) error {
	if err := db.Where("ai_agent_id = ?", agentID).Delete(&models.AIAgentWorkflowBinding{}).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return db.Create(&items).Error
}
