# Sylph 框架

[![Go Report Card](https://goreportcard.com/badge/github.com/sylphbyte/sylph)](https://goreportcard.com/report/github.com/sylphbyte/sylph)
[![GoDoc](https://godoc.org/github.com/sylphbyte/sylph?status.svg)](https://godoc.org/github.com/sylphbyte/sylph)
[![Release](https://img.shields.io/github/v/release/sylphbyte/sylph)](https://github.com/sylphbyte/sylph/releases)
[![License](https://img.shields.io/github/license/sylphbyte/sylph)](https://github.com/sylphbyte/sylph/blob/main/LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://github.com/sylphbyte/sylph)

<p align="center">
  <img src="docs/images/sylph-logo.png" alt="Sylph Logo" width="300" height="auto">
</p>

<p align="center">
  <b>一个高性能、可扩展的 Go 微服务框架，专为构建云原生应用而设计</b>
</p>

## 🌟 特性亮点

Sylph 框架提供了一整套微服务开发工具，助您快速构建高效、稳定的云原生应用：

- **🔄 上下文管理** - 贯穿整个请求生命周期的强大上下文系统
- **📝 结构化日志** - 多级别JSON格式日志，支持异步记录和自定义格式
- **📡 事件系统** - 基于发布-订阅模式的灵活事件处理机制
- **🔔 通知系统** - 实时通知和异步消息传递
- **🌐 HTTP服务** - 基于Gin的高性能Web服务器，简化API开发
- **📨 消息队列** - 无缝集成RocketMQ，支持可靠的异步消息处理
- **⏱️ 定时任务** - 基于Cron表达式的定时任务调度系统
- **🔧 项目管理** - 统一的组件生命周期管理
- **🛡️ 安全工具** - 提供安全的Goroutine管理和GUID生成

## 📦 安装

```bash
# 使用go get安装
go get -u github.com/sylphbyte/sylph

# 或者在go.mod中引用
# go.mod
require (
    github.com/sylphbyte/sylph v1.1.0
)
```

## 🚀 快速开始

创建一个简单的HTTP服务：

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/gin-gonic/gin"
    "github.com/sylphbyte/sylph"
)

func main() {
    // 创建HTTP服务器
    server := sylph.NewGinServer(":8080")
    
    // 注册路由
    server.GET("/api/hello", func(c *gin.Context) {
        // 获取Sylph上下文
        ctx := sylph.FromGin(c)
        
        // 记录请求
        ctx.Info("api.hello", "接收到请求", sylph.H{
            "clientIP": c.ClientIP(),
        })
        
        // 返回响应
        c.JSON(200, gin.H{
            "message": "你好，世界！",
        })
    })
    
    // 创建并启动项目
    project := sylph.NewProject()
    project.Mounts(server)
    
    if err := project.Boots(); err != nil {
        log.Fatalf("启动服务失败: %v", err)
    }
    
    // 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    if err := project.Shutdowns(); err != nil {
        log.Fatalf("关闭服务失败: %v", err)
    }
}
```

## 🔍 核心组件

### 上下文管理

```go
// 创建上下文
ctx := sylph.NewDefaultContext("user-service", "/api/users")

// 存储和获取数据
ctx.Set("userId", "12345")
userId, _ := ctx.Get("userId")

// 结构化日志
ctx.Info("order.service", "订单创建成功", sylph.H{
    "orderId": "ORD123456",
    "amount": 99.99,
})
```

### 事件系统

```go
// 订阅事件
ctx.On("user.registered", func(eventCtx sylph.Context, payload interface{}) {
    userData := payload.(UserData)
    // 处理用户注册事件
})

// 触发事件
ctx.Emit("user.registered", UserData{
    ID: "12345",
    Email: "user@example.com",
})
```

### 定时任务

```go
// 创建定时任务
cronServer := sylph.NewCronServer()
cronServer.AddJob("0 0 * * * *", "每小时任务", func(ctx sylph.Context) {
    ctx.Info("cron.task", "执行定时任务", sylph.H{
        "time": time.Now().Format(time.RFC3339),
    })
})

// 添加到项目
project.Mounts(cronServer)
```

## 📊 性能基准

Sylph框架在高并发场景下表现出色：

| 操作 | 并发量 | QPS | 响应时间 |
|------|-------|-----|---------|
| HTTP请求 | 1000 | 50,000+ | <2ms |
| 日志记录 | 5000 | 200,000+ | <0.5ms |
| 事件触发 | 1000 | 100,000+ | <1ms |

## 🧩 项目结构

```
sylph/
├── context.go        # 上下文管理核心
├── logger.go         # 日志记录系统
├── event.go          # 事件系统
├── gin.go            # HTTP服务集成
├── rocket*.go        # RocketMQ集成
├── cron.go           # 定时任务系统
├── project.go        # 项目管理
└── utils.go          # 工具函数
```

## 📖 文档

详细文档请参考：

- [📚 API文档](./API.md) - 详细的API接口说明
- [📖 使用指南](./GUIDE.md) - 框架使用详解和最佳实践
- [🏗️ 架构设计](./ARCHITECTURE.md) - 框架架构和设计理念
- [💡 示例代码](./examples/) - 示例代码和应用场景

## 🤝 贡献

欢迎贡献代码、报告问题或提出新功能建议。请先阅读[贡献指南](./CONTRIBUTING.md)。

## 📄 许可证

本项目采用 [MIT 许可证](./LICENSE)。

## 🌐 相关项目

- [Gin Web框架](https://github.com/gin-gonic/gin)
- [Apache RocketMQ](https://github.com/apache/rocketmq-client-go)
- [Logrus日志库](https://github.com/sirupsen/logrus) 