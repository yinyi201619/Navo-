package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func main() {
	base := "http://localhost:8080"
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	test := func(name, method, path string, data url.Values, cookies []*http.Cookie) (int, []*http.Cookie, string) {
		var req *http.Request
		if method == "POST" {
			req, _ = http.NewRequest(method, base+path, strings.NewReader(data.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req, _ = http.NewRequest(method, base+path, nil)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ❌ %s: ERROR %v\n", name, err)
			return 0, nil, ""
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		status := resp.StatusCode
		mark := "✅"
		if status >= 400 {
			mark = "❌"
		}
		fmt.Printf("  %s %s: %d\n", mark, name, status)
		return status, resp.Cookies(), string(body)
	}

	fmt.Println("=== 注册测试 ===")
	test("注册页面", "GET", "/register", nil, nil)
	test("注册提交", "POST", "/register", url.Values{
		"username": {"testuser"},
		"password": {"test1234"},
		"email":    {"test@test.com"},
	}, nil)

	fmt.Println("\n=== 登录测试 ===")
	test("登录页面", "GET", "/login", nil, nil)
	_, loginCookies, _ := test("普通用户登录", "POST", "/login", url.Values{
		"username": {"testuser"},
		"password": {"test1234"},
	}, nil)

	fmt.Println("\n=== 登录后页面 ===")
	test("设置页", "GET", "/settings", nil, loginCookies)
	test("安全设置", "GET", "/settings/security", nil, loginCookies)
	test("消息列表", "GET", "/messages", nil, loginCookies)
	test("通知列表", "GET", "/notifications", nil, loginCookies)
	test("发帖页", "GET", "/t/new", nil, loginCookies)
	test("签到 API", "POST", "/api/checkin", nil, loginCookies)

	fmt.Println("\n=== 管理员登录 ===")
	_, adminCookies, _ := test("管理员登录", "POST", "/login", url.Values{
		"username": {"admin"},
		"password": {"admin@12345"},
	}, nil)

	fmt.Println("\n=== 管理后台页面 ===")
	test("仪表盘", "GET", "/admin/", nil, adminCookies)
	test("用户管理", "GET", "/admin/users", nil, adminCookies)
	test("帖子管理", "GET", "/admin/topics", nil, adminCookies)
	test("板块管理", "GET", "/admin/categories", nil, adminCookies)
	test("认证管理", "GET", "/admin/verifications", nil, adminCookies)
	test("积分规则", "GET", "/admin/points", nil, adminCookies)
	test("操作日志", "GET", "/admin/logs", nil, adminCookies)
	test("系统设置", "GET", "/admin/settings", nil, adminCookies)

	fmt.Println("\n=== 完成 ===")
}
