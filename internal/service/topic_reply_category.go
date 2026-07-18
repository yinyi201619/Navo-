package service

import (
	"errors"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"gorm.io/gorm"

	"navo-nt-forum/internal/model"
)

// ===== CategoryService =====

type CategoryService struct {
	db *gorm.DB
}

func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

func (s *CategoryService) All() ([]model.Category, error) {
	var list []model.Category
	err := s.db.Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

func (s *CategoryService) BySlug(slug string) (*model.Category, error) {
	var c model.Category
	err := s.db.Where("slug = ?", slug).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCategoryNotFound
	}
	return &c, err
}

func (s *CategoryService) ByID(id string) (*model.Category, error) {
	var c model.Category
	err := s.db.First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCategoryNotFound
	}
	return &c, err
}

func (s *CategoryService) Create(name, slug, description, icon string, sortOrder int) error {
	c := model.Category{
		Name:        name,
		Slug:        slug,
		Description: description,
		Icon:        icon,
		SortOrder:   sortOrder,
	}
	return s.db.Create(&c).Error
}

func (s *CategoryService) Update(id, name, slug, description, icon string, sortOrder int) error {
	return s.db.Model(&model.Category{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":        name,
		"slug":        slug,
		"description": description,
		"icon":        icon,
		"sort_order":  sortOrder,
	}).Error
}

func (s *CategoryService) Delete(id string) error {
	return s.db.Delete(&model.Category{}, "id = ?", id).Error
}

func (s *CategoryService) IncTopicCount(id string, delta int) error {
	return s.db.Model(&model.Category{}).Where("id = ?", id).
		UpdateColumn("topic_count", gorm.Expr("topic_count + ?", delta)).Error
}

// ===== TopicService =====

type TopicService struct {
	db       *gorm.DB
	category *CategoryService
	point    *PointService
	notify   *NotificationService
}

func NewTopicService(db *gorm.DB, cat *CategoryService, point *PointService, notify *NotificationService) *TopicService {
	return &TopicService{db: db, category: cat, point: point, notify: notify}
}

func (s *TopicService) List(page, size int, categoryID, sort, keyword string) ([]model.Topic, int64, error) {
	var list []model.Topic
	var total int64
	q := s.db.Model(&model.Topic{}).Where("status = 1")
	if categoryID != "" {
		q = q.Where("category_id = ?", categoryID)
	}
	if keyword != "" {
		q = q.Where("title LIKE ?", "%"+keyword+"%")
	}
	q.Count(&total)
	order := "is_pinned DESC, created_at DESC"
	switch sort {
	case "hot":
		order = "is_pinned DESC, (like_count + reply_count * 2 + view_count / 10) DESC, created_at DESC"
	case "essence":
		q = q.Where("is_essence = 1")
	case "latest":
		order = "is_pinned DESC, created_at DESC"
	}
	err := q.Preload("Author").Preload("Category").
		Order(order).
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (s *TopicService) Get(id string) (*model.Topic, error) {
	var t model.Topic
	err := s.db.Preload("Author").Preload("Category").First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTopicNotFound
	}
	return &t, err
}

func (s *TopicService) Create(authorID, categoryID, title, content, tags string) (*model.Topic, error) {
	cat, err := s.category.ByID(categoryID)
	if err != nil {
		return nil, err
	}
	t := model.Topic{
		CategoryID: cat.ID,
		AuthorID:   authorID,
		Title:      title,
		Content:    content,
		Tags:       tags,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Category{}).Where("id = ?", cat.ID).
			UpdateColumn("topic_count", gorm.Expr("topic_count + 1")).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 积分
	_, _, _ = s.point.AddPoints(authorID, "topic", t.ID, "发布主题")
	return &t, nil
}

func (s *TopicService) Update(id, userID, categoryID, title, content, tags string, isAdmin bool) error {
	var t model.Topic
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return ErrTopicNotFound
	}
	if t.AuthorID != userID && !isAdmin {
		return ErrNoPermission
	}
	return s.db.Model(&t).Updates(map[string]interface{}{
		"category_id": categoryID,
		"title":       title,
		"content":     content,
		"tags":        tags,
	}).Error
}

