// Package middleware HTTP 中间件
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"navo-nt-forum/internal/service"
)

type contextKey string

const (
	CtxUserID    contextKey = "user_id"
	CtxUsername  contextKey = "username"
	CtxUserRole  contextKey = "user_role"
	CtxIsLogin   contextKey = "is_login"
)

// Auth 鉴权中间件：从 Cookie / Authorization header 解析 JWT
func Auth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string
			// 1. Cookie
			if ck, err := r.Cookie("token"); err == nil && ck.Value != "" {
				tokenStr = ck.Value
			}
			// 2. Header
			if tokenStr == "" {
				ah := r.Header.Get("Authorization")
				if strings.HasPrefix(ah, "Bearer ") {
					tokenStr = strings.TrimPrefix(ah, "Bearer ")
				}
			}
			if tokenStr != "" {
				claims, err := authSvc.ParseToken(tokenStr)
				if err == nil {
					ctx := r.Context()
					ctx = context.WithValue(ctx, CtxUserID, claims.UID)
					ctx = context.WithValue(ctx, CtxUsername, claims.Username)
					ctx = context.WithValue(ctx, CtxUserRole, claims.Role)
					ctx = context.WithValue(ctx, CtxIsLogin, true)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireLogin 必须登录
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLogin(r) {
			// HTML 请求重定向到登录页
			if acceptHTML(r) {
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusFound)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"code":    401,
				"message": "请先登录",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin 必须管理员
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLogin(r) {
			if acceptHTML(r) {
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusFound)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"code": 401, "message": "请先登录"})
			return
		}
		role, _ := r.Context().Value(CtxUserRole).(string)
		if role != "admin" {
			if acceptHTML(r) {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("无权访问"))
				return
			}
			writeJSON(w, http.StatusForbidden, map[string]interface{}{"code": 403, "message": "无权访问"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Recovery panic 恢复
func Recovery(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("panic recovered",
						zap.Any("error", err),
						zap.String("path", r.URL.Path),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog 访问日志
func AccessLog(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(lrw, r)
			log.Info("access",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", lrw.status),
				zap.Duration("duration", time.Since(start)),
				zap.String("ip", realIP(r)),
			)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.status = code
	l.ResponseWriter.WriteHeader(code)
}

// NoCache 禁用缓存
func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		next.ServeHTTP(w, r)
	})
}

// ===== 辅助函数 =====

func IsLogin(r *http.Request) bool {
	v, ok := r.Context().Value(CtxIsLogin).(bool)
	return ok && v
}

func CurrentUserID(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserID).(string)
	return v
}

func CurrentUsername(r *http.Request) string {
	v, _ := r.Context().Value(CtxUsername).(string)
	return v
}

func CurrentRole(r *http.Request) string {
	v, _ := r.Context().Value(CtxUserRole).(string)
	return v
}

func IsAdmin(r *http.Request) bool {
	return CurrentRole(r) == "admin"
}

func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func acceptHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
