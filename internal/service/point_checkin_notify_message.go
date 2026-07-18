package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"navo-nt-forum/internal/model"
)

// ===== PointService =====

type PointService struct {
	db *gorm.DB
}

func NewPointService(db *gorm.DB) *PointService {
	return &PointService{db: db}
}

// AddPoints 给用户增加积分与经验，并自动判断升级
func (s *PointService) AddPoints(userID, action, refID, remark string) (int, int, error) {
	var rule model.PointRule
	if err := s.db.Where("action = ?", action).First(&rule).Error; err != nil {
		return 0, 0, errors.New("积分规则不存在")
	}

	if rule.DailyLimit > 0 {
		start := time.Now().Truncate(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		var count int64
		s.db.Model(&model.PointLog{}).
			Where("user_id = ? AND action = ? AND created_at >= ? AND created_at < ?", userID, action, start, end).
			Count(&count)
		if count >= int64(rule.DailyLimit) {
			return 0, 0, nil
		}
	}

	return s.addPointsTx(s.db, userID, &rule, refID, remark)
}

func (s *PointService) addPointsTx(tx *gorm.DB, userID string, rule *model.PointRule, refID, remark string) (int, int, error) {
	var u model.User
	if err := tx.First(&u, "id = ?", userID).Error; err != nil {
		return 0, 0, err
	}
	newPoints := u.Points + rule.Points
	newExp := u.Experience + rule.Experience
	newLevel := model.CalcLevel(newExp)

	if err := tx.Model(&u).Updates(map[string]interface{}{
		"points":     newPoints,
		"experience": newExp,
		"level":      newLevel,
	}).Error; err != nil {
		return 0, 0, err
	}

	log := model.PointLog{
		UserID:   userID,
		Action:   rule.Action,
		Delta:    rule.Points,
		ExpDelta: rule.Experience,
		RefID:    refID,
		Remark:   remark,
	}
	return rule.Points, rule.Experience, tx.Create(&log).Error
}

// UserLogs 积分日志
func (s *PointService) UserLogs(userID string, page, size int) ([]model.PointLog, int64, error) {
	var logs []model.PointLog
	var total int64
	q := s.db.Model(&model.PointLog{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs).Error
	return logs, total, err
}

// Rules 获取所有规则
func (s *PointService) Rules() ([]model.PointRule, error) {
	var rules []model.PointRule
	err := s.db.Order("id ASC").Find(&rules).Error
	return rules, err
}

// UpdateRule 更新规则
func (s *PointService) UpdateRule(id int, points, experience, dailyLimit int) error {
	return s.db.Model(&model.PointRule{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"points":      points,
			"experience":  experience,
			"daily_limit": dailyLimit,
		}).Error
}

// ===== CheckinService =====

type CheckinService struct {
	db    *gorm.DB
	point *PointService
}

func NewCheckinService(db *gorm.DB, point *PointService) *CheckinService {
	return &CheckinService{db: db, point: point}
}

// Checkin 签到，返回连续天数与获得积分
func (s *CheckinService) Checkin(userID string) (int, int, error) {
	today := time.Now().Truncate(24 * time.Hour)
	var existing model.Checkin
	err := s.db.Where("user_id = ? AND check_date = ?", userID, today).First(&existing).Error
	if err == nil {
		return 0, 0, ErrAlreadyCheckedIn
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, err
	}

	yesterday := today.Add(-24 * time.Hour)
	var last model.Checkin
	continuous := 1
	err = s.db.Where("user_id = ? AND check_date = ?", userID, yesterday).First(&last).Error
	if err == nil {
		continuous = last.Continuous + 1
	}

	// 基础积分 + 连续签到奖励
	points := 3 + min(continuous-1, 6) // 最多 +6，即 3+6=9
	if continuous >= 7 {
		points += 10 // 7 天额外奖励
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		rec := model.Checkin{
			UserID:     userID,
			CheckDate:  today,
			Continuous: continuous,
			Points:     points,
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}

		var rule model.PointRule
		if err := tx.Where("action = ?", "checkin").First(&rule).Error; err != nil {
			return err
		}

		var u model.User
		if err := tx.First(&u, "id = ?", userID).Error; err != nil {
			return err
		}

		newPoints := u.Points + points
		newExp := u.Experience + rule.Experience
		newLevel := model.CalcLevel(newExp)

		if err := tx.Model(&u).Updates(map[string]interface{}{
			"points":     newPoints,
			"experience": newExp,
			"level":      newLevel,
		}).Error; err != nil {
			return err
		}

		baseLog := model.PointLog{
			UserID:   userID,
			Action:   "checkin",
			Delta:    3,
			ExpDelta: rule.Experience,
			Remark:   "签到奖励",
		}
		if err := tx.Create(&baseLog).Error; err != nil {
			return err
		}

		extra := points - 3
		if extra > 0 {
			tx.Create(&model.PointLog{
				UserID:   userID,
				Action:   "checkin",
				Delta:    extra,
				ExpDelta: 0,
				Remark:   "连续签到奖励",
			})
		}
		return nil
	})
	return continuous, points, err
}

// TodayCheckedIn 今日是否已签到
func (s *CheckinService) TodayCheckedIn(userID string) bool {
	today := time.Now().Truncate(24 * time.Hour)
	var cnt int64
	s.db.Model(&model.Checkin{}).Where("user_id = ? AND check_date = ?", userID, today).Count(&cnt)
	return cnt > 0
}

// ContinuousDays 连续签到天数
func (s *CheckinService) ContinuousDays(userID string) int {
	today := time.Now().Truncate(24 * time.Hour)
	var rec model.Checkin
	err := s.db.Where("user_id = ? AND check_date = ?", userID, today).First(&rec).Error
	if err == nil {
		return rec.Continuous
	}
	yesterday := today.Add(-24 * time.Hour)
	err = s.db.Where("user_id = ? AND check_date = ?", userID, yesterday).First(&rec).Error
	if err == nil {
		return rec.Continuous
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ===== NotificationService =====

type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// Send 发送通知
func (s *NotificationService) Send(userID string, typ model.NotificationType, title, content, link string) error {
	n := model.Notification{
		UserID:  userID,
		Type:    typ,
		Title:   title,
		Content: content,
		Link:    link,
	}
	return s.db.Create(&n).Error
}

// List 用户通知列表
func (s *NotificationService) List(userID string, page, size int, unreadOnly bool) ([]model.Notification, int64, error) {
	var list []model.Notification
	var total int64
	q := s.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("is_read = 0")
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// UnreadCount 未读数
func (s *NotificationService) UnreadCount(userID string) int64 {
	var cnt int64
	s.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = 0", userID).Count(&cnt)
	return cnt
}

// MarkRead 标记已读
func (s *NotificationService) MarkRead(userID, id string) error {
	return s.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true).Error
}

// MarkAllRead 全部已读
func (s *NotificationService) MarkAllRead(userID string) error {
	return s.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Update("is_read", true).Error
}

// ===== MessageService =====

type MessageService struct {
	db    *gorm.DB
	notify *NotificationService
}

func NewMessageService(db *gorm.DB, notify *NotificationService) *MessageService {
	return &MessageService{db: db, notify: notify}
}

// Conversations 会话列表
func (s *MessageService) Conversations(userID string, page, size int) ([]model.Conversation, int64, error) {
	var list []model.Conversation
	var total int64
	q := s.db.Model(&model.Conversation{}).Where("user_a_id = ? OR user_b_id = ?", userID, userID)
	q.Count(&total)
	err := q.Preload("UserA").Preload("UserB").
		Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// GetConversation 获取指定会话
func (s *MessageService) GetConversation(userID, convID string) (*model.Conversation, error) {
	var c model.Conversation
	err := s.db.Where("id = ? AND (user_a_id = ? OR user_b_id = ?)", convID, userID, userID).
		Preload("UserA").Preload("UserB").First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationNotFound
	}
	return &c, err
}

// GetOrCreateConversation 获取或创建两用户间会话
func (s *MessageService) GetOrCreateConversation(userA, userB string) (*model.Conversation, error) {
	var c model.Conversation
	err := s.db.Where("(user_a_id = ? AND user_b_id = ?) OR (user_a_id = ? AND user_b_id = ?)",
		userA, userB, userB, userA).First(&c).Error
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	c = model.Conversation{
		UserAID: userA,
		UserBID: userB,
	}
	err = s.db.Create(&c).Error
	return &c, err
}

// Messages 消息列表（分页，按时间正序便于聊天展示）
func (s *MessageService) Messages(convID, userID string, page, size int) ([]model.Message, int64, error) {
	// 先鉴权
	var c model.Conversation
	err := s.db.Where("id = ? AND (user_a_id = ? OR user_b_id = ?)", convID, userID, userID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, ErrConversationNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	var list []model.Message
	var total int64
	q := s.db.Model(&model.Message{}).Where("conversation_id = ?", convID)
	q.Count(&total)
	err = q.Preload("Sender").Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	// 反转顺序以便正序显示
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, total, err
}

// SendMessage 发送消息
func (s *MessageService) SendMessage(senderID, convID, content string) (*model.Message, error) {
	var c model.Conversation
	err := s.db.Where("id = ? AND (user_a_id = ? OR user_b_id = ?)", convID, senderID, senderID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}

	var receiverID string
	if c.UserAID == senderID {
		receiverID = c.UserBID
	} else {
		receiverID = c.UserAID
	}

	msg := model.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Content:        content,
	}
	preview := content
	if len([]rune(preview)) > 100 {
		preview = string([]rune(preview)[:100])
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&c).Updates(map[string]interface{}{
			"last_message": preview,
			"updated_at":   time.Now(),
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 通知
	go s.notify.Send(receiverID, model.NotifyMessage, "收到一条新私信", preview, "/messages/"+convID)

	return &msg, nil
}

// MarkRead 标记会话内消息已读
func (s *MessageService) MarkRead(convID, userID string) error {
	return s.db.Model(&model.Message{}).
		Where("conversation_id = ? AND sender_id != ? AND is_read = 0", convID, userID).
		Update("is_read", true).Error
}

// UnreadMessageCount 未读私信总数
func (s *MessageService) UnreadMessageCount(userID string) int64 {
	// 找出用户参与的会话
	var convs []string
	s.db.Model(&model.Conversation{}).
		Where("user_a_id = ? OR user_b_id = ?", userID, userID).
		Pluck("id", &convs)
	if len(convs) == 0 {
		return 0
	}
	var cnt int64
	s.db.Model(&model.Message{}).
		Where("conversation_id IN ? AND sender_id != ? AND is_read = 0", convs, userID).
		Count(&cnt)
	return cnt
}
