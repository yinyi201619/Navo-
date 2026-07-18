// Package controller 控制器层
package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"navo-nt-forum/internal/config"
	"navo-nt-forum/internal/middleware"
	"navo-nt-forum/internal/model"
	"navo-nt-forum/internal/service"
	tpl "navo-nt-forum/internal/template"
)

// Base 控制器基类
type Base struct {
	Cfg      *config.Config
	DB       *gorm.DB
	Tpl      *tpl.Engine
	Services *Services
}

// Services 所有服务聚合
type Services struct {
	Auth     *service.AuthService
	User     *service.UserService
	Topic    *service.TopicService
	Reply    *service.ReplyService
	Category *service.CategoryService
	Message  *service.MessageService
	Notify   *service.NotificationService
	Point    *service.PointService
	Checkin  *service.CheckinService
	QQOAuth  *service.QQOAuthService
	Search   *service.SearchService
}

// NewBase 创建基础控制器
func NewBase(cfg *config.Config, db *gorm.DB, tplEngine *tpl.Engine, svcs *Services) *Base {
	return &Base{Cfg: cfg, DB: db, Tpl: tplEngine, Services: svcs}
}

// Render 渲染 HTML
func (b *Base) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["SiteName"] = b.Cfg.Site.Name
	data["SiteDesc"] = b.Cfg.Site.Description
	data["IsLogin"] = middleware.IsLogin(r)
	data["UserID"] = middleware.CurrentUserID(r)
	data["Username"] = middleware.CurrentUsername(r)
	data["IsAdmin"] = middleware.IsAdmin(r)
	data["CurrentPath"] = r.URL.Path
	data["QQLoginEnabled"] = b.Services.QQOAuth.Enabled()
	if middleware.IsLogin(r) {
		uid := middleware.CurrentUserID(r)
		data["UnreadNotify"] = b.Services.Notify.UnreadCount(uid)
		data["UnreadMessage"] = b.Services.Message.UnreadMessageCount(uid)
	}
	html, err := b.Tpl.Render(name, data)
	if err != nil {
		http.Error(w, "模板渲染失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// JSON 输出 JSON
func (b *Base) JSON(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
		"data":    data,
	})
}

func (b *Base) JSONSuccess(w http.ResponseWriter, data interface{}) {
	b.JSON(w, 0, "success", data)
}

func (b *Base) JSONError(w http.ResponseWriter, httpStatus, code int, message string) {
	w.WriteHeader(httpStatus)
	b.JSON(w, code, message, nil)
}

// Page 分页参数
type Page struct {
	Page int
	Size int
}

func getPage(r *http.Request, defaultSize int) Page {
	p := Page{Page: 1, Size: defaultSize}
	if pg := r.URL.Query().Get("page"); pg != "" {
		if n, err := strconv.Atoi(pg); err == nil && n > 0 {
			p.Page = n
		}
	}
	if sz := r.URL.Query().Get("size"); sz != "" {
		if n, err := strconv.Atoi(sz); err == nil && n > 0 && n <= 100 {
			p.Size = n
		}
	}
	return p
}

func totalPages(total int64, size int) int {
	if total == 0 {
		return 0
	}
	p := int(total) / size
	if int(total)%size != 0 {
		p++
	}
	return p
}

func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return r.RemoteAddr
}

func acceptHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func makeUserToken(u *model.User, cfg *config.Config) (string, error) {
	claims := jwt.MapClaims{
		"uid":      u.ID,
		"username": u.Username,
		"role":     string(u.Role),
		"iss":      cfg.JWT.Issuer,
		"exp":      time.Now().Add(cfg.JWT.Expire).Unix(),
		"iat":      time.Now().Unix(),
		"jti":      uuid.NewString(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7 * 24 * 3600,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// ============================================================
// HomeController
// ============================================================

type HomeController struct{ *Base }

func NewHomeController(b *Base) *HomeController { return &HomeController{Base: b} }

func (c *HomeController) Home(w http.ResponseWriter, r *http.Request) {
	page := getPage(r, c.Cfg.Site.PerPage)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "latest"
	}
	topics, total, _ := c.Services.Topic.List(page.Page, page.Size, "", sort, "")
	categories, _ := c.Services.Category.All()

	var userCount, topicCount, replyCount int64
	c.DB.Model(&model.User{}).Count(&userCount)
	c.DB.Model(&model.Topic{}).Where("status = 1").Count(&topicCount)
	c.DB.Model(&model.Reply{}).Where("status = 1").Count(&replyCount)

	c.Render(w, r, "home.tmpl", map[string]interface{}{
		"Title":      "首页",
		"Active":     "home",
		"Topics":     topics,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Sort":       sort,
		"Categories": categories,
		"Stats": map[string]int64{
			"User":  userCount,
			"Topic": topicCount,
			"Reply": replyCount,
		},
	})
}

// ============================================================
// SearchController
// ============================================================

type SearchController struct{ *Base }

func NewSearchController(b *Base) *SearchController { return &SearchController{Base: b} }

func (c *SearchController) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "topic"
	}
	page := getPage(r, c.Cfg.Site.PerPage)

	data := map[string]interface{}{
		"Title":       "搜索",
		"Keyword":     q,
		"Type":        typ,
		"Page":        page.Page,
		"TotalPages":  0,
		"Total":       0,
		"Results":     nil,
		"HotSearches": []string{"Navo NT", "QQ Bot", "机器人", "插件", "教程"},
	}

	if q != "" {
		result, _ := c.Services.Search.Search(q, page.Page, page.Size)
		switch typ {
		case "user":
			data["Results"] = result.Users
			data["Total"] = result.UserCnt
			data["TotalPages"] = totalPages(result.UserCnt, page.Size)
		case "category":
			data["Results"] = result.Categories
			data["Total"] = result.CategoryCnt
			data["TotalPages"] = totalPages(result.CategoryCnt, page.Size)
		default:
			data["Results"] = result.Topics
			data["Total"] = result.TopicCnt
			data["TotalPages"] = totalPages(result.TopicCnt, page.Size)
		}
	}

	c.Render(w, r, "search.tmpl", data)
}

// ============================================================
// AuthController
// ============================================================

type AuthController struct{ *Base }

func NewAuthController(b *Base) *AuthController { return &AuthController{Base: b} }

func (c *AuthController) LoginPage(w http.ResponseWriter, r *http.Request) {
	if middleware.IsLogin(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	c.Render(w, r, "login.tmpl", map[string]interface{}{
		"Title":    "登录",
		"Active":   "login",
		"Redirect": r.URL.Query().Get("redirect"),
	})
}

func (c *AuthController) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	user, token, err := c.Services.Auth.Login(username, password, realIP(r))
	if err != nil {
		if acceptHTML(r) {
			c.Render(w, r, "login.tmpl", map[string]interface{}{
				"Title":    "登录",
				"Error":    err.Error(),
				"Redirect": redirect,
			})
			return
		}
		c.JSONError(w, 401, 401, err.Error())
		return
	}
	setAuthCookie(w, token)
	if redirect == "" {
		redirect = "/"
	}
	if acceptHTML(r) {
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	c.JSONSuccess(w, map[string]interface{}{"token": token, "user": user})
}

func (c *AuthController) RegisterPage(w http.ResponseWriter, r *http.Request) {
	if middleware.IsLogin(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !c.Cfg.Site.AllowRegister {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "注册已关闭",
			"Message": "当前站点已关闭注册。",
		})
		return
	}
	c.Render(w, r, "register.tmpl", map[string]interface{}{
		"Title":  "注册",
		"Active": "register",
	})
}

func (c *AuthController) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	if !c.Cfg.Site.AllowRegister {
		c.JSONError(w, 403, 403, "注册已关闭")
		return
	}
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	user, err := c.Services.Auth.Register(username, email, password, realIP(r))
	if err != nil {
		if acceptHTML(r) {
			c.Render(w, r, "register.tmpl", map[string]interface{}{
				"Title": "注册",
				"Error": err.Error(),
			})
			return
		}
		c.JSONError(w, 400, 400, err.Error())
		return
	}
	token, _ := makeUserToken(user, c.Cfg)
	setAuthCookie(w, token)
	if acceptHTML(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	c.JSONSuccess(w, map[string]interface{}{"user": user})
}

