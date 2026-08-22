# mockd 多语言汉化指南 / Internationalization (i18n) Guide

本分支在 mockd 基础上完成了 **CLI 多语言框架** 与 **内置 Web Dashboard 汉化**，支持英文 / 简体中文切换。
This branch adds a **CLI i18n framework** and **Chinese localization for the built-in Web Dashboard**, with English / Simplified Chinese switching support.

---

## 功能总览 / Feature Overview

| 模块 / Module | 能力 / Capability |
|---|---|
| CLI 命令与帮助 / CLI commands & help | 命令描述、flag 说明、cobra 模板全部支持运行时翻译 |
| 语言切换 / Language switch | 全局参数 `--lang`，支持 `en` / `zh-CN` |
| Web Dashboard | 内置管理面板（admin 端口 4290）已汉化为中文界面 |
| 翻译维护 / Translation maintenance | 语言包集中在 `pkg/i18n/locales/`，易于扩展新语言 |

---

## 快速使用 / Quick Start

### CLI 多语言切换 / CLI language switching

```bash
# 中文界面（命令描述、帮助、flag 说明均为中文）
mockd --lang zh-CN --help
mockd --lang zh-CN add http --help

# 英文界面（默认，保持上游行为）
mockd --lang en --help

# 直接使用
mockd --lang zh-CN start --no-auth
```

### 内置 Dashboard / Built-in Dashboard

使用 `-tags dashboard` 构建后，启动服务并访问 admin 端口（默认 `http://localhost:4290`）即可看到中文管理面板。

```bash
# 构建（默认 macOS/本机架构）
go build -tags dashboard -o mockd ./cmd/mockd

# 运行
./mockd start --no-auth
# 浏览器打开 http://localhost:4290
```

### 交叉编译（Linux 信创环境）/ Cross-compile for Linux

```bash
# linux/arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -tags dashboard -trimpath -ldflags="-s -w" -o mockd-linux-arm64 ./cmd/mockd

# linux/amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -tags dashboard -trimpath -ldflags="-s -w" -o mockd-linux-amd64 ./cmd/mockd
```

国内网络建议设置 `GOPROXY=https://goproxy.cn,direct`。

---

## 架构说明 / Architecture

### CLI i18n 框架 / CLI i18n framework

新增 `pkg/i18n` 包，核心文件：

```
pkg/i18n/
├── i18n.go          # 翻译核心：T() 运行时查表、--lang 全局参数注入
├── format.go        # 格式化辅助
└── locales/
    ├── en.json      # 英文语言包（554 条 key）
    └── zh-CN.json   # 简体中文语言包（554 条 key）
```

使用方式：

```go
import "github.com/getmockd/mockd/pkg/i18n"

// 在命令定义中包裹用户可见文案
cmd.Flags().String("path", "", i18n.T("The URL where this mock will respond"))
```

翻译 key 以英文原文为 key，语言包文件按 `key -> 翻译` 映射。新增文案只需在两个语言包中添加对应条目。

### Dashboard 汉化 / Dashboard localization

Dashboard 前端为 Svelte 单 bundle 构建（上游源码在私有仓库，dist 由 CI 注入），本分支采用**字符串替换硬汉化**方式：

- 从 JS bundle 提取用户可见 UI 文案（370 条，共 430 处替换）
- 保留 HTTP 方法、协议名、CSS 类、SVG 路径、第三方库内部字符串
- `index.html` 设置 `lang="zh-CN"`，标题为「mockd - 模拟服务器管理面板」
- dist 目录已强制纳入版本控制（上游 `.gitignore` 忽略了 `dist/`，需 `git add -f`）

---

## 翻译维护 / Maintaining Translations

### 新增翻译条目 / Adding new keys

1. 在 CLI 代码中将文案包裹为 `i18n.T("...")`。
2. 在 `pkg/i18n/locales/en.json` 与 `zh-CN.json` 中同时添加条目。

### 校验 / Validation

```bash
go build ./...        # 编译校验
go test ./pkg/i18n/   # 翻译加载测试
```

---

## 已知说明 / Known Notes

- `TestLoadProjectConfig_Discovery` 在 macOS 上失败（`/var` 符号链接差异），上游原始仓库同样失败，与汉化无关。
- Dashboard 当前固定中文显示；如需界面内语言切换，需要上游开放前端源码后接入完整 i18n 方案。

---

## 许可证 / License

本项目基于上游 [Apache License 2.0](/LICENSE)。翻译与本地化内容同样遵循该许可证。
