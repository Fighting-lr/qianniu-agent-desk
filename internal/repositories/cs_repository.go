package repositories

import (
	"log/slog"

	"agent-desk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// ============================================================================
// [AgentDesk-BEGIN] 数据共情者 · 仓储层
//
// 遵循 agent-desk 既有 repositories 写法：包级单例 + 方法收 db *gorm.DB。
// 生产调用统一传 sqls.DB()；测试可传内存库。
// ============================================================================

// ===== [AgentDesk-BEGIN] 风险等级排序表达式（跨数据库兼容） =====
//
// 为什么不用 MySQL 的 FIELD()：
//   FIELD() 是 MySQL 专有函数。本项目默认跑 SQLite，执行时报
//   "no such function: FIELD"；而 GORM 的 Find() 不检查 .Error，
//   错误被静默吞掉 —— 表现是接口返回空列表、日志里一行报错都没有。
//   本项目三处查询曾因此全部返回空（风险看板列表空白、风险明细为空）。
//
// CASE WHEN 是标准 SQL，SQLite / MySQL 都能正常执行，且排序语义一致。
const (
	// OrderRiskLevelSQL 风险事件按等级排序（critical 最前）。
	OrderRiskLevelSQL = "CASE level WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 ELSE 5 END"
	// OrderOverallRiskSQL 会话按整体风险排序（critical 最前）。
	OrderOverallRiskSQL = "CASE overall_risk WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 ELSE 5 END"
)

// ===== [AgentDesk-END] =====

var (
	CSSessionContextRepository = newCSSessionContextRepository()
	CSRiskEventRepository      = newCSRiskEventRepository()
)

// ---------------------------------------------------------------- 会话全景 ---

func newCSSessionContextRepository() *csSessionContextRepository {
	return &csSessionContextRepository{}
}

type csSessionContextRepository struct{}

func (r *csSessionContextRepository) Get(db *gorm.DB, id int64) *models.CSSessionContext {
	ret := &models.CSSessionContext{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *csSessionContextRepository) GetBySessionNo(sessionNo string) *models.CSSessionContext {
	ret := &models.CSSessionContext{}
	if err := sqls.DB().First(ret, "session_no = ?", sessionNo).Error; err != nil {
		return nil
	}
	return ret
}

func (r *csSessionContextRepository) GetByConversationID(conversationID int64) *models.CSSessionContext {
	ret := &models.CSSessionContext{}
	if err := sqls.DB().First(ret, "conversation_id = ?", conversationID).Error; err != nil {
		return nil
	}
	return ret
}

// UpsertBySessionNo 按 session_no 幂等写入。存在则整行更新，不存在则插入。
// 导入脚本可重复执行，不会产生重复行。
func (r *csSessionContextRepository) UpsertBySessionNo(item *models.CSSessionContext) error {
	if item == nil || item.SessionNo == "" {
		return nil
	}
	existing := r.GetBySessionNo(item.SessionNo)
	if existing == nil {
		return sqls.DB().Create(item).Error
	}
	item.ID = existing.ID
	return sqls.DB().Model(&models.CSSessionContext{}).
		Where("id = ?", existing.ID).
		Select("*").
		Omit("id", "created_at").
		Updates(item).Error
}

// UpsertByConversationID 按 conversation_id 幂等写入。
func (r *csSessionContextRepository) UpsertByConversationID(item *models.CSSessionContext) error {
	if item == nil || item.ConversationID <= 0 {
		return nil
	}
	existing := r.GetByConversationID(item.ConversationID)
	if existing == nil {
		return sqls.DB().Create(item).Error
	}
	item.ID = existing.ID
	return sqls.DB().Model(&models.CSSessionContext{}).
		Where("id = ?", existing.ID).
		Select("*").
		Omit("id", "created_at").
		Updates(item).Error
}


func (r *csSessionContextRepository) Count() int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSSessionContext{}).Count(&n).Error; err != nil {
		slog.Error("CSSessionContextRepository.Count failed", "err", err)
		return 0
	}
	return n
}

func (r *csSessionContextRepository) CountByRiskLevel(level string) int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSSessionContext{}).Where("overall_risk = ?", level).Count(&n).Error; err != nil {
		slog.Error("CSSessionContextRepository.CountByRiskLevel failed", "level", level, "err", err)
		return 0
	}
	return n
}

// CountEvidenceMismatch 统计存在图文证据不一致的会话数。
// 这是「数据共情者」多模态差异化的量化指标。
func (r *csSessionContextRepository) CountEvidenceMismatch() int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSSessionContext{}).
		Where("evidence_mismatch IS NOT NULL AND evidence_mismatch != '' AND evidence_mismatch != '[]'").
		Count(&n).Error; err != nil {
		slog.Error("CSSessionContextRepository.CountEvidenceMismatch failed", "err", err)
		return 0
	}
	return n
}

