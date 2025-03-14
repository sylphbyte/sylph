# Sylph 框架使用指南

## 目录

1. [快速入门](#快速入门)
2. [基础组件](#基础组件)
3. [高级功能](#高级功能)
4. [最佳实践](#最佳实践)
5. [排错指南](#排错指南)
6. [性能优化](#性能优化)

## 快速入门

### 安装

```bash
# 使用go get安装
go get -u github.com/sylphbyte/sylph

# 通过go.mod添加依赖
# go.mod
module your-project

go 1.24.0

require (
    github.com/sylphbyte/sylph v1.0.0
)
```

### 创建项目骨架

以下是一个简单的REST API服务的基本结构：

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
    
    // 添加API路由
    setupRoutes(server)
    
    // 创建项目
    project := sylph.NewProject()
    project.Mounts(server)
    
    // 启动项目
    if err := project.Boots(); err != nil {
        log.Fatalf("启动服务失败: %v", err)
    }
    
    // 等待中断信号以优雅地关闭服务器
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 关闭项目
    if err := project.Shutdowns(); err != nil {
        log.Fatalf("关闭服务失败: %v", err)
    }
}

func setupRoutes(server sylph.GinServer) {
    // 健康检查接口
    server.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })
    
    // API接口组
    api := server.Group("/api")
    {
        api.GET("/users/:id", getUser)
        api.POST("/users", createUser)
        api.PUT("/users/:id", updateUser)
        api.DELETE("/users/:id", deleteUser)
    }
}

func getUser(c *gin.Context) {
    // 从Gin上下文获取Sylph上下文
    ctx := sylph.FromGin(c)
    
    // 记录请求
    ctx.Info("api.users.get", "获取用户信息", sylph.H{
        "id": c.Param("id"),
    })
    
    // 获取用户逻辑...
    
    c.JSON(200, gin.H{
        "id": c.Param("id"),
        "name": "张三",
        "email": "zhangsan@example.com",
    })
}

// 其他处理函数...
```

## 基础组件

### 上下文管理

上下文是Sylph框架中最核心的概念，它贯穿整个请求处理流程。

#### 创建上下文

```go
// 创建上下文
ctx := sylph.NewDefaultContext("服务名称", "请求路径")
```

#### 上下文数据共享

```go
// 存储数据
ctx.Set("userId", "12345")

// 获取数据
if userId, ok := ctx.Get("userId"); ok {
    // 使用userId
    fmt.Println("当前用户ID:", userId)
}
```

#### 上下文传递

```go
// 跨函数调用传递上下文
func processRequest(ctx sylph.Context, req Request) Response {
    // 使用上下文调用其他函数
    data, err := fetchData(ctx, req.ID)
    if err != nil {
        // 记录错误
        ctx.Error("processRequest", "获取数据失败", err, sylph.H{
            "requestId": req.ID,
        })
        return ErrorResponse(err)
    }
    
    return SuccessResponse(data)
}

func fetchData(ctx sylph.Context, id string) (Data, error) {
    // 记录操作
    ctx.Debug("fetchData", "正在获取数据", sylph.H{"id": id})
    
    // 业务逻辑...
    
    return data, nil
}
```

### 日志记录

Sylph提供了丰富的日志记录功能，支持多级别的结构化日志。

#### 日志级别

```go
// 不同日志级别
ctx.Debug("位置", "调试信息", data)  // 调试级别，用于开发环境
ctx.Info("位置", "信息记录", data)   // 信息级别，常规信息记录
ctx.Warn("位置", "警告信息", data)   // 警告级别，潜在问题
ctx.Error("位置", "错误信息", err, data) // 错误级别，程序错误
ctx.Fatal("位置", "致命错误", data)  // 致命级别，程序无法继续运行
ctx.Panic("位置", "恐慌错误", data)  // 恐慌级别，立即中断
```

#### 结构化日志

```go
// 使用结构化数据
ctx.Info("order.service", "订单创建成功", sylph.H{
    "orderId": "ORD123456",
    "amount": 99.99,
    "currency": "CNY",
    "customer": sylph.H{
        "id": "CUST789",
        "name": "张三",
    },
    "items": []sylph.H{
        {"id": "PROD001", "quantity": 2},
        {"id": "PROD002", "quantity": 1},
    },
})
```

#### 配置日志

```go
// 创建自定义日志配置
config := &sylph.LoggerConfig{
    Path:         "/var/log/app",
    Level:        logrus.InfoLevel,
    Format:       "json",
    ReportCaller: true,
    Async:        true,
}

// 创建自定义日志记录器
logger := sylph.NewLogger("api-service", config)

// 应用关闭前确保资源释放
defer sylph.CloseLoggers()
```

### 事件系统

事件系统允许你创建松耦合的组件，通过事件通信而不是直接调用。

#### 订阅事件

```go
// 订阅事件
ctx.On("user.registered", func(eventCtx sylph.Context, payload interface{}) {
    // 类型断言获取数据
    userData := payload.(UserData)
    
    // 处理事件
    eventCtx.Info("event.user.registered", "新用户注册", sylph.H{
        "userId": userData.ID,
        "email": userData.Email,
    })
    
    // 发送欢迎邮件
    sendWelcomeEmail(eventCtx, userData)
})
```

#### 触发事件

```go
// 同步触发事件
userData := UserData{
    ID: "12345",
    Email: "user@example.com",
    Name: "张三",
}
ctx.Emit("user.registered", userData)

// 异步触发事件(不等待完成)
ctx.AsyncEmit("user.registered", userData)

// 异步触发事件(等待所有处理完成)
ctx.AsyncEmitAndWait("user.registered", userData)
```

### HTTP服务

Sylph框架集成了Gin，简化了HTTP服务的创建和管理。

#### 创建HTTP服务器

```go
// 创建HTTP服务器
server := sylph.NewGinServer(":8080")

// 添加中间件
server.Use(gin.Recovery())
server.Use(gin.Logger())

// 注册路由
server.GET("/api/users", listUsers)
server.POST("/api/users", createUser)
```

#### 接口处理函数

```go
func listUsers(c *gin.Context) {
    // 从Gin上下文获取Sylph上下文
    ctx := sylph.FromGin(c)
    
    // 记录请求
    ctx.Info("api.users.list", "获取用户列表", sylph.H{
        "page": c.Query("page"),
        "limit": c.Query("limit"),
    })
    
    // 获取数据...
    users := []gin.H{
        {"id": "1", "name": "张三"},
        {"id": "2", "name": "李四"},
    }
    
    // 返回响应
    c.JSON(200, gin.H{
        "data": users,
        "total": 2,
    })
}
```

### 消息队列

Sylph集成了RocketMQ，提供可靠的消息队列功能。

#### 创建生产者

```go
// 创建生产者
producer := sylph.NewRocketProducer("ProducerGroup", []string{"localhost:9876"})

// 创建消息
message := sylph.NewRocketMessage("TopicTest", "TagA", []byte(`{"id": "12345", "name": "张三"}`))

// 发送消息
err := producer.SendSync(message)
if err != nil {
    ctx.Error("mq.producer", "发送消息失败", err, sylph.H{
        "topic": message.Topic,
        "tag": message.Tag,
    })
    return
}

// 关闭生产者
defer producer.Close()
```

#### 创建消费者

```go
// 创建消费者
consumer := sylph.NewRocketConsumer("ConsumerGroup", []string{"localhost:9876"})

// 订阅主题
consumer.Subscribe("TopicTest", "TagA", func(ctx sylph.Context, msg *sylph.RocketMessage) sylph.ConsumeConcludeAction {
    // 记录消息接收
    ctx.Info("mq.consumer", "接收到消息", sylph.H{
        "topic": msg.Topic,
        "tag": msg.Tag,
        "body": string(msg.Body),
    })
    
    // 处理消息...
    var data map[string]interface{}
    if err := json.Unmarshal(msg.Body, &data); err != nil {
        ctx.Error("mq.consumer", "解析消息失败", err, sylph.H{
            "body": string(msg.Body),
        })
        return sylph.ConsumeRetryLater
    }
    
    // 成功处理
    return sylph.ConsumeSuccess
})

// 启动消费者
consumer.Start()

// 关闭消费者
defer consumer.Shutdown()
```

### 定时任务

Sylph提供了简洁的定时任务接口，支持Cron表达式。

#### 创建定时任务

```go
// 创建定时任务服务
cronServer := sylph.NewCronServer()

// 添加定时任务 (每天凌晨3点执行)
cronServer.AddJob("0 0 3 * * *", "数据备份", func(ctx sylph.Context) {
    ctx.Info("cron.backup", "开始数据备份", sylph.H{
        "time": time.Now().Format(time.RFC3339),
    })
    
    // 执行备份逻辑...
    
    ctx.Info("cron.backup", "数据备份完成", sylph.H{
        "duration": "5m30s",
    })
})

// 添加到项目
project := sylph.NewProject()
project.Mounts(cronServer)
```

## 高级功能

### JWT集成

```go
// 创建JWT声明
type UserClaim struct {
    ID      string
    Token   string
    Issuer  string
}

// 实现IJwtClaim接口
func (c *UserClaim) TakeId() string {
    return c.ID
}

func (c *UserClaim) TakeToken() string {
    return c.Token
}

func (c *UserClaim) TakeIssuer() string {
    return c.Issuer
}

func (c *UserClaim) IssuerIs(name string) bool {
    return c.Issuer == name
}

// 存储JWT声明
func handleLogin(c *gin.Context) {
    ctx := sylph.FromGin(c)
    
    // ... 验证用户凭证 ...
    
    // 创建并存储JWT声明
    claim := &UserClaim{
        ID:     "12345",
        Token:  "eyJhbGciOiJ...",
        Issuer: "auth-service",
    }
    ctx.StoreJwtClaim(claim)
    
    // 后续处理...
}

// 获取JWT声明
func handleProtectedResource(c *gin.Context) {
    ctx := sylph.FromGin(c)
    
    // 获取JWT声明
    claim := ctx.JwtClaim()
    if claim == nil {
        c.JSON(401, gin.H{"error": "未授权访问"})
        return
    }
    
    // 使用声明信息
    userId := claim.TakeId()
    
    // 处理请求...
}
```

### 安全的Goroutine

```go
// 安全启动goroutine
sylph.SafeGo(ctx, "background.task", func() {
    // 长时间运行的任务，可能会panic
    processLargeDataset(ctx)
})

// 与普通的goroutine启动相比
// 不安全方式:
go func() {
    // 如果panic，整个程序可能崩溃
    processLargeDataset(ctx)
}()
```

## 最佳实践

### 上下文管理最佳实践

1. **始终传递上下文**: 在函数调用链中传递上下文，而不是创建新的上下文
2. **上下文命名规范**: 使用清晰的位置标识符，如 `模块.组件.功能`
3. **数据共享**: 不要在上下文中存储大对象，会增加内存占用

### 日志记录最佳实践

1. **结构化日志**: 使用结构化数据而不是字符串拼接
2. **合适的日志级别**: 
   - Debug: 开发调试信息
   - Info: 正常操作信息
   - Warn: 潜在问题警告
   - Error: 操作失败但不影响整体功能
   - Fatal/Panic: 严重错误，程序无法继续
3. **上下文关联**: 确保日志包含跟踪ID、请求路径等上下文信息
4. **敏感信息处理**: 不要记录密码、令牌等敏感信息

### 错误处理最佳实践

1. **统一错误处理**: 使用统一的错误处理模式
   ```go
   if err != nil {
       ctx.Error("service.Operation", "操作失败", err, sylph.H{"param": value})
       return nil, err
   }
   ```

2. **错误传递**: 包装错误以提供更多上下文
   ```go
   if err != nil {
       return nil, fmt.Errorf("获取用户数据失败: %w", err)
   }
   ```

3. **错误类型**: 使用有意义的错误类型，而不是通用错误

### 资源管理最佳实践

1. **及时释放资源**: 使用defer确保资源释放
   ```go
   defer func() {
       if err := producer.Close(); err != nil {
           ctx.Error("cleanup", "关闭生产者失败", err, nil)
       }
   }()
   ```

2. **优雅关闭**: 注册信号处理，优雅关闭服务
   ```go
   c := make(chan os.Signal, 1)
   signal.Notify(c, os.Interrupt, syscall.SIGTERM)
   <-c
   project.Shutdowns()
   ```

## 排错指南

### 常见问题

1. **日志丢失**
   - 检查是否启用了异步日志但应用关闭前未调用`Close()`
   - 验证日志级别设置是否正确

   解决方案:
   ```go
   // 确保在应用关闭前关闭日志管理器
   defer sylph.CloseLoggers()
   ```

2. **内存使用过高**
   - 检查是否有对象循环引用
   - 是否有资源泄漏

   解决方案:
   ```go
   // 调整异步日志缓冲区大小
   config := &sylph.LoggerConfig{
       Async: true,
       BufferSize: 500, // 根据实际需求调整
   }
   ```

3. **消息处理失败**
   - 检查RocketMQ连接配置
   - 确认消费者组和主题配置正确

   解决方案:
   ```go
   // 添加重试逻辑
   for i := 0; i < 3; i++ {
       err := producer.SendSync(message)
       if err == nil {
           break
       }
       
       ctx.Warn("mq.producer", "发送消息失败，准备重试", sylph.H{
           "attempt": i+1,
           "error": err.Error(),
       })
       
       time.Sleep(time.Second * time.Duration(i+1))
   }
   ```

### 调试技巧

1. **日志级别**:
   ```go
   // 开发环境设置为Debug级别
   loggerConfig.Level = logrus.DebugLevel
   ```

2. **请求跟踪**:
   ```go
   // 输出跟踪ID
   traceId := ctx.TakeHeader().TraceId()
   fmt.Printf("跟踪ID: %s\n", traceId)
   ```

3. **性能分析**:
   ```go
   // 添加pprof支持
   import _ "net/http/pprof"
   
   // 在main函数中启动pprof服务
   go func() {
       http.ListenAndServe("localhost:6060", nil)
   }()
   ```

## 性能优化

### 日志优化

1. **异步日志**:
   ```go
   config := &sylph.LoggerConfig{
       Async: true,
       BufferSize: 1000,
   }
   ```

2. **日志级别**:
   ```go
   // 生产环境使用Info或更高级别
   config.Level = logrus.InfoLevel
   ```

### 内存优化

1. **对象池**:
   ```go
   // 对频繁创建的对象使用对象池
   var messagePool = sync.Pool{
       New: func() interface{} {
           return &Message{}
       },
   }
   
   // 获取对象
   msg := messagePool.Get().(*Message)
   
   // 使用完毕后归还
   messagePool.Put(msg)
   ```

2. **减少内存分配**:
   ```go
   // 预分配内存
   data := make([]byte, 0, 1024)
   ```

### 并发优化

1. **合理的并发度**:
   ```go
   // 限制goroutine数量
   worker := make(chan struct{}, runtime.NumCPU())
   
   for _, item := range items {
       worker <- struct{}{}
       
       go func(item Item) {
           defer func() { <-worker }()
           // 处理item
       }(item)
   }
   ```

2. **避免锁竞争**:
   ```go
   // 使用分段锁
   type ConcurrentMap struct {
       segments [16]struct {
           sync.RWMutex
           data map[string]interface{}
       }
   }
   