package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/model"
)

// QQOAuthService QQ 快捷登录服务
type QQOAuthService struct {
	db  *gorm.DB
	cfg *config.Config

	stateStore map[string]string
	stateMu    sync.Mutex
}

func NewQQOAuthService(db *gorm.DB, cfg *config.Config) *QQOAuthService {
	return &QQOAuthService{
		db:         db,
		cfg:        cfg,
		stateStore: make(map[string]string),
	}
}

// Enabled 是否启用
func (s *QQOAuthService) Enabled() bool {
	return s.cfg.QQOAuth.Enabled && s.cfg.QQOAuth.AppID != "" && s.cfg.QQOAuth.AppKey != ""
}

// AuthorizeURL 生成 QQ 授权 URL
func (s *QQOAuthService) AuthorizeURL(redirectURI string) (string, string, error) {
	state := randomState(32)
	s.stateMu.Lock()
	s.stateStore[state] = redirectURI
	s.stateMu.Unlock()
	go s.expireState(state, 10*time.Minute)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", s.cfg.QQOAuth.AppID)
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	params.Set("scope", s.cfg.QQOAuth.Scope)
	authURL := "https://graph.qq.com/oauth2.0/authorize?" + params.Encode()
	return authURL, state, nil
}

func (s *QQOAuthService) expireState(state string, d time.Duration) {
	time.Sleep(d)
	s.stateMu.Lock()
	delete(s.stateStore, state)
	s.stateMu.Unlock()
}

// ConsumeState 校验并消耗 state，返回保存的 redirect_uri
func (s *QQOAuthService) ConsumeState(state string) (string, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	uri, ok := s.stateStore[state]
	if ok {
		delete(s.stateStore, state)
	}
	return uri, ok
}

// Callback 处理 QQ 回调，返回用户或创建/绑定
func (s *QQOAuthService) Callback(code, state, redirectURI string) (*model.User, string, error) {
	if code == "" {
		return nil, "", errors.New("缺少授权 code")
	}

	accessToken, err := s.getAccessToken(code, redirectURI)
	if err != nil {
		return nil, "", fmt.Errorf("获取 access_token 失败: %w", err)
	}

	openID, err := s.getOpenID(accessToken)
	if err != nil {
		return nil, "", fmt.Errorf("获取 openid 失败: %w", err)
	}

	userInfo, err := s.getUserInfo(accessToken, openID)
	if err != nil {
		return nil, "", fmt.Errorf("获取用户信息失败: %w", err)
	}

	var oauth model.UserOAuth
	err = s.db.Where("provider = ? AND open_id = ?", model.OAuthQQ, openID).First(&oauth).Error
	if err == nil {
		// 已绑定，更新信息并登录
		now := time.Now()
		s.db.Model(&oauth).Updates(map[string]interface{}{
			"nickname":    userInfo.Nickname,
			"avatar_url":  userInfo.FigureURLQQ,
			"access_token": accessToken,
			"expires_at":  now.Add(30 * 24 * time.Hour),
		})
		var u model.User
		s.db.First(&u, "id = ?", oauth.UserID)
		token, err := s.generateUserToken(&u)
		if err != nil {
			return nil, "", err
		}
		return &u, token, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}

	// 未绑定，自动创建新用户
	user, err := s.createUserFromQQ(openID, accessToken, userInfo)
	if err != nil {
		return nil, "", err
	}
	token, err := s.generateUserToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *QQOAuthService) generateUserToken(u *model.User) (string, error) {
	return "", nil // 由 AuthService 处理；此处留占位，实际在 controller 中调 AuthService
}

type qqAccessTokenResp struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (s *QQOAuthService) getAccessToken(code, redirectURI string) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("client_id", s.cfg.QQOAuth.AppID)
	params.Set("client_secret", s.cfg.QQOAuth.AppKey)
	params.Set("code", code)
	params.Set("redirect_uri", redirectURI)

	resp, err := httpGet("https://graph.qq.com/oauth2.0/token?" + params.Encode())
	if err != nil {
		return "", err
	}

	// QQ 返回的是 form 格式：access_token=xxx&expires_in=xxx
	values, err := url.ParseQuery(string(resp))
	if err != nil {
		return "", err
	}
	token := values.Get("access_token")
	if token == "" {
		return "", fmt.Errorf("access_token 为空: %s", string(resp))
	}
	return token, nil
}

type qqOpenIDResp struct {
	ClientID string `json:"client_id"`
	OpenID   string `json:"openid"`
	UnionID  string `json:"unionid"`
}

