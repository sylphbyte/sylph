# Sylph 框架

[![Go Report Card](https://goreportcard.com/badge/github.com/sylphbyte/sylph)](https://goreportcard.com/report/github.com/sylphbyte/sylph)
[![GoDoc](https://godoc.org/github.com/sylphbyte/sylph?status.svg)](https://godoc.org/github.com/sylphbyte/sylph)
[![License](https://img.shields.io/github/license/sylphbyte/sylph)](https://github.com/sylphbyte/sylph/blob/main/LICENSE)

Sylph 是一个高性能、可扩展的 Go 微服务框架，专为构建云原生应用而设计。它提供了丰富的功能和组件，帮助开发者快速构建稳定、高效、易于维护的应用程序。

## 核心功能

- **上下文管理**：提供强大的上下文管理能力，贯穿整个请求生命周期
- **结构化日志**：支持多级别的结构化日志记录，便于分析和追踪
- **事件系统**：基于主题的发布-订阅模式，支持同步和异步事件处理
- **通知系统**：提供灵活的通知机制，用于异步通知和系统解耦
- **HTTP服务**：基于Gin的HTTP服务器，简化Web API的开发
- **消息队列**：集成RocketMQ，提供可靠的消息队列功能
- **定时任务**：支持基于Cron表达式的定时任务调度
- **项目管理**：统一的项目生命周期管理，简化组件的启动和关闭
- **安全工具**：提供安全的Goroutine启动、GUID生成等辅助功能

## 快速入门

### 安装

```bash
go get -u github.com/sylphbyte/sylph
```

### 创建HTTP服务器

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
        // 从Gin上下文获取Sylph上下文
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
    
    // 创建项目
    project := sylph.NewProject()
    project.Mounts(server)
    
    // 启动项目
    if err := project.Boots(); err != nil {
        log.Fatalf("启动服务失败: %v", err)
    }
    
    // 等待中断信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 关闭项目
    if err := project.Shutdowns(); err != nil {
        log.Fatalf("关闭服务失败: %v", err)
    }
}
```

## 使用示例

### 上下文管理

```go
// 创建上下文
ctx := sylph.NewDefaultContext("user-service", "/api/users")

// 存储数据
ctx.Set("userId", "12345")

// 获取数据
if userId, ok := ctx.Get("userId"); ok {
    // 使用userId
}

// 注册上下文结束回调
ctx.OnDone(func() {
    // 资源清理
})
```

### 结构化日志

```go
// 使用上下文进行日志记录
ctx.Info("order.service", "订单创建成功", sylph.H{
    "orderId": "ORD123456",
    "amount": 99.99,
    "userId": "12345",
})

// 错误日志
err := doSomething()
if err != nil {
    ctx.Error("order.service", "创建订单失败", err, sylph.H{
        "userId": "12345",
        "productId": "PROD001",
    })
}
```

### 事件系统

```go
// 订阅事件
ctx.On("user.registered", func(eventCtx sylph.Context, payload interface{}) {
    // 处理事件
    userData := payload.(UserData)
    
    // 发送欢迎邮件等操作
})

// 触发事件
ctx.Emit("user.registered", UserData{
    ID: "12345",
    Email: "user@example.com",
    Name: "张三",
})
```

### HTTP服务

```go
// 创建HTTP服务器
server := sylph.NewGinServer(":8080")

// 添加中间件
server.Use(gin.Recovery())

// 路由组
api := server.Group("/api")
{
    api.GET("/users", listUsers)
    api.GET("/users/:id", getUser)
    api.POST("/users", createUser)
    api.PUT("/users/:id", updateUser)
    api.DELETE("/users/:id", deleteUser)
}

// 处理函数
func getUser(c *gin.Context) {
    ctx := sylph.FromGin(c)
    
    // 业务逻辑
    // ...
    
    c.JSON(200, gin.H{/* 响应数据 */})
}
```

### 消息队列

```go
// 创建生产者
producer := sylph.NewRocketProducer("ProducerGroup", []string{"localhost:9876"})

// 发送消息
message := sylph.NewRocketMessage("TopicTest", "TagA", []byte(`{"id": "12345"}`))
err := producer.SendSync(message)

// 创建消费者
consumer := sylph.NewRocketConsumer("ConsumerGroup", []string{"localhost:9876"})

// 订阅主题
consumer.Subscribe("TopicTest", "TagA", func(ctx sylph.Context, msg *sylph.RocketMessage) sylph.ConsumeConcludeAction {
    // 处理消息
    ctx.Info("mq.consumer", "接收到消息", sylph.H{
        "topic": msg.Topic,
        "body": string(msg.Body),
    })
    
    // 返回消费结果
    return sylph.ConsumeSuccess
})

// 启动消费者
consumer.Start()
```

### 定时任务

```go
// 创建定时任务服务
cronServer := sylph.NewCronServer()

// 添加定时任务
cronServer.AddJob("0 0 * * * *", "每小时任务", func(ctx sylph.Context) {
    ctx.Info("cron.task", "执行定时任务", sylph.H{
        "time": time.Now().Format(time.RFC3339),
    })
    
    // 执行定时任务逻辑
})

// 添加到项目
project.Mounts(cronServer)
```

## 最佳实践

### 上下文管理最佳实践

1. **始终传递上下文**: 在函数调用链中传递上下文，而不是创建新的上下文
2. **上下文命名规范**: 使用清晰的位置标识符，如 `模块.组件.功能`
3. **数据共享**: 不要在上下文中存储大对象，会增加内存占用

### 日志记录最佳实践

1. **结构化日志**: 使用结构化数据而不是字符串拼接
2. **合适的日志级别**: 根据实际情况选择合适的日志级别
3. **上下文关联**: 确保日志包含跟踪ID、请求路径等上下文信息
4. **敏感信息处理**: 不要记录密码、令牌等敏感信息

### 错误处理最佳实践

1. **统一错误处理**: 使用统一的错误处理模式
2. **错误传递**: 包装错误以提供更多上下文
3. **错误类型**: 使用有意义的错误类型，而不是通用错误

### 资源管理最佳实践

1. **及时释放资源**: 使用defer确保资源释放
2. **优雅关闭**: 注册信号处理，优雅关闭服务

## 文档

详细文档请参考以下资源：

- [API文档](./API.md) - 详细的API接口文档
- [使用指南](./GUIDE.md) - 框架使用指南和示例
- [架构设计](./ARCHITECTURE.md) - 框架架构设计文档

## 贡献

欢迎贡献代码、报告问题或提出新功能建议。请先阅读[贡献指南](./CONTRIBUTING.md)。

## 许可证

本项目采用 [MIT 许可证](./LICENSE)。 