func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// QQLogin 跳转到 QQ 授权页
func (c *AuthController) QQLogin(w http.ResponseWriter, r *http.Request) {
	if !c.Services.QQOAuth.Enabled() {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "QQ 登录未启用",
			"Message": "管理员尚未配置 QQ 快捷登录。",
		})
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	redirectURI := scheme + "://" + r.Host + c.Cfg.QQOAuth.Callback
	authURL, _, err := c.Services.QQOAuth.AuthorizeURL(redirectURI)
	if err != nil {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "登录失败",
			"Message": err.Error(),
		})
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

// QQCallback QQ 授权回调
func (c *AuthController) QQCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	redirectURI := scheme + "://" + r.Host + c.Cfg.QQOAuth.Callback

	// 验证 state（可选，能取到就用）
	if state != "" {
		c.Services.QQOAuth.ConsumeState(state)
	}

	user, _, err := c.Services.QQOAuth.Callback(code, state, redirectURI)
	if err != nil {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "QQ 登录失败",
			"Message": err.Error(),
		})
		return
	}
	token, _ := makeUserToken(user, c.Cfg)
	setAuthCookie(w, token)
	http.Redirect(w, r, "/", http.StatusFound)
}

// ============================================================
// CategoryController
// ============================================================

type CategoryController struct{ *Base }

func NewCategoryController(b *Base) *CategoryController { return &CategoryController{Base: b} }

func (c *CategoryController) List(w http.ResponseWriter, r *http.Request) {
	categories, _ := c.Services.Category.All()
	hotTopics, _, _ := c.Services.Topic.List(1, 5, "", "hot", "")
	c.Render(w, r, "categories.tmpl", map[string]interface{}{
		"Title":      "板块",
		"Active":     "categories",
		"Categories": categories,
		"HotTopics":  hotTopics,
	})
}

func (c *CategoryController) Detail(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := c.Services.Category.BySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page := getPage(r, c.Cfg.Site.PerPage)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "latest"
	}
	topics, total, _ := c.Services.Topic.List(page.Page, page.Size, cat.ID, sort, "")
	c.Render(w, r, "category.tmpl", map[string]interface{}{
		"Title":      cat.Name,
		"Active":     "categories",
		"Category":   cat,
		"Topics":     topics,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Sort":       sort,
	})
}

// ============================================================
// TopicController
// ============================================================

type TopicController struct{ *Base }

func NewTopicController(b *Base) *TopicController { return &TopicController{Base: b} }

func (c *TopicController) Detail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	topic, err := c.Services.Topic.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	go c.Services.Topic.IncView(id)

	page := getPage(r, 30)
	replies, replyTotal, _ := c.Services.Reply.ListByTopic(id, page.Page, page.Size)

	hasLiked := false
	hasFav := false
	if middleware.IsLogin(r) {
		uid := middleware.CurrentUserID(r)
		hasLiked = c.Services.Topic.HasLiked(uid, id)
		hasFav = c.Services.Topic.HasFavorited(uid, id)
	}
	authorBrief := c.Services.User.ToBrief(topic.Author)

	c.Render(w, r, "topic.tmpl", map[string]interface{}{
		"Title":      topic.Title,
		"Topic":      topic,
		"Author":     authorBrief,
		"Replies":    replies,
		"ReplyTotal": replyTotal,
		"ReplyPages": totalPages(replyTotal, page.Size),
		"Page":       page.Page,
		"HasLiked":   hasLiked,
		"HasFav":     hasFav,
	})
}

func (c *TopicController) NewPage(w http.ResponseWriter, r *http.Request) {
	cats, _ := c.Services.Category.All()
	c.Render(w, r, "topic_new.tmpl", map[string]interface{}{
		"Title":      "发帖",
		"Active":     "new",
		"Categories": cats,
	})
}

func (c *TopicController) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	categoryID := r.FormValue("category_id")
	title := strings.TrimSpace(r.FormValue("title"))
	content := r.FormValue("content")
	tags := r.FormValue("tags")

	if title == "" || content == "" {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "错误",
			"Message": "标题和内容不能为空",
		})
		return
	}
	uid := middleware.CurrentUserID(r)
	topic, err := c.Services.Topic.Create(uid, categoryID, title, content, tags)
	if err != nil {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "发帖失败",
			"Message": err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/t/"+topic.ID, http.StatusFound)
}

