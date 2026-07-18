package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"navo-nt-forum/internal/model"
)

// ===== UserService =====

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// UserBrief 用户简要信息
type UserBrief struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Signature   string `json:"signature"`
	Level       int    `json:"level"`
	Points      int    `json:"points"`
	Experience  int    `json:"experience"`
	Role        string `json:"role"`
	Status      int8   `json:"status"`
	Verified    bool   `json:"verified"`
	VerifyType  string `json:"verifyType"`
	VerifyLabel string `json:"verifyLabel"`
}

// ToBrief 转换为简要信息（包含认证）
func (s *UserService) ToBrief(u *model.User) UserBrief {
	b := UserBrief{
		ID:         u.ID,
		Username:   u.Username,
		Avatar:     u.Avatar,
		Signature:  u.Signature,
		Level:      u.Level,
		Points:     u.Points,
		Experience: u.Experience,
		Role:       string(u.Role),
		Status:     int8(u.Status),
	}
	var v model.UserVerification
	if err := s.db.Where("user_id = ? AND revoked_at IS NULL", u.ID).First(&v).Error; err == nil {
		b.Verified = true
		b.VerifyType = string(v.VerifyType)
		b.VerifyLabel = v.Label
	}
	return b
}

// GetByID 通过 ID 查询
func (s *UserService) GetByID(id string) (*model.User, error) {
	var u model.User
	err := s.db.First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

// GetByUsername 通过用户名查询
func (s *UserService) GetByUsername(username string) (*model.User, error) {
	var u model.User
	err := s.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

// List 分页查询用户
func (s *UserService) List(page, size int, keyword string) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	q := s.db.Model(&model.User{})
	if keyword != "" {
		q = q.Where("username LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&users).Error
	return users, total, err
}

// UpdateProfile 更新个人资料
func (s *UserService) UpdateProfile(userID, avatar, signature string) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"avatar":    avatar,
			"signature": signature,
		}).Error
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(userID, oldPw, newPw string) error {
	var u model.User
	if err := s.db.First(&u, "id = ?", userID).Error; err != nil {
		return ErrUserNotFound
	}
	if !checkPw(oldPw, u.PasswordHash) {
		return errors.New("旧密码错误")
	}
	hash, err := hashPw(newPw)
	if err != nil {
		return err
	}
	return s.db.Model(&u).Update("password_hash", hash).Error
}

// GrantVerification 授予认证
func (s *UserService) GrantVerification(userID string, vt model.VerifyType, label, adminID string) error {
	var existing model.UserVerification
	err := s.db.Where("user_id = ? AND verify_type = ?", userID, vt).First(&existing).Error
	if err == nil {
		// 已存在，恢复
		return s.db.Model(&existing).Updates(map[string]interface{}{
			"label":      label,
			"granted_by": adminID,
			"granted_at": time.Now(),
			"revoked_at": nil,
		}).Error
	}
	v := model.UserVerification{
		UserID:     userID,
		VerifyType: vt,
		Label:      label,
		GrantedBy:  adminID,
	}
	return s.db.Create(&v).Error
}

// RevokeVerification 撤销认证
func (s *UserService) RevokeVerification(userID string, vt model.VerifyType) error {
	now := time.Now()
	return s.db.Model(&model.UserVerification{}).
		Where("user_id = ? AND verify_type = ? AND revoked_at IS NULL", userID, vt).
		Update("revoked_at", now).Error
}

// ListVerifications 认证列表
func (s *UserService) ListVerifications(page, size int) ([]model.UserVerification, int64, error) {
	var list []model.UserVerification
	var total int64
	q := s.db.Model(&model.UserVerification{}).Where("revoked_at IS NULL")
	q.Count(&total)
	err := q.Preload("User").Order("granted_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// UserTopics 用户主题列表
func (s *UserService) UserTopics(userID string, page, size int) ([]model.Topic, int64, error) {
	var topics []model.Topic
	var total int64
	q := s.db.Model(&model.Topic{}).Where("author_id = ? AND status = 1")
	q.Count(&total)
	err := q.Preload("Category").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&topics).Error
	return topics, total, err
}

// UserReplies 用户回复列表
func (s *UserService) UserReplies(userID string, page, size int) ([]model.Reply, int64, error) {
	var replies []model.Reply
	var total int64
	q := s.db.Model(&model.Reply{}).Where("author_id = ? AND status = 1")
	q.Count(&total)
	err := q.Preload("Topic").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&replies).Error
	return replies, total, err
}

// UserFavorites 用户收藏列表
func (s *UserService) UserFavorites(userID string, page, size int) ([]model.Favorite, int64, error) {
	var favs []model.Favorite
	var total int64
	q := s.db.Model(&model.Favorite{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Preload("Topic").Preload("Topic.Author").Preload("Topic.Category").
		Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&favs).Error
	return favs, total, err
}

// AdjustPoints 管理员调整积分
func (s *UserService) AdjustPoints(userID string, pointsDelta, expDelta int, remark, adminID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.First(&u, "id = ?", userID).Error; err != nil {
			return ErrUserNotFound
		}
		newPoints := u.Points + pointsDelta
		newExp := u.Experience + expDelta
		if newExp < 0 {
			newExp = 0
		}
		if newPoints < 0 {
			newPoints = 0
		}
		newLevel := model.CalcLevel(newExp)
		if err := tx.Model(&u).Updates(map[string]interface{}{
			"points":     newPoints,
			"experience": newExp,
			"level":      newLevel,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.PointLog{
			UserID:   userID,
			Action:   "admin",
			Delta:    pointsDelta,
			ExpDelta: expDelta,
			Remark:   remark,
		}).Error
	})
}

// SetRole 设置角色
func (s *UserService) SetRole(userID string, role model.Role) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).Update("role", role).Error
}

// SetStatus 设置状态
func (s *UserService) SetStatus(userID string, status model.UserStatus) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).Update("status", status).Error
}

// VerificationsOfUser 查询某用户的所有有效认证
func (s *UserService) VerificationsOfUser(userID string) ([]model.UserVerification, error) {
	var list []model.UserVerification
	err := s.db.Where("user_id = ? AND revoked_at IS NULL", userID).Find(&list).Error
	return list, err
}

// SanitizeUsername 清理用户名
func SanitizeUsername(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