func (s *QQOAuthService) getOpenID(accessToken string) (string, error) {
	body, err := httpGet("https://graph.qq.com/oauth2.0/me?access_token=" + accessToken + "&unionid=1&fmt=json")
	if err != nil {
		return "", err
	}
	// QQ 可能返回 callback(...) 包裹，需要剥掉
	body = stripJSONP(body)
	var r qqOpenIDResp
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("解析 openid 响应失败: %w, body=%s", err, string(body))
	}
	if r.OpenID == "" {
		return "", errors.New("openid 为空")
	}
	return r.OpenID, nil
}

type qqUserInfoResp struct {
	Ret         int    `json:"ret"`
	Msg         string `json:"msg"`
	Nickname    string `json:"nickname"`
	FigureURLQQ string `json:"figureurl_qq_2"`
	Gender      string `json:"gender"`
}

func (s *QQOAuthService) getUserInfo(accessToken, openID string) (*qqUserInfoResp, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("oauth_consumer_key", s.cfg.QQOAuth.AppID)
	params.Set("openid", openID)
	body, err := httpGet("https://graph.qq.com/user/get_user_info?" + params.Encode())
	if err != nil {
		return nil, err
	}
	var r qqUserInfoResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	if r.Ret != 0 {
		return nil, fmt.Errorf("QQ 用户信息错误: %d - %s", r.Ret, r.Msg)
	}
	return &r, nil
}

func (s *QQOAuthService) createUserFromQQ(openID, accessToken string, info *qqUserInfoResp) (*model.User, error) {
	var user *model.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 生成唯一用户名
		username := generateQQUsername(info.Nickname)
		// 确保唯一
		for i := 1; i < 100; i++ {
			var cnt int64
			tx.Model(&model.User{}).Where("username = ?", username).Count(&cnt)
			if cnt == 0 {
				break
			}
			username = generateQQUsername(info.Nickname) + fmt.Sprintf("%d", i)
		}

		randomPw := randomState(16)
		hash, _ := hashPw(randomPw)

		u := model.User{
			Username:     username,
			PasswordHash: hash,
			Avatar:       info.FigureURLQQ,
			Signature:    "通过 QQ 登录",
			Role:         model.RoleUser,
			Status:       model.UserStatusActive,
		}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		user = &u

		oauth := model.UserOAuth{
			UserID:      u.ID,
			Provider:    model.OAuthQQ,
			OpenID:      openID,
			Nickname:    info.Nickname,
			AvatarURL:   info.FigureURLQQ,
			AccessToken: accessToken,
		}
		if err := tx.Create(&oauth).Error; err != nil {
			return err
		}
		return nil
	})
	return user, err
}

// ===== 工具函数 =====

func randomState(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func stripJSONP(data []byte) []byte {
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "callback(") {
		s = strings.TrimPrefix(s, "callback(")
		s = strings.TrimSuffix(s, ");")
		s = strings.TrimSuffix(s, ")")
	}
	return []byte(s)
}

func httpGet(u string) ([]byte, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func generateQQUsername(nickname string) string {
	// 移除特殊字符
	var b strings.Builder
	for _, r := range nickname {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || len([]rune{r}) > 1 {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" || len([]rune(name)) < 2 {
		name = "qq_user"
	}
	if len([]rune(name)) > 20 {
		name = string([]rune(name)[:20])
	}
	return name
}

// BindQQ 将 QQ 绑定到已有用户
func (s *QQOAuthService) BindQQ(userID, code, redirectURI string) error {
	if code == "" {
		return errors.New("缺少授权 code")
	}
	accessToken, err := s.getAccessToken(code, redirectURI)
	if err != nil {
		return err
	}
	openID, err := s.getOpenID(accessToken)
	if err != nil {
		return err
	}
	var existing model.UserOAuth
	err = s.db.Where("provider = ? AND open_id = ?", model.OAuthQQ, openID).First(&existing).Error
	if err == nil {
		return errors.New("该 QQ 已绑定其他账号")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	userInfo, _ := s.getUserInfo(accessToken, openID)
	oauth := model.UserOAuth{
		UserID:      userID,
		Provider:    model.OAuthQQ,
		OpenID:      openID,
		Nickname:    "",
		AvatarURL:   "",
		AccessToken: accessToken,
	}
	if userInfo != nil {
		oauth.Nickname = userInfo.Nickname
		oauth.AvatarURL = userInfo.FigureURLQQ
	}
	return s.db.Create(&oauth).Error
}

// UnbindQQ 解绑
func (s *QQOAuthService) UnbindQQ(userID string) error {
	return s.db.Where("user_id = ? AND provider = ?", userID, model.OAuthQQ).Delete(&model.UserOAuth{}).Error
}

// GetBinding 查询用户 QQ 绑定
func (s *QQOAuthService) GetBinding(userID string) (*model.UserOAuth, error) {
	var o model.UserOAuth
	err := s.db.Where("user_id = ? AND provider = ?", userID, model.OAuthQQ).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &o, err
}
