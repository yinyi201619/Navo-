package navo

import "embed"

// WebFS 嵌入的前端资源（模板 + 静态文件）
//
//go:embed all:web
var WebFS embed.FS
