// Command server Navo NT QQ BOT 论坛入口
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/database"
	"navo-nt-forum/internal/router"
	"navo-nt-forum/internal/service"
	tpl "navo-nt-forum/internal/template"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	migrate := flag.Bool("migrate", false, "仅执行数据库迁移")
	flag.Parse()

	logger, _ := newLogger("info", "console")
	defer logger.Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}
	logger.Info("配置加载成功", zap.String("config", *configPath))

	logger, _ = newLogger(cfg.Log.Level, cfg.Log.Encoding)
	defer logger.Sync()

	db, err := database.Init(cfg, logger)
	if err != nil {
		logger.Fatal("数据库初始化失败", zap.Error(err))
	}
	if *migrate {
		logger.Info("数据库迁移完成")
		return
	}

	userSvc := service.NewUserService(db)
	tplEngine, err := tpl.NewEngine(tpl.OSFS{}, "web/templates", cfg.Site.Name, cfg.Site.Description, userSvc)
	if err != nil {
		logger.Fatal("模板引擎初始化失败", zap.Error(err))
	}
	logger.Info("模板引擎初始化完成")

	router.SetStaticFS(http.Dir("web/static"))

	handler := router.NewRouter(cfg, db, tplEngine, logger)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("正在关闭服务器...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Fatal("服务器关闭失败", zap.Error(err))
		}
		logger.Info("服务器已关闭")
	}()

	logger.Info("服务器启动", zap.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("服务器启动失败", zap.Error(err))
	}
}

func newLogger(level, encoding string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(lvl),
		Development: false,
		Encoding:    encoding,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
			EncodeDuration: zapcore.SecondsDurationEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	return cfg.Build()
}
