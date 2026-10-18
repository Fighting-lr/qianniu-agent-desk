package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"agent-desk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var UserService = newUserService()

func newUserService() *userService {
	return &userService{}
}

type userService struct {
}

// [AgentDesk-BEGIN] 注册相关的校验规则
var (
	// 用户名：3-32 位字母 / 数字 / 下划线
	registerUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	// 密码最小长度（bcrypt 输入上限 72 字节，超出会被静默截断）
	registerMinPasswordLength = 6
)

// [AgentDesk-END]

func (s *userService) Get(id int64) *models.User {
	return repositories.UserRepository.Get(sqls.DB(), id)
}

func (s *userService) Take(where ...interface{}) *models.User {
	return repositories.UserRepository.Take(sqls.DB(), where...)
}

func (s *userService) Find(cnd *sqls.Cnd) []models.User {
	return repositories.UserRepository.Find(sqls.DB(), cnd)
}

func (s *userService) FindOne(cnd *sqls.Cnd) *models.User {
	return repositories.UserRepository.FindOne(sqls.DB(), cnd)
}

func (s *userService) FindPageByParams(params *params.QueryParams) (list []models.User, paging *sqls.Paging) {
	return repositories.UserRepository.FindPageByParams(sqls.DB(), params)
}

func (s *userService) FindPageByCnd(cnd *sqls.Cnd) (list []models.User, paging *sqls.Paging) {
	return repositories.UserRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *userService) Count(cnd *sqls.Cnd) int64 {
	return repositories.UserRepository.Count(sqls.DB(), cnd)
}

func (s *userService) FindByIds(ids []int64) []models.User {
	return repositories.UserRepository.FindByIds(sqls.DB(), ids)
}

func (s *userService) Create(t *models.User) error {
	return repositories.UserRepository.Create(sqls.DB(), t)
}

func (s *userService) Update(t *models.User) error {
	return repositories.UserRepository.Update(sqls.DB(), t)
}

func (s *userService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.UserRepository.Updates(sqls.DB(), id, columns)
}

func (s *userService) UpdateColumn(id int64, name string, value interface{}) error {
	return repositories.UserRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *userService) GetByUsername(username string) *models.User {
	return repositories.UserRepository.GetByUsername(sqls.DB(), username)
}

func (s *userService) GetByMobile(mobile string) *models.User {
	return repositories.UserRepository.GetByMobile(sqls.DB(), mobile)
}

func (s *userService) GetByEmail(email string) *models.User {
	return repositories.UserRepository.GetByEmail(sqls.DB(), email)
}

func (s *userService) CreateUser(req request.CreateUserRequest, operator *dto.AuthPrincipal) (*models.User, string, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, "", errorsx.InvalidParamI18n("error.e0257")
	}
	if s.GetByUsername(username) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0259")
	}

	mobile := utils.NormalizeNullableString(req.Mobile)
	email := utils.NormalizeNullableString(req.Email)
	if mobile != nil && s.GetByMobile(*mobile) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0206")
	}
	if email != nil && s.GetByEmail(*email) != nil {
		return nil, "", errorsx.InvalidParamI18n("error.e0338")
	}

	plain, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return nil, "", err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	user := &models.User{
		Username:     username,
		Nickname:     strings.TrimSpace(req.Nickname),
		Password:     string(passwordHash),
		Avatar:       strings.TrimSpace(req.Avatar),
		Mobile:       mobile,
		Email:        email,
		Status:       enums.StatusOk,
		Remark:       strings.TrimSpace(req.Remark),
		PasswordSalt: "",
		AuditFields:  utils.BuildAuditFields(operator),
	}
	if user.Nickname == "" {
		user.Nickname = username
	}

	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.UserRepository.Create(ctx.Tx, user); err != nil {
			return err
		}
		return s.replaceUserRolesDB(ctx.Tx, user.ID, req.RoleIDs, operator)
	})
	if err != nil {
		return nil, "", err
	}
	return user, plain, nil
}