func (c *TopicController) Reply(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	content := strings.TrimSpace(r.FormValue("content"))
	parentID := r.FormValue("parent_id")
	uid := middleware.CurrentUserID(r)
	_, err := c.Services.Reply.Create(uid, id, parentID, content)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/t/"+id, http.StatusFound)
}

// ============================================================
// APIController 异步接口
// ============================================================

type APIController struct{ *Base }

func NewAPIController(b *Base) *APIController { return &APIController{Base: b} }

func (c *APIController) LikeTopic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := middleware.CurrentUserID(r)
	liked, cnt, err := c.Services.Topic.ToggleLike(uid, id)
	if err != nil {
		c.JSONError(w, 500, 500, err.Error())
		return
	}
	c.JSONSuccess(w, map[string]interface{}{"liked": liked, "count": cnt})
}

func (c *APIController) FavoriteTopic(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := middleware.CurrentUserID(r)
	fav, err := c.Services.Topic.ToggleFavorite(uid, id)
	if err != nil {
		c.JSONError(w, 500, 500, err.Error())
		return
	}
	c.JSONSuccess(w, map[string]interface{}{"favorited": fav})
}

func (c *APIController) LikeReply(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := middleware.CurrentUserID(r)
	liked, cnt, err := c.Services.Reply.ToggleLike(uid, id)
	if err != nil {
		c.JSONError(w, 500, 500, err.Error())
		return
	}
	c.JSONSuccess(w, map[string]interface{}{"liked": liked, "count": cnt})
}

func (c *APIController) Checkin(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	continuous, points, err := c.Services.Checkin.Checkin(uid)
	if err != nil {
		if err == service.ErrAlreadyCheckedIn {
			c.JSONError(w, 400, 400, "今日已签到")
			return
		}
		c.JSONError(w, 500, 500, err.Error())
		return
	}
	c.JSONSuccess(w, map[string]interface{}{
		"continuous": continuous,
		"points":     points,
	})
}

// ============================================================
// UserController
// ============================================================

type UserController struct{ *Base }

func NewUserController(b *Base) *UserController { return &UserController{Base: b} }

func (c *UserController) Profile(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := c.Services.User.GetByUsername(username)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	brief := c.Services.User.ToBrief(user)
	levelInfo := model.GetLevelInfo(user.Experience)

	topics, topicTotal, _ := c.Services.User.UserTopics(user.ID, 1, 10)
	replies, replyTotal, _ := c.Services.User.UserReplies(user.ID, 1, 10)

	verifications, _ := c.Services.User.VerificationsOfUser(user.ID)
	isSelf := middleware.CurrentUserID(r) == user.ID

	c.Render(w, r, "user.tmpl", map[string]interface{}{
		"Title":         user.Username,
		"User":          user,
		"Brief":         brief,
		"LevelInfo":     levelInfo,
		"Topics":        topics,
		"TopicTotal":    topicTotal,
		"Replies":       replies,
		"ReplyTotal":    replyTotal,
		"Verifications": verifications,
		"IsSelf":        isSelf,
	})
}

func (c *UserController) UserTopics(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := c.Services.User.GetByUsername(username)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page := getPage(r, c.Cfg.Site.PerPage)
	topics, total, _ := c.Services.User.UserTopics(user.ID, page.Page, page.Size)
	c.Render(w, r, "user_topics.tmpl", map[string]interface{}{
		"Title":      user.Username + "的主题",
		"User":       user,
		"Topics":     topics,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Tab":        "topics",
	})
}

func (c *UserController) SettingsPage(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	user, _ := c.Services.User.GetByID(uid)
	binding, _ := c.Services.QQOAuth.GetBinding(uid)
	c.Render(w, r, "settings.tmpl", map[string]interface{}{
		"Title":   "个人设置",
		"User":    user,
		"Binding": binding,
		"Tab":     "profile",
	})
}

