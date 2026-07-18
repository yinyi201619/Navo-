# Navo NT QQ BOT 论坛

> 众合会自研论坛 · Go 语言实现 · 借鉴 bbsgo 架构思想 · ARM64 架构部署

## ✨ 特性

- 🎨 **毛玻璃简约风格** — Glassmorphism 设计 + 圆润边框 + 蓝紫渐变主色
- 👤 **用户系统** — 注册/登录 / JWT 认证 / QQ 快捷登录
- 🏆 **等级积分制** — 7 级成长体系、签到奖励、每日积分上限
- ✅ **官方认证** — 官方认证 / 优质贡献者 / 机器人认证三种标识
- 💬 **私信系统** — 用户间一对一私信、未读提醒
- 📝 **完整论坛** — 板块、帖子、回复、楼中楼、点赞、收藏、搜索
- 🛠 **管理后台** — 仪表盘、用户管理、内容审核、认证管理、积分规则、操作日志
- ⚡ **高性能** — Go 原生 + GORM + 服务端渲染，单二进制部署
- 📱 **响应式** — 桌面 / 平板 / 移动端自适应
- 🔧 **ARM64 原生** — 纯 Go 实现，`CGO_ENABLED=0`，树莓派/ARM 服务器直接运行

## 🏗 技术栈

| 层 | 选型 |
|----|------|
| 语言 | Go 1.22+ |
| 路由 | go-chi/chi v5 |
| ORM | GORM v2 |
| 数据库 | MySQL 8.0 / SQLite（默认） |
| 认证 | JWT + HttpOnly Cookie |
| 模板 | Go html/template（服务端渲染） |
| 样式 | 手写 CSS + CSS 变量（毛玻璃风格） |
| 日志 | zap |
| 配置 | viper + YAML |
| Markdown | goldmark（GFM 兼容） |
| 部署 | 单二进制 / Docker / ARM64 |

## 🚀 快速开始

### 环境要求

- Go 1.22+
- SQLite（默认，零配置）或 MySQL 8.0

### 本地运行

```bash
# 克隆后
go mod tidy
make run
```

访问 http://localhost:8080

默认管理员账号：
- 用户名：`admin`
- 密码：`admin@12345`

### ARM64 构建

```bash
# 在 x86_64 机器上交叉编译
make build-arm64

# 产物：bin/navo-forum-arm64
```

### Docker

```bash
# 构建
docker build -t navo-forum .

# 运行
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  --name navo-forum \
  navo-forum
```

### ARM64 Docker

```bash
docker buildx build --platform linux/arm64 -t navo-forum:arm64 --load .
```

## ⚙️ 配置

编辑 `configs/config.yaml`：

```yaml
server:
  port: 8080

database:
  driver: sqlite       # mysql / sqlite
  dsn: "data/navo.db"  # sqlite 文件路径 / mysql dsn

jwt:
  secret: "change-me"

# QQ 快捷登录（可选）
qq_oauth:
  enabled: false
  app_id: "你的AppID"
  app_key: "你的AppKey"
  callback: "/auth/qq/callback"
```

## 📁 项目结构

```
navo-nt-forum/
├── cmd/server/main.go        # 程序入口
├── internal/
│   ├── config/               # 配置加载
│   ├── model/                # 数据模型
│   ├── database/             # 数据库初始化
│   ├── service/              # 业务服务层
│   ├── repository/           # 数据仓储层
│   ├── controller/           # 控制器
│   ├── middleware/           # 中间件
│   ├── router/               # 路由注册
│   └── template/             # 模板渲染封装
├── web/
│   ├── templates/            # html/template 模板
│   └── static/               # 静态资源（CSS/JS/Img）
├── configs/config.yaml       # 配置文件
├── embed.go                  # embed 打包
├── Makefile
├── Dockerfile
└── go.mod
```

## 🧩 功能模块

### 前台
- 首页（Hero + 板块导航 + 最新/热门/精华帖子）
- 板块列表 / 板块详情
- 帖子详情（Markdown 渲染、点赞、收藏、回复）
- 发帖（Markdown 编辑器）
- 用户主页（等级、积分、认证、主题/回复/收藏）
- 私信（会话列表、聊天界面）
- 通知中心
- 个人设置（资料、安全、QQ绑定）
- 搜索（帖子 + 用户）
- QQ 快捷登录

### 等级体系
| 等级 | 经验 | 颜色 |
|------|------|------|
| Lv1 新手 | 0 | 灰色 |
| Lv2 学徒 | 100 | 绿色 |
| Lv3 行家 | 500 | 蓝色 |
| Lv4 专家 | 2000 | 紫色 |
| Lv5 大师 | 8000 | 橙色 |
| Lv6 宗师 | 30000 | 红色 |
| Lv7 传说 | 100000 | 金色 |

### 管理后台
- 仪表盘（统计 + 趋势）
- 用户管理（搜索、禁言/封禁、角色变更、认证、积分调整）
- 帖子管理（置顶、加精、删除、恢复）
- 板块管理（增删改、排序）
- 认证管理（列表查询）
- 积分规则管理
- 操作日志
- 系统设置

## 🔐 安全

- 密码 bcrypt 哈希
- JWT + HttpOnly Cookie
- CSRF（表单提交）
- XSS 防护（模板自动转义 + Markdown 白名单）
- SQL 注入防护（GORM 参数化查询）
- 管理员操作全量审计

## 📜 License

MIT
