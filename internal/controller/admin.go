package controller

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"navo-nt-forum/internal/middleware"
	"navo-nt-forum/internal/model"
)

// ============================================================
// AdminController 后台管理
// ============================================================

type AdminController struct{ *Base }

func NewAdminController(b *Base) *AdminController { return &AdminController{Base: b} }

func (c *AdminController) Dashboard(w http.ResponseWriter, r *http.Request) {
	var userCount, topicCount, replyCount, todayTopics int64
	c.DB.Model(&model.User{}).Count(&userCount)
	c.DB.Model(&model.Topic{}).Where("status = 1").Count(&topicCount)
	c.DB.Model(&model.Reply{}).Where("status = 1").Count(&replyCount)

	// 近 7 天趋势
	type dailyRow struct {
		Date  string
		Count int64
	}
	var topicDaily []dailyRow
	c.DB.Model(&model.Topic{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= DATE('now', '-6 days')").
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&topicDaily)

	var userDaily []dailyRow
	c.DB.Model(&model.User{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= DATE('now', '-6 days')").
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&userDaily)

	c.Render(w, r, "admin/dashboard.tmpl", map[string]interface{}{
		"Title":      "仪表盘",
		"Active":     "dashboard",
		"UserCount":  userCount,
		"TopicCount": topicCount,
		"ReplyCount": replyCount,
		"TodayTopics": todayTopics,
		"TopicDaily": topicDaily,
		"UserDaily":  userDaily,
	})
}

// ===== 用户管理 =====

func (c *AdminController) Users(w http.ResponseWriter, r *http.Request) {
	page := getPage(r, 20)
	keyword := r.URL.Query().Get("keyword")
	users, total, _ := c.Services.User.List(page.Page, page.Size, keyword)

	// 批量获取认证
	userIDs := make([]string, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	var verifications []model.UserVerification
	c.DB.Where("user_id IN ? AND revoked_at IS NULL", userIDs).Find(&verifications)
	verifyMap := make(map[string][]model.UserVerification)
	for _, v := range verifications {
		verifyMap[v.UserID] = append(verifyMap[v.UserID], v)
	}

	c.Render(w, r, "admin/users.tmpl", map[string]interface{}{
		"Title":      "用户管理",
		"Active":     "users",
		"Users":      users,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Keyword":    keyword,
		"VerifyMap":  verifyMap,
	})
}

func (c *AdminController) UserEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := c.Services.User.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	verifications, _ := c.Services.User.VerificationsOfUser(id)
	c.Render(w, r, "admin/user_edit.tmpl", map[string]interface{}{
		"Title":         "编辑用户",
		"Active":        "users",
		"User":          user,
		"Verifications": verifications,
	})
}

func (c *AdminController) UserUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	avatar := r.FormValue("avatar")
	signature := r.FormValue("signature")
	role := r.FormValue("role")
	statusStr := r.FormValue("status")

	c.Services.User.UpdateProfile(id, avatar, signature)
	if role != "" {
		c.Services.User.SetRole(id, model.Role(role))
	}
	if statusStr != "" {
		st, _ := strconv.Atoi(statusStr)
		c.Services.User.SetStatus(id, model.UserStatus(st))
	}
	http.Redirect(w, r, "/admin/users", http.StatusFound)
}

func (c *AdminController) UserVerify(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	action := r.FormValue("action") // grant/revoke
	vt := model.VerifyType(r.FormValue("verify_type"))
	label := r.FormValue("label")
	adminID := middleware.CurrentUserID(r)

	if action == "grant" {
		c.Services.User.GrantVerification(id, vt, label, adminID)
	} else if action == "revoke" {
		c.Services.User.RevokeVerification(id, vt)
	}
	http.Redirect(w, r, "/admin/users/"+id, http.StatusFound)
}

func (c *AdminController) UserAdjustPoints(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	pointsStr := r.FormValue("points")
	expStr := r.FormValue("experience")
	remark := r.FormValue("remark")
	points, _ := strconv.Atoi(pointsStr)
	exp, _ := strconv.Atoi(expStr)
	adminID := middleware.CurrentUserID(r)
	c.Services.User.AdjustPoints(id, points, exp, remark, adminID)
	http.Redirect(w, r, "/admin/users/"+id, http.StatusFound)
}

// ===== 帖子管理 =====

func (c *AdminController) Topics(w http.ResponseWriter, r *http.Request) {
	page := getPage(r, 20)
	keyword := r.URL.Query().Get("keyword")
	topics, total, _ := c.Services.Topic.AdminList(page.Page, page.Size, keyword)
	c.Render(w, r, "admin/topics.tmpl", map[string]interface{}{
		"Title":      "帖子管理",
		"Active":     "topics",
		"Topics":     topics,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
		"Keyword":    keyword,
	})
}

func (c *AdminController) TopicModerate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	action := r.FormValue("action")
	valStr := r.FormValue("value")
	val := valStr == "1" || valStr == "true"
	c.Services.Topic.Moderate(id, action, val)
	http.Redirect(w, r, "/admin/topics", http.StatusFound)
}

