# ALLinker — 跨 Agent 协作入口工具

> 为不同的 AI Agent 软件提供统一的协作入口，实现跨 Agent 协同工作。

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

[English](../README.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Français](README.fr.md)

---

## 概述

ALLinker 是一个**基于 CLI 的协作入口工具**，专为在同一项目目录中协作的多个 AI Agent 工具（如 Cline、CodeX、自定义 Agent 等）而设计。

当多个 Agent 在同一个项目中独立工作时，常常面临：

- **文件冲突** — 多个 Agent 同时编辑同一文件
- **信息孤岛** — Agent 之间无法直接通信
- **操作不可追溯** — 无法审计谁在何时做了什么

ALLinker 通过**四个协作原语**解决这些问题：

| 原语 | 解决的问题 |
|------|-----------|
| **文件锁定** | Agent 编辑文件前先加锁，防止冲突 |
| **消息通信** | Agent 之间互相发送消息，支持 `@` 提醒 |
| **文件监听** | Agent 注册监听位，感知同事的进展 |
| **账号管理** | 身份签名 + 三级权限 + 完整审计追踪 |

---

## 快速开始

### 编译

```bash
git clone <repo-url>
cd ALLinker
go build -o allinker.exe .
```

### 注册 Agent

```bash
./allinker register --name TRAE --role agent
./allinker register --name CodeX --role agent
./allinker register --name 管理员 --role admin
```

### 文件锁定

```bash
./allinker lock -f PLAN_001.md -t 30 --user TRAE    # 阻塞等待（最多 30 秒）
./allinker tryLock -f PLAN_001.md --user TRAE        # 非阻塞尝试
./allinker unlock -f PLAN_001.md --user TRAE         # 释放锁
./allinker status -f PLAN_001.md                     # 查看锁状态
./allinker status --all                              # 查看所有锁
```

### 消息通信

```bash
./allinker send --at CodeX --msg "请实现用户认证模块" --user TRAE
./allinker send --at All --msg "群发通知" --user TRAE
./allinker recv                                                   # 接收消息
./allinker history --with CodeX --limit 10                        # 查看历史记录
```

### 文件监听 — 等待同伴响应

Agent A 向 Agent B 派发任务，然后监听 B 返回的响应文件：

```bash
# Agent A：注册一个监听位，等待同伴的响应文件
./allinker watch add --name "resp-auth-module" -d ./CodeX -p "RESP_*.md" --user TRAE

# Agent A：阻塞等待响应文件出现（最多 300 秒超时）
./allinker wait -d ./CodeX -f "RESP_*.md" -t 300

# Agent A：检查响应是否已到达
./allinker watch check --name "resp-auth-module"

# 列出所有活跃监听位
./allinker watch list

# 完成后移除监听位
./allinker watch remove --name "resp-auth-module"
```

---

## 中心服务模式 — 跨主机局域网协作

ALLinker 可以以 HTTP 服务的形式长期驻留运行，**同一局域网内不同主机**上的 Agent 通过网络调用实现跨主机协作。这是多机团队协作的核心机制。

```bash
# 启动服务
./allinker -server --port 8080

# 客户端模式（连接远程服务）
./allinker --connect http://127.0.0.1:8080 lock -f PLAN_001.md --user TRAE

# 自动模式：检测到服务则走网络，否则本地执行
./allinker --auto send --at CodeX --msg "你好" --user TRAE

# 服务管理
./allinker -server --stop
./allinker -server --status
```

### HTTP API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/command` | POST | 远程执行命令 |
| `/api/v1/health` | GET | 健康检查 |
| `/api/v1/status` | GET | 服务状态 |

---

## 全平台编译

在 Windows 上运行附带的构建脚本，可生成跨平台二进制文件：

```bat
build.bat
```

生成文件：

| 二进制文件 | 平台 |
|-----------|------|
| `ALLinker_windows_amd64.exe` | Windows x64 |
| `ALLinker_windows_386.exe` | Windows x86 |
| `ALLinker_linux_amd64` | Linux x64 |
| `ALLinker_linux_arm64` | Linux ARM64 |
| `ALLinker_darwin_amd64` | macOS Intel |
| `ALLinker_darwin_arm64` | macOS Apple Silicon |

---

## 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 操作成功 |
| 1 | 一般错误 |
| 2 | 超时（wait） |
| 3 | 锁获取失败（tryLock） |
| 4 | 账号不存在 |
| 5 | 权限不足 |
| 6 | 文件不存在 |

---

## 数据存储

所有数据存储在 `.alf/` 目录下（可通过 `--data-dir` 指定）：

```
.alf/
├── users.json        # 用户账号
├── config.json       # 工具配置
├── counter.json      # ID 计数器
├── watchlist.json    # 监听位注册表
├── allinker.db       # SQLite 数据库（消息 + 锁 + 监听位）
└── Logs/             # 日志文件（每日轮转 YYYY-MM-DD.log）
```

写操作采用**原子写入**（临时文件 → 重命名），防止数据损坏。

---

## 项目结构

```
.
├── main.go        # 入口
├── go.mod
├── build.bat      # 跨平台编译脚本
├── account/       # 账号管理
├── cli/           # CLI 命令路由
├── config/        # 配置管理
├── core/          # 全局单例
├── init/          # 数据目录与数据库初始化
├── lock/          # 文件锁
├── logutil/       # 日志与审计
├── message/       # 消息通信
├── model/         # 数据模型
├── storage/       # JSON 持久化
├── wait/          # 阻塞式文件等待
└── watch/         # 文件监听
```

---

## 许可证

[Apache License 2.0](../LICENCE)
