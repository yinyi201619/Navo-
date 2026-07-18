// Package database 负责数据库初始化与连接
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/model"
)

// Init 初始化数据库连接并执行迁移
func Init(cfg *config.Config, log *zap.Logger) (*gorm.DB, error) {
	gormLogLevel := parseLogLevel(cfg.Database.LogLevel)

	var db *gorm.DB
	var err error
	switch cfg.Database.Driver {
	case "sqlite":
		// 确保 sqlite 文件目录存在
		if dir := filepath.Dir(cfg.Database.DSN); dir != "" && dir != "." {
			if e := os.MkdirAll(dir, 0755); e != nil {
				return nil, fmt.Errorf("创建 sqlite 目录失败: %w", e)
			}
		}
		db, err = gorm.Open(sqlite.Open(cfg.Database.DSN+"?busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{
			Logger: logger.Default.LogMode(gormLogLevel),
		})
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.Database.DSN), &gorm.Config{
			Logger: logger.Default.LogMode(gormLogLevel),
		})
	default:
		return nil, fmt.Errorf("不支持的数据库驱动: %s", cfg.Database.Driver)
	}
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("迁移失败: %w", err)
	}

	if err := seedDefaults(db, cfg); err != nil {
		return nil, fmt.Errorf("初始化数据失败: %w", err)
	}

	log.Info("数据库初始化完成", zap.String("driver", cfg.Database.Driver))
	return db, nil
}

func parseLogLevel(s string) logger.LogLevel {
	switch s {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.UserVerification{},
		&model.UserOAuth{},
		&model.Category{},
		&model.Topic{},
		&model.Reply{},
		&model.TopicLike{},
		&model.ReplyLike{},
		&model.Favorite{},
		&model.Conversation{},
		&model.Message{},
		&model.Notification{},
		&model.PointLog{},
		&model.Checkin{},
		&model.AdminLog{},
		&model.PointRule{},
		&model.Setting{},
	)
}

func seedDefaults(db *gorm.DB, cfg *config.Config) error {
	// 积分规则
	var ruleCount int64
	db.Model(&model.PointRule{}).Count(&ruleCount)
	if ruleCount == 0 {
		rules := []model.PointRule{
			{ID: 1, Action: "topic", Name: "发主题帖", Points: 5, Experience: 10, DailyLimit: 20},
			{ID: 2, Action: "reply", Name: "发回复", Points: 2, Experience: 3, DailyLimit: 50},
			{ID: 3, Action: "liked", Name: "被点赞", Points: 1, Experience: 1, DailyLimit: 0},
			{ID: 4, Action: "checkin", Name: "每日签到", Points: 3, Experience: 2, DailyLimit: 1},
			{ID: 5, Action: "essence", Name: "被加精", Points: 20, Experience: 30, DailyLimit: 0},
			{ID: 6, Action: "invite", Name: "邀请注册", Points: 50, Experience: 50, DailyLimit: 0},
		}
		if err := db.Create(&rules).Error; err != nil {
			return err
		}
	}

	// 系统设置
	var settingCount int64
	db.Model(&model.Setting{}).Count(&settingCount)
	if settingCount == 0 {
		settings := []model.Setting{
			{Key: "site_name", Value: cfg.Site.Name},
			{Key: "site_description", Value: cfg.Site.Description},
			{Key: "allow_register", Value: boolStr(cfg.Site.AllowRegister)},
			{Key: "upload_limit_mb", Value: fmt.Sprintf("%d", cfg.Site.UploadLimitMB)},
		}
		if err := db.Create(&settings).Error; err != nil {
			return err
		}
	}

	// 默认板块
	var catCount int64
	db.Model(&model.Category{}).Count(&catCount)
	if catCount == 0 {
		categories := []model.Category{
			{Name: "综合讨论", Slug: "general", Description: "QQ 机器人综合讨论区", Icon: "message-circle", SortOrder: 1},
			{Name: "开发交流", Slug: "dev", Description: "机器人开发与技术交流", Icon: "code", SortOrder: 2},
			{Name: "配置教程", Slug: "tutorial", Description: "配置教程与玩法分享", Icon: "book-open", SortOrder: 3},
			{Name: "问题求助", Slug: "help", Description: "使用问题与求助", Icon: "help-circle", SortOrder: 4},
			{Name: "资源分享", Slug: "resource", Description: "插件、资源分享", Icon: "package", SortOrder: 5},
			{Name: "站务公告", Slug: "announce", Description: "站点公告与反馈", Icon: "megaphone", SortOrder: 6},
		}
		if err := db.Create(&categories).Error; err != nil {
			return err
		}
	}

	// 初始管理员
	var adminCount int64
	db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&adminCount)
	if adminCount == 0 {
		hash, _ := HashPassword(cfg.Admin.Password)
		admin := model.User{
			Username:     cfg.Admin.Username,
			Email:        cfg.Admin.Email,
			PasswordHash: hash,
			Role:         model.RoleAdmin,
			Status:       model.UserStatusActive,
		}
		if err := db.Create(&admin).Error; err != nil {
			return err
		}
	}

	return nil
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
