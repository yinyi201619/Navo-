package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	base := "http://localhost:8080"
	paths := []string{
		"/",
		"/login",
		"/register",
		"/search?q=test",
		"/categories",
		"/u/admin",
		"/static/css/style.css",
	}
	client := &http.Client{Timeout: 5 * time.Second}
	for _, p := range paths {
		resp, err := client.Get(base + p)
		if err != nil {
			fmt.Printf("  %-40s -> ERROR: %v\n", p, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			s := string(body)
			if len(s) > 400 {
				s = s[:400]
			}
			fmt.Printf("  %-40s -> %s\n    %s\n", p, resp.Status, s)
		} else {
			fmt.Printf("  %-40s -> %s (%d bytes)\n", p, resp.Status, len(body))
		}
	}
}
