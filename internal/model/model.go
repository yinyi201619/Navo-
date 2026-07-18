// Package model 定义数据库模型
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ===== 基础类型 =====

// UserStatus 用户状态
type UserStatus int8

const (
	UserStatusBanned UserStatus = -1 // 封禁
	UserStatusMuted  UserStatus = 0  // 禁言
	UserStatusActive UserStatus = 1  // 正常
)

// Role 用户角色
type Role string

const (
	RoleUser      Role = "user"
	RoleModerator Role = "moderator"
	RoleAdmin     Role = "admin"
)

// VerifyType 认证类型
type VerifyType string

const (
	VerifyOfficial     VerifyType = "official"     // 官方认证
	VerifyContributor  VerifyType = "contributor"  // 优质贡献者
	VerifyBot          VerifyType = "bot"          // 机器人认证
)

// NotificationType 通知类型
type NotificationType string

const (
	NotifyReply   NotificationType = "reply"
	NotifyLike    NotificationType = "like"
	NotifySystem  NotificationType = "system"
	NotifyMessage NotificationType = "message"
)

// ===== 模型 =====

// User 用户表
type User struct {
	ID           string     `gorm:"primaryKey;size:36" json:"id"`
	Username     string     `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email        string     `gorm:"size:100;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	Avatar       string     `gorm:"size:500" json:"avatar"`
	Signature    string     `gorm:"size:200" json:"signature"`
	Role         Role       `gorm:"size:20;not null;default:user;index" json:"role"`
	Points       int        `gorm:"not null;default:0" json:"points"`
	Experience   int        `gorm:"not null;default:0" json:"experience"`
	Level        int        `gorm:"not null;default:1" json:"level"`
	Status       UserStatus `gorm:"not null;default:1" json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	LastLoginIP  string     `gorm:"size:45" json:"lastLoginIp"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}

// UserVerification 用户认证
type UserVerification struct {
	ID        string      `gorm:"primaryKey;size:36" json:"id"`
	UserID    string      `gorm:"size:36;not null;uniqueIndex:uk_user_type,priority:1" json:"userId"`
	VerifyType VerifyType `gorm:"size:20;not null;uniqueIndex:uk_user_type,priority:2" json:"verifyType"`
	Label     string      `gorm:"size:100;not null" json:"label"`
	GrantedBy string      `gorm:"size:36;not null" json:"grantedBy"`
	GrantedAt time.Time   `json:"grantedAt"`
	RevokedAt *time.Time  `json:"revokedAt"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (v *UserVerification) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	return nil
}

// Category 板块
type Category struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	ParentID    *string   `gorm:"size:36;index" json:"parentId"`
	Name        string    `gorm:"size:50;not null" json:"name"`
	Slug        string    `gorm:"size:80;uniqueIndex;not null" json:"slug"`
	Description string    `gorm:"size:500" json:"description"`
	Icon        string    `gorm:"size:100" json:"icon"`
	SortOrder   int       `gorm:"not null;default:0;index" json:"sortOrder"`
	TopicCount  int       `gorm:"not null;default:0" json:"topicCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	Parent   *Category  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children []Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}

// Topic 帖子
type Topic struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	CategoryID    string     `gorm:"size:36;not null;index" json:"categoryId"`
	AuthorID      string     `gorm:"size:36;not null;index" json:"authorId"`
	Title         string     `gorm:"size:200;not null" json:"title"`
	Content       string     `gorm:"type:mediumtext;not null" json:"content"`
	Tags          string     `gorm:"size:255" json:"tags"`
	ReplyCount    int        `gorm:"not null;default:0" json:"replyCount"`
	ViewCount     int        `gorm:"not null;default:0" json:"viewCount"`
	LikeCount     int        `gorm:"not null;default:0" json:"likeCount"`
	FavoriteCount int        `gorm:"not null;default:0" json:"favoriteCount"`
	IsPinned      bool       `gorm:"not null;default:false" json:"isPinned"`
	IsEssence     bool       `gorm:"not null;default:false" json:"isEssence"`
	Status        int8       `gorm:"not null;default:1" json:"status"`
	LastReplyAt   *time.Time `gorm:"index" json:"lastReplyAt"`
	CreatedAt     time.Time  `gorm:"index" json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`

	Author   *User     `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Category *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
}

func (t *Topic) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}

// Reply 回复
type Reply struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	TopicID   string    `gorm:"size:36;not null;index:idx_topic,priority:1" json:"topicId"`
	ParentID  *string   `gorm:"size:36;index" json:"parentId"`
	AuthorID  string    `gorm:"size:36;not null;index" json:"authorId"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	LikeCount int       `gorm:"not null;default:0" json:"likeCount"`
	Floor     int       `gorm:"not null;index:idx_topic,priority:2" json:"floor"`
	Status    int8      `gorm:"not null;default:1" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Author  *User   `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Parent  *Reply  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Topic   *Topic  `gorm:"foreignKey:TopicID" json:"topic,omitempty"`
}

