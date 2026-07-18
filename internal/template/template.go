// Package template 模板渲染封装
package template

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"navo-nt-forum/internal/model"
	"navo-nt-forum/internal/service"
)

type OSFS struct{}

func (OSFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (OSFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

func (OSFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

type FS interface {
	fs.FS
	fs.ReadDirFS
}

// Engine 模板引擎
type Engine struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
	siteName  string
	siteDesc  string
	userSvc   *service.UserService
}

// NewEngine 创建模板引擎
func NewEngine(tplFS FS, root string, siteName, siteDesc string, userSvc *service.UserService) (*Engine, error) {
	eng := &Engine{
		templates: make(map[string]*template.Template),
		siteName:  siteName,
		siteDesc:  siteDesc,
		userSvc:   userSvc,
	}
	eng.funcMap = template.FuncMap{
		"add":          add,
		"sub":          sub,
		"mul":          mul,
		"div":          div,
		"truncate":     truncate,
		"formatTime":   formatTime,
		"relativeTime": relativeTime,
		"safeHTML":     safeHTML,
		"levelInfo":    levelInfo,
		"levelColor":   levelColor,
		"md":           service.RenderMarkdown,
		"seq":          seq,
		"userAvatar":   userAvatar,
		"statusClass":  statusClass,
		"roleClass":    roleClass,
		"statusLabel":  statusLabel,
		"roleLabel":    roleLabel,
		"highlight":    highlight,
		"formatDateTime": formatDateTime,
		"addf":           addf,
		"mulf":           mulf,
	}

	layouts, err := fs.Glob(tplFS, filepath.Join(root, "layout/*.tmpl"))
	if err != nil {
		return nil, err
	}
	partials, err := fs.Glob(tplFS, filepath.Join(root, "partial/*.tmpl"))
	if err != nil {
		return nil, err
	}

	pages, err := fs.Glob(tplFS, filepath.Join(root, "*.tmpl"))
	if err != nil {
		return nil, err
	}
	adminPages, _ := fs.Glob(tplFS, filepath.Join(root, "admin/*.tmpl"))
	pages = append(pages, adminPages...)

	for _, page := range pages {
		name := strings.TrimPrefix(page, root+"/")
		files := append([]string{page}, layouts...)
		files = append(files, partials...)
		t := template.New(name).Funcs(eng.funcMap)
		t, err := t.ParseFS(tplFS, files...)
		if err != nil {
			return nil, err
		}
		eng.templates[name] = t
	}
	return eng, nil
}

// Render 渲染模板
func (e *Engine) Render(name string, data map[string]interface{}) (string, error) {
	t, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("模板不存在: %s", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ===== 模板函数 =====

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

func add(a, b int) int { return a + b }
func sub(a, b int) int { return a - b }
func mul(a, b int) int { return a * b }
func div(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func addf(a, b interface{}) float64 { return toFloat(a) + toFloat(b) }
func mulf(a, b interface{}) float64 { return toFloat(a) * toFloat(b) }

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func formatTime(args ...interface{}) string {
	if len(args) == 0 {
		return "-"
	}
	t := toTime(args[0])
	if t.IsZero() {
		return "-"
	}
	layout := "2006-01-02 15:04"
	if len(args) >= 2 {
		if s, ok := args[1].(string); ok {
			layout = s
		}
	}
	return t.Format(layout)
}

func toTime(v interface{}) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case *time.Time:
		if val == nil {
			return time.Time{}
		}
		return *val
	default:
		return time.Time{}
	}
}

func relativeTime(t interface{}) string {
	tt := toTime(t)
	if tt.IsZero() {
		return "-"
	}
	d := time.Since(tt)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return intStr(int(d.Minutes())) + " 分钟前"
	case d < 24*time.Hour:
		return intStr(int(d.Hours())) + " 小时前"
	case d < 30*24*time.Hour:
		return intStr(int(d.Hours()/24)) + " 天前"
	default:
		return tt.Format("2006-01-02")
	}
}

func intStr(i int) string {
	return strconv.Itoa(i)
}

func seq(start, end int) []int {
	var s []int
	for i := start; i <= end; i++ {
		s = append(s, i)
	}
	return s
}

func userAvatar(username string) string {
	if username == "" {
		return "https://api.dicebear.com/7.x/avataaars/svg?seed=user"
	}
	return "https://api.dicebear.com/7.x/avataaars/svg?seed=" + username
}

func toInt8(v interface{}) int8 {
	switch val := v.(type) {
	case int8:
		return val
	case int:
		return int8(val)
	case int64:
		return int8(val)
	default:
		return 0
	}
}

func statusClass(s interface{}) string {
	v := toInt8(s)
	switch v {
	case 1:
		return "active"
	case 0:
		return "muted"
	case -1:
		return "banned"
	}
	return "muted"
}

func statusLabel(s interface{}) string {
	v := toInt8(s)
	switch v {
	case 1:
		return "正常"
	case 0:
		return "禁言"
	case -1:
		return "封禁"
	}
	return "未知"
}

func roleClass(r string) string {
	switch r {
	case "admin":
		return "role-admin"
	case "moderator":
		return "role-moderator"
	default:
		return "role-user"
	}
}

func roleLabel(r string) string {
	switch r {
	case "admin":
		return "管理员"
	case "moderator":
		return "版主"
	default:
		return "用户"
	}
}

func highlight(text, keyword string) template.HTML {
	if keyword == "" {
		return template.HTML(template.HTMLEscapeString(text))
	}
	escaped := template.HTMLEscapeString(text)
	escapedKw := template.HTMLEscapeString(keyword)
	highlighted := strings.ReplaceAll(escaped, escapedKw, `<mark style="background:rgba(250, 204, 21, 0.4);padding:0 2px;border-radius:3px;">`+escapedKw+`</mark>`)
	return template.HTML(highlighted)
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04")
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

func levelInfo(exp int) model.LevelInfo {
	return model.GetLevelInfo(exp)
}

func levelColor(level int) string {
	for _, t := range model.LevelThresholds {
		if t.Level == level {
			return t.Color
		}
	}
	return "#9CA3AF"
}
