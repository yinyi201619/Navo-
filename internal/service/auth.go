// Package service 业务服务层
package service

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/database"
	"navo-nt-forum/internal/model"
)

// ===== 通用错误 =====

var (
	ErrInvalidCredential = errors.New("用户名或密码错误")
	ErrUserExists        = errors.New("用户名或邮箱已存在")
	ErrUserNotFound      = errors.New("用户不存在")
	ErrUserBanned        = errors.New("账号已被封禁")
	ErrUserMuted         = errors.New("账号已被禁言")
	ErrTopicNotFound     = errors.New("帖子不存在")
	ErrReplyNotFound     = errors.New("回复不存在")
	ErrCategoryNotFound  = errors.New("板块不存在")
	ErrNoPermission      = errors.New("没有操作权限")
	ErrAlreadyLiked      = errors.New("已经点过赞")
	ErrRegisterDisabled  = errors.New("注册已关闭")
	ErrAlreadyCheckedIn  = errors.New("今日已签到")
	ErrConversationNotFound = errors.New("会话不存在")
)

// ===== AuthService =====

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

// Register 注册
func (s *AuthService) Register(username, email, password, ip string) (*model.User, error) {
	if !s.cfg.Site.AllowRegister {
		return nil, ErrRegisterDisabled
	}
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 50 {
		return nil, errors.New("用户名长度需为 3-50")
	}
	if len(password) < 6 {
		return nil, errors.New("密码长度至少 6 位")
	}

	var exists int64
	s.db.Model(&model.User{}).Where("username = ? OR (email != '' AND email = ?)", username, email).Count(&exists)
	if exists > 0 {
		return nil, ErrUserExists
	}

	hash, err := database.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	// 注册赠送初始经验
	s.db.Model(&model.User{}).Where("id = ?", user.ID).Update("experience", 0)
	return &user, nil
}

// Login 登录
func (s *AuthService) Login(username, password, ip string) (*model.User, string, error) {
	var user model.User
	err := s.db.Where("username = ? OR email = ?", username, username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", ErrInvalidCredential
	}
	if err != nil {
		return nil, "", err
	}
	if !database.CheckPassword(password, user.PasswordHash) {
		return nil, "", ErrInvalidCredential
	}
	if user.Status == model.UserStatusBanned {
		return nil, "", ErrUserBanned
	}

	now := time.Now()
	s.db.Model(&user).Updates(map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	token, err := s.generateToken(&user)
	if err != nil {
		return nil, "", err
	}
	return &user, token, nil
}

func (s *AuthService) generateToken(u *model.User) (string, error) {
	claims := jwt.MapClaims{
		"uid":      u.ID,
		"username": u.Username,
		"role":     string(u.Role),
		"iss":      s.cfg.JWT.Issuer,
		"exp":      time.Now().Add(s.cfg.JWT.Expire).Unix(),
		"iat":      time.Now().Unix(),
		"jti":      uuid.NewString(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWT.Secret))
}

// ParseToken 解析 token
func (s *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不支持")
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tok.Claims.(*Claims); ok && tok.Valid {
		return claims, nil
	}
	return nil, errors.New("token 无效")
}

// Claims JWT 载荷
type Claims struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GetByID 通过 ID 查询用户
func (s *AuthService) GetByID(id string) (*model.User, error) {
	var u model.User
	err := s.db.First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

// GetByUsername 通过用户名查询
func (s *AuthService) GetByUsername(username string) (*model.User, error) {
	var u model.User
	err := s.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &u, err
}
