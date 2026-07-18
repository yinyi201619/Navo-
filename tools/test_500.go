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

	get := func(path string, cookies []*http.Cookie) (int, string) {
		req, _ := http.NewRequest("GET", base+path, nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err.Error()
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(body)
	}

	post := func(path string, data url.Values, cookies []*http.Cookie) (int, []*http.Cookie, string) {
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(data.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err.Error()
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, resp.Cookies(), string(body)
	}

	// 管理员登录
	_, adminCookies, _ := post("/login", url.Values{
		"username": {"admin"},
		"password": {"admin@12345"},
	}, nil)

	pages := []string{
		"/settings",
		"/admin/users",
		"/admin/categories",
		"/admin/points",
	}

	fmt.Println("=== 500 页面详情 ===")
	for _, p := range pages {
		code, body := get(p, adminCookies)
		if code == 500 {
			fmt.Printf("\n--- %s ---\n", p)
			lines := strings.Split(body, "\n")
			for _, l := range lines {
				if strings.Contains(l, "模板渲染失败") || strings.Contains(l, "error") || strings.Contains(l, "Error") {
					fmt.Printf("  %s\n", strings.TrimSpace(l))
				}
			}
		}
	}

	// 签到 API
	fmt.Println("\n--- 签到 API ---")
	_, userCookies, _ := post("/login", url.Values{
		"username": {"testuser"},
		"password": {"test1234"},
	}, nil)
	code, _, body := post("/api/checkin", url.Values{}, userCookies)
	fmt.Printf("  状态: %d\n", code)
	lines := strings.Split(body, "\n")
	for _, l := range lines {
		ll := strings.TrimSpace(l)
		if ll != "" {
			fmt.Printf("  %s\n", ll)
		}
	}
}