func (c *UserController) SettingsSave(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	r.ParseForm()
	avatar := r.FormValue("avatar")
	signature := r.FormValue("signature")
	c.Services.User.UpdateProfile(uid, avatar, signature)
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (c *UserController) SecurityPage(w http.ResponseWriter, r *http.Request) {
	c.Render(w, r, "security.tmpl", map[string]interface{}{
		"Title": "安全设置",
		"Tab":   "security",
	})
}

func (c *UserController) ChangePassword(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	r.ParseForm()
	oldPw := r.FormValue("old_password")
	newPw := r.FormValue("new_password")
	if err := c.Services.User.ChangePassword(uid, oldPw, newPw); err != nil {
		c.Render(w, r, "message.tmpl", map[string]interface{}{
			"Title":   "修改失败",
			"Message": err.Error(),
		})
		return
	}
	c.Render(w, r, "message.tmpl", map[string]interface{}{
		"Title":   "修改成功",
		"Message": "密码已更新，请重新登录。",
		"Link":    "/logout",
	})
}

// ============================================================
// MessageController
// ============================================================

type MessageController struct{ *Base }

func NewMessageController(b *Base) *MessageController { return &MessageController{Base: b} }

func (c *MessageController) Index(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	page := getPage(r, 20)
	convs, total, _ := c.Services.Message.Conversations(uid, page.Page, page.Size)
	c.Render(w, r, "messages.tmpl", map[string]interface{}{
		"Title":        "私信",
		"Active":       "messages",
		"Conversations": convs,
		"Total":        total,
		"CurrentUserID": uid,
	})
}

func (c *MessageController) Conversation(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	convID := chi.URLParam(r, "id")
	conv, err := c.Services.Message.GetConversation(uid, convID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// 标记已读
	c.Services.Message.MarkRead(convID, uid)

	page := getPage(r, 50)
	messages, total, _ := c.Services.Message.Messages(convID, uid, page.Page, page.Size)

	var otherUser *model.User
	if conv.UserAID == uid {
		otherUser = conv.UserB
	} else {
		otherUser = conv.UserA
	}
	otherBrief := c.Services.User.ToBrief(otherUser)

	convs, _, _ := c.Services.Message.Conversations(uid, 1, 20)

	c.Render(w, r, "message_detail.tmpl", map[string]interface{}{
		"Title":         "与" + otherUser.Username + "的对话",
		"Active":        "messages",
		"Conversations": convs,
		"Conv":          conv,
		"OtherUser":     otherUser,
		"OtherBrief":    otherBrief,
		"Messages":      messages,
		"MsgTotal":      total,
		"CurrentUserID": uid,
	})
}

func (c *MessageController) Send(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	convID := chi.URLParam(r, "id")
	r.ParseForm()
	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Redirect(w, r, "/messages/"+convID, http.StatusFound)
		return
	}
	_, err := c.Services.Message.SendMessage(uid, convID, content)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/messages/"+convID, http.StatusFound)
}

func (c *MessageController) NewConversation(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	to := r.URL.Query().Get("to")
	if to == "" {
		http.Redirect(w, r, "/messages", http.StatusFound)
		return
	}
	target, err := c.Services.User.GetByUsername(to)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if target.ID == uid {
		http.Redirect(w, r, "/messages", http.StatusFound)
		return
	}
	conv, err := c.Services.Message.GetOrCreateConversation(uid, target.ID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/messages/"+conv.ID, http.StatusFound)
}

// ============================================================
// NotificationController
// ============================================================

type NotificationController struct{ *Base }

func NewNotificationController(b *Base) *NotificationController {
	return &NotificationController{Base: b}
}

func (c *NotificationController) Index(w http.ResponseWriter, r *http.Request) {
	uid := middleware.CurrentUserID(r)
	page := getPage(r, 20)
	unreadOnly := r.URL.Query().Get("filter") == "unread"
	list, total, _ := c.Services.Notify.List(uid, page.Page, page.Size, unreadOnly)
	c.Services.Notify.MarkAllRead(uid)
	c.Render(w, r, "notifications.tmpl", map[string]interface{}{
		"Title":      "通知",
		"Active":     "notifications",
		"List":       list,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Filter":     r.URL.Query().Get("filter"),
	})
}
