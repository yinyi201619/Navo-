// Package router 路由注册
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/controller"
	"navo-nt-forum/internal/middleware"
	"navo-nt-forum/internal/service"
	tpl "navo-nt-forum/internal/template"
	"gorm.io/gorm"
)

// NewRouter 构造路由
func NewRouter(cfg *config.Config, db *gorm.DB, tplEngine *tpl.Engine, log *zap.Logger) http.Handler {
	r := chi.NewRouter()

	// 中间件
	r.Use(middleware.Recovery(log))
	r.Use(middleware.AccessLog(log))
	r.Use(middleware.Auth(newAuthService(cfg, db)))

	// 初始化服务
	svcs := initServices(db, cfg)

	// 控制器
	base := controller.NewBase(cfg, db, tplEngine, svcs)
	homeCtl := controller.NewHomeController(base)
	authCtl := controller.NewAuthController(base)
	categoryCtl := controller.NewCategoryController(base)
	topicCtl := controller.NewTopicController(base)
	userCtl := controller.NewUserController(base)
	messageCtl := controller.NewMessageController(base)
	notifyCtl := controller.NewNotificationController(base)
	searchCtl := controller.NewSearchController(base)
	apiCtl := controller.NewAPIController(base)
	adminCtl := controller.NewAdminController(base)

	// 静态资源
	r.Handle("/static/*", http.StripPrefix("/static/", staticFileHandler()))

	// 前台页面
	r.Get("/", homeCtl.Home)
	r.Get("/login", authCtl.LoginPage)
	r.Post("/login", authCtl.LoginSubmit)
	r.Get("/register", authCtl.RegisterPage)
	r.Post("/register", authCtl.RegisterSubmit)
	r.Post("/logout", authCtl.Logout)

	// QQ OAuth
	r.Get("/auth/qq", authCtl.QQLogin)
	r.Get("/auth/qq/callback", authCtl.QQCallback)

	// 板块
	r.Get("/categories", categoryCtl.List)
	r.Get("/c/{slug}", categoryCtl.Detail)

	// 帖子
	r.Get("/t/{id}", topicCtl.Detail)
	r.Get("/search", searchCtl.Search)

	// 用户主页
	r.Get("/u/{username}", userCtl.Profile)
	r.Get("/u/{username}/topics", userCtl.UserTopics)

	// 需要登录的路由
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin)

		r.Get("/t/new", topicCtl.NewPage)
		r.Post("/t/new", topicCtl.Create)
		r.Post("/t/{id}/reply", topicCtl.Reply)

		r.Get("/messages", messageCtl.Index)
		r.Get("/messages/new", messageCtl.NewConversation)
		r.Get("/messages/{id}", messageCtl.Conversation)
		r.Post("/messages/{id}", messageCtl.Send)

		r.Get("/settings", userCtl.SettingsPage)
		r.Post("/settings", userCtl.SettingsSave)
		r.Get("/settings/security", userCtl.SecurityPage)
		r.Post("/settings/password", userCtl.ChangePassword)

		r.Get("/notifications", notifyCtl.Index)

		// API 接口
		r.Route("/api", func(r chi.Router) {
			r.Post("/topics/{id}/like", apiCtl.LikeTopic)
			r.Post("/topics/{id}/favorite", apiCtl.FavoriteTopic)
			r.Post("/replies/{id}/like", apiCtl.LikeReply)
			r.Post("/checkin", apiCtl.Checkin)
		})
	})

	// 后台路由
	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.RequireAdmin)

		r.Get("/", adminCtl.Dashboard)
		r.Get("/users", adminCtl.Users)
		r.Get("/users/{id}", adminCtl.UserEdit)
		r.Post("/users/{id}", adminCtl.UserUpdate)
		r.Post("/users/{id}/verify", adminCtl.UserVerify)
		r.Post("/users/{id}/points", adminCtl.UserAdjustPoints)

		r.Get("/topics", adminCtl.Topics)
		r.Post("/topics/{id}/moderate", adminCtl.TopicModerate)

		r.Get("/categories", adminCtl.Categories)
		r.Post("/categories", adminCtl.CategorySave)
		r.Post("/categories/{id}/delete", adminCtl.CategoryDelete)

		r.Get("/verifications", adminCtl.Verifications)
		r.Get("/points", adminCtl.Points)
		r.Post("/points", adminCtl.PointSave)

		r.Get("/logs", adminCtl.Logs)
		r.Get("/settings", adminCtl.Settings)
		r.Post("/settings", adminCtl.SettingsSave)
	})

	// 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("404 Not Found"))
	})

	return r
}

func newAuthService(cfg *config.Config, db *gorm.DB) *service.AuthService {
	return service.NewAuthService(db, cfg)
}

// 静态资源由 main 中注入
var staticFS http.FileSystem

func SetStaticFS(fs http.FileSystem) {
	staticFS = fs
}

func staticFileHandler() http.Handler {
	if staticFS == nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(staticFS)
}

func initServices(db *gorm.DB, cfg *config.Config) *controller.Services {
	authSvc := service.NewAuthService(db, cfg)
	userSvc := service.NewUserService(db)
	catSvc := service.NewCategoryService(db)
	pointSvc := service.NewPointService(db)
	notifySvc := service.NewNotificationService(db)
	checkinSvc := service.NewCheckinService(db, pointSvc)
	topicSvc := service.NewTopicService(db, catSvc, pointSvc, notifySvc)
	replySvc := service.NewReplyService(db, pointSvc, notifySvc)
	msgSvc := service.NewMessageService(db, notifySvc)
	qqOAuthSvc := service.NewQQOAuthService(db, cfg)
	searchSvc := service.NewSearchService(db)

	return &controller.Services{
		Auth:     authSvc,
		User:     userSvc,
		Topic:    topicSvc,
		Reply:    replySvc,
		Category: catSvc,
		Message:  msgSvc,
		Notify:   notifySvc,
		Point:    pointSvc,
		Checkin:  checkinSvc,
		QQOAuth:  qqOAuthSvc,
		Search:   searchSvc,
	}
}