// CountGroupBySceneMinor 按子场景分组统计，供看板的场景分布图使用。
func (r *csSessionContextRepository) CountGroupBySceneMinor() map[string]int64 {
	type row struct {
		SceneMinor string
		N          int64
	}
	var rows []row
	if err := sqls.DB().Model(&models.CSSessionContext{}).
		Select("scene_minor, COUNT(*) AS n").
		Where("scene_minor != ''").
		Group("scene_minor").
		Order("n DESC").
		Limit(20).
		Scan(&rows).Error; err != nil {
		slog.Error("CSSessionContextRepository.CountGroupBySceneMinor failed", "err", err)
		return map[string]int64{}
	}
	out := make(map[string]int64, len(rows))
	for _, x := range rows {
		out[x.SceneMinor] = x.N
	}
	return out
}

// ---------------------------------------------------------------- 风险明细 ---

func newCSRiskEventRepository() *csRiskEventRepository {
	return &csRiskEventRepository{}
}

type csRiskEventRepository struct{}

func (r *csRiskEventRepository) Get(db *gorm.DB, id int64) *models.CSRiskEvent {
	ret := &models.CSRiskEvent{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

// ListBySessionNo 取某会话的全部风险，等级倒序 + 分数倒序。
func (r *csRiskEventRepository) ListBySessionNo(sessionNo string) []models.CSRiskEvent {
	var list []models.CSRiskEvent
	if err := sqls.DB().Where("session_no = ?", sessionNo).
		Order(OrderRiskLevelSQL + ", score DESC").
		Find(&list).Error; err != nil {
		slog.Error("CSRiskEventRepository.ListBySessionNo failed", "sessionNo", sessionNo, "err", err)
		return nil
	}
	return list
}

// ListByConversationID 供工作流节点在实时链路里按会话 ID 查。
func (r *csRiskEventRepository) ListByConversationID(conversationID int64) []models.CSRiskEvent {
	var list []models.CSRiskEvent
	if err := sqls.DB().Where("conversation_id = ?", conversationID).
		Order(OrderRiskLevelSQL + ", score DESC").
		Find(&list).Error; err != nil {
		slog.Error("CSRiskEventRepository.ListByConversationID failed", "conversationID", conversationID, "err", err)
		return nil
	}
	return list
}

func (r *csRiskEventRepository) CountByCategory(category string) int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSRiskEvent{}).Where("category = ?", category).Count(&n).Error; err != nil {
		slog.Error("CSRiskEventRepository.CountByCategory failed", "category", category, "err", err)
		return 0
	}
	return n
}

func (r *csRiskEventRepository) CountByLevel(level string) int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSRiskEvent{}).Where("level = ?", level).Count(&n).Error; err != nil {
		slog.Error("CSRiskEventRepository.CountByLevel failed", "level", level, "err", err)
		return 0
	}
	return n
}

// CountUnhandled 统计未处置的风险事件数 —— 看板右上角的"待办"角标。
func (r *csRiskEventRepository) CountUnhandled() int64 {
	var n int64
	if err := sqls.DB().Model(&models.CSRiskEvent{}).Where("handled = ?", false).Count(&n).Error; err != nil {
		slog.Error("CSRiskEventRepository.CountUnhandled failed", "err", err)
		return 0
	}
	return n
}

// MarkHandled 客服处置后回写，形成闭环。
func (r *csRiskEventRepository) MarkHandled(id int64) error {
	return sqls.DB().Model(&models.CSRiskEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{"handled": true, "handled_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
}

// MarkHandledBySessionNo 批量处置某会话下所有未处理风险。
// 返回受影响行数，供前端确认"已处理 N 项"。
func (r *csRiskEventRepository) MarkHandledBySessionNo(sessionNo string) (int64, error) {
	tx := sqls.DB().Model(&models.CSRiskEvent{}).
		Where("session_no = ? AND handled = ?", sessionNo, false).
		Updates(map[string]any{"handled": true, "handled_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if tx.Error != nil {
		slog.Error("CSRiskEventRepository.MarkHandledBySessionNo failed", "sessionNo", sessionNo, "err", tx.Error)
	}
	return tx.RowsAffected, tx.Error
}

// MarkHandledByConversationID 批量处置某站内会话下所有未处理风险。
func (r *csRiskEventRepository) MarkHandledByConversationID(conversationID int64) (int64, error) {
	tx := sqls.DB().Model(&models.CSRiskEvent{}).
		Where("conversation_id = ? AND handled = ?", conversationID, false).
		Updates(map[string]any{"handled": true, "handled_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if tx.Error != nil {
		slog.Error("CSRiskEventRepository.MarkHandledByConversationID failed", "conversationID", conversationID, "err", tx.Error)
	}
	return tx.RowsAffected, tx.Error
}

// DeleteBySessionNo 供导入脚本重灌前清理。
func (r *csRiskEventRepository) DeleteBySessionNo(sessionNo string) error {
	return sqls.DB().Where("session_no = ?", sessionNo).Delete(&models.CSRiskEvent{}).Error
}