func (s *userService) UpdateUser(req request.UpdateUserRequest, operator *dto.AuthPrincipal) error {
	user := s.Get(req.ID)
	if user == nil || user.DeletedAt != nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}

	mobile := utils.NormalizeNullableString(req.Mobile)
	email := utils.NormalizeNullableString(req.Email)
	if mobile != nil {
		if existed := s.GetByMobile(*mobile); existed != nil && existed.ID != req.ID {
			return errorsx.InvalidParamI18n("error.e0206")
		}
	}
	if email != nil {
		if existed := s.GetByEmail(*email); existed != nil && existed.ID != req.ID {
			return errorsx.InvalidParamI18n("error.e0338")
		}
	}

	return s.Updates(req.ID, map[string]any{
		"nickname":         strings.TrimSpace(req.Nickname),
		"avatar":           strings.TrimSpace(req.Avatar),
		"mobile":           mobile,
		"email":            email,
		"remark":           strings.TrimSpace(req.Remark),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *userService) DeleteUser(id int64, operator *dto.AuthPrincipal) error {
	user := s.Get(id)
	if user == nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}

	if err := s.Updates(id, map[string]any{
		"status":           enums.StatusDisabled,
		"deleted_at":       time.Now(),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(id, operator.UserID, operator.Username)
}

func (s *userService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	user := s.Get(id)
	if user == nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	if !slices.Contains(enums.StatusValues, enums.Status(status)) {
		return errorsx.InvalidParamI18n("error.e0254")
	}
	if err := s.Updates(id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	}); err != nil {
		return err
	}
	if status == int(enums.StatusDisabled) || status == int(enums.StatusDeleted) {
		return LoginSessionService.RevokeByUser(id, operator.UserID, operator.Username)
	}
	return nil
}

func (s *userService) ResetPassword(userID int64, operator *dto.AuthPrincipal) (string, error) {
	password, err := utils.GenerateRandomPassword(12)
	if err != nil {
		return "", err
	}
	if err = s.changePassword(userID, password, operator); err != nil {
		return "", err
	}
	return password, nil
}

// [AgentDesk-BEGIN] 开放注册
//
// 背景：上游预留了 i18n 文案（auth.noAccount / auth.signUp）和登录页一处
// 被注释掉的入口链接，但注册能力一直没有落地。
//
// 设计取舍：
//   · 默认角色 cs_user（客服，最小权限）—— 注册者拿不到任何管理能力
//   · 注册成功后自动挂到第一个启用中的客服组并创建客服档案，
//     这样"注册 → 登录 → 进工作台 → 认领会话"一步到位；
//     没有可用客服组时跳过档案创建，不阻塞注册
//   · 档案的 auto_assign_enabled 强制为 false：gorm 的 default:true 标签
//     会把 bool 零值当"未设置"而回落到数据库默认值（实测踩过），
//     注册用户不参与自动分配，避免打乱既有会话状态
func (s *userService) Register(req request.RegisterRequest) (*models.User, error) {
	username := strings.TrimSpace(req.Username)
	nickname := strings.TrimSpace(req.Nickname)
	password := req.Password

	// 用户名规则：3-32 位字母 / 数字 / 下划线
	if !registerUsernamePattern.MatchString(username) {
		return nil, errorsx.InvalidParamI18n("error.e0348")
	}
	if s.GetByUsername(username) != nil {
		return nil, errorsx.InvalidParamI18n("error.e0259")
	}
	// bcrypt 的输入上限是 72 字节，超出会静默截断，这里一并拦下
	if len(password) < registerMinPasswordLength || len(password) > 72 {
		return nil, errorsx.InvalidParamI18n("error.e0349")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if nickname == "" {
		nickname = username
	}

	user := &models.User{
		Username:     username,
		Nickname:     nickname,
		Password:     string(passwordHash),
		Status:       enums.StatusOk,
		PasswordSalt: "",
		// 注册没有"操作人"；创建后用自增 ID 回填审计字段
		AuditFields: utils.BuildAuditFields(nil),
	}

	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.UserRepository.Create(ctx.Tx, user); err != nil {
			return err
		}
		// 审计字段里的 create_user_id 依赖自增 ID，创建后回填一次
		operator := &dto.AuthPrincipal{UserID: user.ID, Username: username}
		if err := repositories.UserRepository.Updates(ctx.Tx, user.ID, map[string]any{
			"create_user_id":   user.ID,
			"create_user_name": username,
			"update_user_id":   user.ID,
			"update_user_name": username,
		}); err != nil {
			return err
		}

		// 默认角色：客服（按 code 查；角色缺失不阻塞注册）
		if role := RoleService.Take("code = ? AND status = ?", constants.RoleCodeCsUser, int(enums.StatusOk)); role != nil {
			if err := s.replaceUserRolesDB(ctx.Tx, user.ID, []int64{role.ID}, operator); err != nil {
				return err
			}
		}
		return s.ensureAgentProfileForUser(ctx.Tx, user, operator)
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ensureAgentProfileForUser 给新注册用户创建客服档案（若已有则跳过）。
// 没有可用客服组时静默跳过 —— 档案缺失只会影响"认领会话"，不影响登录与后台。
func (s *userService) ensureAgentProfileForUser(tx *gorm.DB, user *models.User, operator *dto.AuthPrincipal) error {
	if AgentProfileService.Take("user_id = ?", user.ID) != nil {
		return nil
	}
	team := AgentTeamService.Take("status = ?", int(enums.StatusOk))
	if team == nil {
		return nil
	}

	profile := &models.AgentProfile{
		UserID:                user.ID,
		TeamID:                team.ID,
		AgentCode:             fmt.Sprintf("U%06d", user.ID),
		DisplayName:           user.Nickname,
		ServiceStatus:         enums.ServiceStatusIdle,
		MaxConcurrentCount:    5,
		AutoAssignEnabled:     false,
		ReceiveOfflineMessage: false,
		Status:                enums.StatusOk,
		AuditFields:           utils.BuildAuditFields(operator),
	}
	if err := tx.Create(profile).Error; err != nil {
		return err
	}
	// gorm 的 default:true 标签会忽略 bool 零值，必须用显式 Update 才能写成 false
	return tx.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).
		UpdateColumn("auto_assign_enabled", false).Error
}

// [AgentDesk-END]

func (s *userService) ChangeOwnPassword(password string, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.UserID <= 0 {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return s.changePassword(operator.UserID, password, operator)
}

func (s *userService) AssignRoles(userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	user := s.Get(userID)
	if user == nil || user.DeletedAt != nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	if err := s.replaceUserRoles(userID, roleIDs, operator); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(userID, operator.UserID, operator.Username)
}

func (s *userService) replaceUserRoles(userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return s.replaceUserRolesDB(ctx.Tx, userID, roleIDs, operator)
	})
}

func (s *userService) replaceUserRolesDB(db *gorm.DB, userID int64, roleIDs []int64, operator *dto.AuthPrincipal) error {
	if err := db.Where("user_id = ?", userID).Delete(&models.UserRole{}).Error; err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		role := RoleService.Get(roleID)
		if role == nil {
			return errorsx.InvalidParamI18n("error.e0305")
		}
		if role.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.e0291")
		}
		relation := &models.UserRole{
			UserID:      userID,
			RoleID:      roleID,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err := db.Create(relation).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *userService) changePassword(userID int64, password string, operator *dto.AuthPrincipal) error {
	user := s.Get(userID)
	if user == nil || user.DeletedAt != nil {
		return errorsx.InvalidParamI18n("error.e0255")
	}
	if strings.TrimSpace(password) == "" {
		return errorsx.InvalidParamI18n("error.e0220")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now()
	if err = s.Updates(userID, map[string]any{
		"password":         string(passwordHash),
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       now,
	}); err != nil {
		return err
	}
	return LoginSessionService.RevokeByUser(userID, operator.UserID, operator.Username)
}