func (s *TopicService) IncView(id string) {
	s.db.Model(&model.Topic{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1"))
}

func (s *TopicService) ToggleLike(userID, topicID string) (bool, int, error) {
	var like model.TopicLike
	err := s.db.Where("topic_id = ? AND user_id = ?", topicID, userID).First(&like).Error
	liked := false
	if err == nil {
		// 取消点赞
		s.db.Delete(&like)
		s.db.Model(&model.Topic{}).Where("id = ?", topicID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		liked = false
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		s.db.Create(&model.TopicLike{TopicID: topicID, UserID: userID})
		s.db.Model(&model.Topic{}).Where("id = ?", topicID).
			UpdateColumn("like_count", gorm.Expr("like_count + 1"))
		liked = true
		var t model.Topic
		s.db.First(&t, "id = ?", topicID)
		if t.AuthorID != userID {
			_, _, _ = s.point.AddPoints(t.AuthorID, "liked", topicID, "帖子被点赞")
		}
	} else {
		return false, 0, err
	}
	var cnt int64
	s.db.Model(&model.Topic{}).Where("id = ?", topicID).Pluck("like_count", &cnt)
	return liked, int(cnt), nil
}

func (s *TopicService) HasLiked(userID, topicID string) bool {
	var cnt int64
	s.db.Model(&model.TopicLike{}).Where("topic_id = ? AND user_id = ?", topicID, userID).Count(&cnt)
	return cnt > 0
}

func (s *TopicService) ToggleFavorite(userID, topicID string) (bool, error) {
	var fav model.Favorite
	err := s.db.Where("user_id = ? AND topic_id = ?", userID, topicID).First(&fav).Error
	favActive := false
	if err == nil {
		s.db.Delete(&fav)
		s.db.Model(&model.Topic{}).Where("id = ?", topicID).
			UpdateColumn("favorite_count", gorm.Expr("GREATEST(favorite_count - 1, 0)"))
		favActive = false
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		s.db.Create(&model.Favorite{UserID: userID, TopicID: topicID})
		s.db.Model(&model.Topic{}).Where("id = ?", topicID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1"))
		favActive = true
	} else {
		return false, err
	}
	return favActive, nil
}

func (s *TopicService) HasFavorited(userID, topicID string) bool {
	var cnt int64
	s.db.Model(&model.Favorite{}).Where("user_id = ? AND topic_id = ?", userID, topicID).Count(&cnt)
	return cnt > 0
}

// Moderate 管理员操作：置顶/加精/删除
func (s *TopicService) Moderate(id, action string, val bool) error {
	updates := map[string]interface{}{}
	switch action {
	case "pin":
		updates["is_pinned"] = val
	case "essence":
		updates["is_essence"] = val
		var t model.Topic
		s.db.First(&t, "id = ?", id)
		if val {
			_, _, _ = s.point.AddPoints(t.AuthorID, "essence", id, "帖子加精")
		}
	case "delete":
		return s.db.Model(&model.Topic{}).Where("id = ?", id).Update("status", 0).Error
	case "restore":
		return s.db.Model(&model.Topic{}).Where("id = ?", id).Update("status", 1).Error
	}
	return s.db.Model(&model.Topic{}).Where("id = ?", id).Updates(updates).Error
}

// AdminList 后台帖子列表
func (s *TopicService) AdminList(page, size int, keyword string) ([]model.Topic, int64, error) {
	var list []model.Topic
	var total int64
	q := s.db.Model(&model.Topic{})
	if keyword != "" {
		q = q.Where("title LIKE ?", "%"+keyword+"%")
	}
	q.Count(&total)
	err := q.Preload("Author").Preload("Category").
		Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// ===== ReplyService =====

type ReplyService struct {
	db     *gorm.DB
	point  *PointService
	notify *NotificationService
}

func NewReplyService(db *gorm.DB, point *PointService, notify *NotificationService) *ReplyService {
	return &ReplyService{db: db, point: point, notify: notify}
}

func (s *ReplyService) ListByTopic(topicID string, page, size int) ([]model.Reply, int64, error) {
	var list []model.Reply
	var total int64
	q := s.db.Model(&model.Reply{}).Where("topic_id = ? AND status = 1", topicID)
	q.Count(&total)
	err := q.Preload("Author").Preload("Parent.Author").
		Order("floor ASC").
		Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (s *ReplyService) Create(authorID, topicID, parentID, content string) (*model.Reply, error) {
	var t model.Topic
	if err := s.db.First(&t, "id = ?", topicID).Error; err != nil {
		return nil, ErrTopicNotFound
	}
	// 计算楼层
	var maxFloor int
	s.db.Model(&model.Reply{}).Where("topic_id = ?", topicID).
		Select("COALESCE(MAX(floor), 0)").Scan(&maxFloor)

	r := model.Reply{
		TopicID:  topicID,
		AuthorID: authorID,
		Content:  content,
		Floor:    maxFloor + 1,
	}
	if parentID != "" {
		r.ParentID = &parentID
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&t).Updates(map[string]interface{}{
			"reply_count":  gorm.Expr("reply_count + 1"),
			"last_reply_at": now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	// 积分
	_, _, _ = s.point.AddPoints(authorID, "reply", r.ID, "发表回复")

	// 通知楼主
	if t.AuthorID != authorID {
		preview := content
		if len([]rune(preview)) > 50 {
			preview = string([]rune(preview)[:50])
		}
		go s.notify.Send(t.AuthorID, model.NotifyReply, "你的帖子有新回复", preview, "/t/"+topicID)
	}
	// 通知被回复人
	if parentID != "" {
		var parent model.Reply
		s.db.First(&parent, "id = ?", parentID)
		if parent.AuthorID != authorID && parent.AuthorID != t.AuthorID {
			preview := content
			if len([]rune(preview)) > 50 {
				preview = string([]rune(preview)[:50])
			}
			go s.notify.Send(parent.AuthorID, model.NotifyReply, "有人回复了你", preview, "/t/"+topicID)
		}
	}
	return &r, nil
}

func (s *ReplyService) ToggleLike(userID, replyID string) (bool, int, error) {
	var like model.ReplyLike
	err := s.db.Where("reply_id = ? AND user_id = ?", replyID, userID).First(&like).Error
	liked := false
	if err == nil {
		s.db.Delete(&like)
		s.db.Model(&model.Reply{}).Where("id = ?", replyID).
			UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - 1, 0)"))
		liked = false
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		s.db.Create(&model.ReplyLike{ReplyID: replyID, UserID: userID})
		s.db.Model(&model.Reply{}).Where("id = ?", replyID).
			UpdateColumn("like_count", gorm.Expr("like_count + 1"))
		liked = true
		var r model.Reply
		s.db.First(&r, "id = ?", replyID)
		if r.AuthorID != userID {
			_, _, _ = s.point.AddPoints(r.AuthorID, "liked", replyID, "回复被点赞")
		}
	} else {
		return false, 0, err
	}
	var cnt int64
	s.db.Model(&model.Reply{}).Where("id = ?", replyID).Pluck("like_count", &cnt)
	return liked, int(cnt), nil
}

func (s *ReplyService) Delete(id, userID string, isAdmin bool) error {
	var r model.Reply
	if err := s.db.First(&r, "id = ?", id).Error; err != nil {
		return ErrReplyNotFound
	}
	if r.AuthorID != userID && !isAdmin {
		return ErrNoPermission
	}
	return s.db.Model(&r).Update("status", 0).Error
}

// ===== SearchService =====

type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

type SearchResult struct {
	Topics      []model.Topic
	Users       []model.User
	Categories  []model.Category
	Total       int64
	TopicCnt    int64
	UserCnt     int64
	CategoryCnt int64
}

func (s *SearchService) Search(q string, page, size int) (*SearchResult, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return &SearchResult{}, nil
	}
	result := &SearchResult{}

	var topics []model.Topic
	var tCnt int64
	tq := s.db.Model(&model.Topic{}).Where("status = 1 AND (title LIKE ? OR content LIKE ?)", "%"+q+"%", "%"+q+"%")
	tq.Count(&tCnt)
	tq.Preload("Author").Preload("Category").
		Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&topics)
	result.Topics = topics
	result.TopicCnt = tCnt

	var users []model.User
	var uCnt int64
	uq := s.db.Model(&model.User{}).Where("username LIKE ?", "%"+q+"%")
	uq.Count(&uCnt)
	uq.Limit(10).Find(&users)
	result.Users = users
	result.UserCnt = uCnt

	var categories []model.Category
	var cCnt int64
	cq := s.db.Model(&model.Category{}).Where("name LIKE ? OR description LIKE ?", "%"+q+"%", "%"+q+"%")
	cq.Count(&cCnt)
	cq.Order("sort_order ASC, id ASC").Limit(20).Find(&categories)
	result.Categories = categories
	result.CategoryCnt = cCnt

	result.Total = tCnt + uCnt + cCnt
	return result, nil
}

// ===== Markdown 渲染 =====

var mdRenderer = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
)

func RenderMarkdown(src string) string {
	var buf strings.Builder
	if err := mdRenderer.Convert([]byte(src), &buf); err != nil {
		return src
	}
	return buf.String()
}