func (r *Reply) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// TopicLike 帖子点赞
type TopicLike struct {
	TopicID   string    `gorm:"primaryKey;size:36" json:"topicId"`
	UserID    string    `gorm:"primaryKey;size:36" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReplyLike 回复点赞
type ReplyLike struct {
	ReplyID   string    `gorm:"primaryKey;size:36" json:"replyId"`
	UserID    string    `gorm:"primaryKey;size:36" json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

// Favorite 收藏
type Favorite struct {
	UserID    string    `gorm:"primaryKey;size:36" json:"userId"`
	TopicID   string    `gorm:"primaryKey;size:36" json:"topicId"`
	CreatedAt time.Time `json:"createdAt"`

	Topic *Topic `gorm:"foreignKey:TopicID" json:"topic,omitempty"`
}

// Conversation 私信会话
type Conversation struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	UserAID     string    `gorm:"size:36;not null;uniqueIndex:uk_pair,priority:1;index" json:"userAId"`
	UserBID     string    `gorm:"size:36;not null;uniqueIndex:uk_pair,priority:2;index" json:"userBId"`
	LastMessage string    `gorm:"size:500" json:"lastMessage"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	UserA *User `gorm:"foreignKey:UserAID" json:"userA,omitempty"`
	UserB *User `gorm:"foreignKey:UserBID" json:"userB,omitempty"`
}

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}

// Message 私信消息
type Message struct {
	ID             string    `gorm:"primaryKey;size:36" json:"id"`
	ConversationID string    `gorm:"size:36;not null;index:idx_conv,priority:1" json:"conversationId"`
	SenderID       string    `gorm:"size:36;not null;index" json:"senderId"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	IsRead         bool      `gorm:"not null;default:false" json:"isRead"`
	CreatedAt      time.Time `gorm:"index:idx_conv,priority:2" json:"createdAt"`

	Sender       *User         `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	Conversation *Conversation `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}

// Notification 通知
type Notification struct {
	ID        string            `gorm:"primaryKey;size:36" json:"id"`
	UserID    string            `gorm:"size:36;not null;index:idx_user_read,priority:1" json:"userId"`
	Type      NotificationType  `gorm:"size:30;not null" json:"type"`
	Title     string            `gorm:"size:200;not null" json:"title"`
	Content   string            `gorm:"size:500" json:"content"`
	Link      string            `gorm:"size:500" json:"link"`
	IsRead    bool              `gorm:"not null;default:false;index:idx_user_read,priority:2" json:"isRead"`
	CreatedAt time.Time         `gorm:"index:idx_user_read,priority:3" json:"createdAt"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	return nil
}

// PointLog 积分日志
type PointLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"size:36;not null;index:idx_user,priority:1" json:"userId"`
	Action    string    `gorm:"size:30;not null" json:"action"`
	Delta     int       `gorm:"not null" json:"delta"`
	ExpDelta  int       `gorm:"not null;default:0" json:"expDelta"`
	RefID     string    `gorm:"size:36" json:"refId"`
	Remark    string    `gorm:"size:200" json:"remark"`
	CreatedAt time.Time `gorm:"index:idx_user,priority:2" json:"createdAt"`
}

// Checkin 签到
type Checkin struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"size:36;not null;uniqueIndex:uk_user_date,priority:1" json:"userId"`
	CheckDate  time.Time `gorm:"type:date;not null;uniqueIndex:uk_user_date,priority:2" json:"checkDate"`
	Continuous int       `gorm:"not null;default:1" json:"continuous"`
	Points     int       `gorm:"not null;default:0" json:"points"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AdminLog 管理员操作日志
type AdminLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminID    string    `gorm:"size:36;not null;index:idx_admin,priority:1" json:"adminId"`
	Action     string    `gorm:"size:50;not null" json:"action"`
	TargetType string    `gorm:"size:30;not null;index:idx_target,priority:1" json:"targetType"`
	TargetID   string    `gorm:"size:36;index:idx_target,priority:2" json:"targetId"`
	Detail     string    `gorm:"type:text" json:"detail"`
	IP         string    `gorm:"size:45" json:"ip"`
	CreatedAt  time.Time `gorm:"index:idx_admin,priority:2" json:"createdAt"`
}

// PointRule 积分规则
type PointRule struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	Action     string    `gorm:"size:30;uniqueIndex;not null" json:"action"`
	Name       string    `gorm:"size:50;not null" json:"name"`
	Points     int       `gorm:"not null;default:0" json:"points"`
	Experience int       `gorm:"not null;default:0" json:"experience"`
	DailyLimit int       `gorm:"not null;default:0" json:"dailyLimit"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Setting 系统设置
type Setting struct {
	Key       string    `gorm:"primaryKey;size:50" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ===== 等级阈值 =====

// LevelThresholds 经验阈值表
var LevelThresholds = []struct {
	Level    int
	MinExp   int
	Name     string
	Color    string
}{
	{1, 0, "新手", "#9CA3AF"},
	{2, 100, "学徒", "#10B981"},
	{3, 500, "行家", "#3B82F6"},
	{4, 2000, "专家", "#8B5CF6"},
	{5, 8000, "大师", "#F59E0B"},
	{6, 30000, "宗师", "#EF4444"},
	{7, 100000, "传说", "#FACC15"},
}

// CalcLevel 根据经验计算等级
func CalcLevel(exp int) int {
	level := 1
	for _, t := range LevelThresholds {
		if exp >= t.MinExp {
			level = t.Level
		}
	}
	return level
}

// LevelInfo 等级信息
type LevelInfo struct {
	Level      int    `json:"level"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	CurrentExp int    `json:"currentExp"`
	NextExp    int    `json:"nextExp"`  // 下一级所需经验，0 表示已满级
	Progress   int    `json:"progress"` // 0-100
}

// GetLevelInfo 获取等级详情
func GetLevelInfo(exp int) LevelInfo {
	info := LevelInfo{CurrentExp: exp}
	currentIdx := 0
	for i, t := range LevelThresholds {
		if exp >= t.MinExp {
			info.Level = t.Level
			info.Name = t.Name
			info.Color = t.Color
			currentIdx = i
		}
	}
	if currentIdx < len(LevelThresholds)-1 {
		next := LevelThresholds[currentIdx+1]
		info.NextExp = next.MinExp
		cur := LevelThresholds[currentIdx].MinExp
		if next.MinExp > cur {
			info.Progress = (exp - cur) * 100 / (next.MinExp - cur)
		}
	} else {
		info.NextExp = 0
		info.Progress = 100
	}
	return info
}