// ===== 板块管理 =====

func (c *AdminController) Categories(w http.ResponseWriter, r *http.Request) {
	cats, _ := c.Services.Category.All()
	c.Render(w, r, "admin/categories.tmpl", map[string]interface{}{
		"Title":      "板块管理",
		"Active":     "categories",
		"Categories": cats,
	})
}

func (c *AdminController) CategorySave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id := r.FormValue("id")
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	description := r.FormValue("description")
	icon := r.FormValue("icon")
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	if id != "" {
		c.Services.Category.Update(id, name, slug, description, icon, sortOrder)
	} else {
		c.Services.Category.Create(name, slug, description, icon, sortOrder)
	}
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

func (c *AdminController) CategoryDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c.Services.Category.Delete(id)
	http.Redirect(w, r, "/admin/categories", http.StatusFound)
}

// ===== 认证管理 =====

func (c *AdminController) Verifications(w http.ResponseWriter, r *http.Request) {
	page := getPage(r, 20)
	list, total, _ := c.Services.User.ListVerifications(page.Page, page.Size)
	c.Render(w, r, "admin/verifications.tmpl", map[string]interface{}{
		"Title":      "认证管理",
		"Active":     "verifications",
		"List":       list,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
	})
}

// ===== 积分管理 =====

func (c *AdminController) Points(w http.ResponseWriter, r *http.Request) {
	rules, _ := c.Services.Point.Rules()
	c.Render(w, r, "admin/points.tmpl", map[string]interface{}{
		"Title":  "积分规则",
		"Active": "points",
		"Rules":  rules,
	})
}

func (c *AdminController) PointSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	idStr := r.FormValue("id")
	points, _ := strconv.Atoi(r.FormValue("points"))
	exp, _ := strconv.Atoi(r.FormValue("experience"))
	dailyLimit, _ := strconv.Atoi(r.FormValue("daily_limit"))
	id, _ := strconv.Atoi(idStr)
	c.Services.Point.UpdateRule(id, points, exp, dailyLimit)
	http.Redirect(w, r, "/admin/points", http.StatusFound)
}

// ===== 操作日志 =====

func (c *AdminController) Logs(w http.ResponseWriter, r *http.Request) {
	page := getPage(r, 30)
	var logs []model.AdminLog
	var total int64
	q := c.DB.Model(&model.AdminLog{})
	q.Count(&total)
	q.Order("created_at DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&logs)
	c.Render(w, r, "admin/logs.tmpl", map[string]interface{}{
		"Title":      "操作日志",
		"Active":     "logs",
		"Logs":       logs,
		"Total":      total,
		"TotalPages": totalPages(total, page.Size),
		"Page":       page.Page,
	})
}

// ===== 系统设置 =====

func (c *AdminController) Settings(w http.ResponseWriter, r *http.Request) {
	var settings []model.Setting
	c.DB.Find(&settings)
	settingMap := make(map[string]string)
	for _, s := range settings {
		settingMap[s.Key] = s.Value
	}
	c.Render(w, r, "admin/settings.tmpl", map[string]interface{}{
		"Title":   "系统设置",
		"Active":  "settings",
		"SettingMap": settingMap,
	})
}

func (c *AdminController) SettingsSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	keys := []string{"site_name", "site_description", "allow_register", "upload_limit_mb"}
	for _, k := range keys {
		v := r.FormValue(k)
		c.DB.Model(&model.Setting{}).Where("`key` = ?", k).Update("value", v)
	}
	// 更新缓存中的配置
	c.Cfg.Site.Name = r.FormValue("site_name")
	c.Cfg.Site.Description = r.FormValue("site_description")
	c.Cfg.Site.AllowRegister = r.FormValue("allow_register") == "1"
	if mb, err := strconv.Atoi(r.FormValue("upload_limit_mb")); err == nil {
		c.Cfg.Site.UploadLimitMB = mb
	}
	http.Redirect(w, r, "/admin/settings", http.StatusFound)
}
