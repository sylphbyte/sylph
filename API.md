# Sylph 框架 API 文档

## 目录

- [上下文 (Context)](#上下文-context)
- [日志系统 (Logger)](#日志系统-logger)
- [事件系统 (Event)](#事件系统-event)
- [HTTP服务 (Gin)](#http服务-gin)
- [消息队列 (RocketMQ)](#消息队列-rocketmq)
- [定时任务 (Cron)](#定时任务-cron)
- [项目管理 (Project)](#项目管理-project)
- [工具函数 (Utils)](#工具函数-utils)

## 上下文 (Context)

### 接口定义

```go
// Context 核心上下文接口，继承标准库context.Context，并扩展多种功能
type Context interface {
    context.Context     // 标准 context
    LogContext          // 日志功能
    DataContext         // 数据功能
    TakeHeader() IHeader // 获取请求头信息
    Clone() Context     // 创建上下文副本

    TakeLogger() ILogger // 获取日志记录器

    // 日志方法
    Info(location, msg string, data any)
    Trace(location, msg string, data any)
    Debug(location, msg string, data any)
    Warn(location, msg string, data any)
    Fatal(location, msg string, data any)
    Panic(location, msg string, data any)
    Error(location, message string, err error, data any)

    // JWT相关方法
    StoreJwtClaim(claim IJwtClaim) // 存储JWT声明
    JwtClaim() (claim IJwtClaim)   // 获取JWT声明

    // 通知相关方法
    SendError(title string, err error, fields ...H)   // 发送错误通知
    SendWarning(title string, fields ...H)           // 发送警告通知
    SendSuccess(title string, fields ...H)           // 发送成功通知
    SendInfo(title string, fields ...H)              // 发送信息通知
    
    // 事件系统方法
    On(eventName string, handlers ...EventHandler)     // 订阅事件
    OffEvent(eventName string)                         // 取消订阅事件
    Emit(eventName string, payload interface{})        // 同步触发事件
    AsyncEmit(eventName string, payload interface{})   // 异步触发事件(不等待)
    AsyncEmitAndWait(eventName string, payload interface{}) // 异步触发事件并等待完成
}

// DataContext 数据上下文接口，提供键值存储功能
type DataContext interface {
    Get(key string) (val any, ok bool) // 获取指定键的值
    Set(key string, val any)           // 设置指定键的值
}

// LogContext 日志上下文接口，定义了各种日志级别的记录方法
type LogContext interface {
    Info(location, msg string, data any)                 // 记录信息级别日志
    Trace(location, msg string, data any)                // 记录跟踪级别日志
    Debug(location, msg string, data any)                // 记录调试级别日志
    Warn(location, msg string, data any)                 // 记录警告级别日志
    Fatal(location, msg string, data any)                // 记录致命级别日志
    Panic(location, msg string, data any)                // 记录恐慌级别日志
    Error(location, message string, err error, data any) // 记录错误级别日志
}
```

### 创建上下文

```go
// 创建默认上下文
// 参数:
//   - endpoint: 服务端点名称
//   - path: 请求路径
// 返回: 
//   - Context: 上下文实例
func NewDefaultContext(endpoint Endpoint, path string) Context
```

### 使用示例

```go
// 创建上下文
ctx := sylph.NewDefaultContext("api-service", "/user/profile")

// 上下文数据存取
ctx.Set("userId", "12345")
if userId, ok := ctx.Get("userId"); ok {
    fmt.Println("用户ID:", userId)
}

// 记录日志
ctx.Info("user.service", "用户登录成功", sylph.H{"userId": "12345"})
ctx.Error("api.handler", "处理请求失败", err, sylph.H{"path": "/user/profile"})

// 克隆上下文
newCtx := ctx.Clone()
```

## 日志系统 (Logger)

### 接口定义

```go
// ILogger 日志记录器接口，定义了各种日志级别的记录方法
type ILogger interface {
    Info(message *LoggerMessage)             // 记录信息级别日志
    Trace(message *LoggerMessage)            // 记录跟踪级别日志
    Debug(message *LoggerMessage)            // 记录调试级别日志
    Warn(message *LoggerMessage)             // 记录警告级别日志
    Fatal(message *LoggerMessage)            // 记录致命级别日志
    Panic(message *LoggerMessage)            // 记录恐慌级别日志
    Error(message *LoggerMessage, err error) // 记录错误级别日志
}

// LoggerMessage 日志消息结构体，包含日志的全部上下文信息
type LoggerMessage struct {
    Header   *Header `json:"header"`    // 请求头信息
    Location string  `json:"-"`         // 代码位置，不序列化到JSON
    Message  string  `json:"-"`         // 日志消息，不序列化到JSON
    Data     any     `json:"data,omitempty"`    // 数据内容
    Error    string  `json:"error,omitempty"`   // 错误信息
    Stack    string  `json:"stack,omitempty"`   // 堆栈信息
}
```

### 创建日志记录器

```go
// 创建新的日志记录器
// 参数:
//   - name: 日志记录器名称
//   - opt: 日志配置选项
// 返回:
//   - *Logger: 日志记录器实例
func NewLogger(name string, opt *LoggerConfig) *Logger

// 创建带默认配置的日志记录器
// 参数:
//   - name: 日志记录器名称
// 返回:
//   - *Logger: 日志记录器实例
func DefaultLogger(name string) *Logger
```

### 日志配置

```go
// LoggerConfig 日志配置结构体
type LoggerConfig struct {
    Path          string         // 日志文件路径
    Level         logrus.Level   // 日志级别
    Format        string         // 日志格式 (json/text)
    ReportCaller  bool           // 是否记录调用者信息
    Async         bool           // 是否使用异步记录
}
```

### 使用示例

```go
// 创建自定义日志配置
config := &LoggerConfig{
    Path:         "/var/log/app",
    Level:        logrus.InfoLevel,
    Format:       "json",
    ReportCaller: true,
    Async:        true,
}

// 创建日志记录器
logger := NewLogger("api-service", config)

// 记录日志
logger.Info(&LoggerMessage{
    Header:   &Header{Endpoint: "api-service", TraceIdVal: "abc123"},
    Location: "user.service",
    Message:  "用户登录成功",
    Data:     sylph.H{"userId": "12345"},
})

// 记录错误
logger.Error(&LoggerMessage{
    Header:   &Header{Endpoint: "api-service", TraceIdVal: "abc123"},
    Location: "auth.service",
    Message:  "验证失败",
    Data:     sylph.H{"userId": "12345"},
}, errors.New("token expired"))
```

## 事件系统 (Event)

### 接口定义

```go
// EventHandler 定义事件处理函数类型，接收上下文对象和事件载荷作为参数
type EventHandler func(ctx Context, payload interface{})
```

### 事件方法

```go
// 订阅事件
// 将处理函数添加到指定事件的订阅列表中
// 参数:
//   - eventName: 事件名称
//   - handlers: 一个或多个事件处理函数
func (ctx *DefaultContext) On(eventName string, handlers ...EventHandler)

// 取消订阅事件
// 从订阅映射中移除指定事件的所有处理函数
// 参数:
//   - eventName: 要取消订阅的事件名称
func (ctx *DefaultContext) OffEvent(eventName string)

// 同步触发事件
// 同步调用所有订阅了指定事件的处理函数
// 参数:
//   - eventName: 事件名称
//   - payload: 事件载荷
func (ctx *DefaultContext) Emit(eventName string, payload interface{})

// 异步触发事件(不等待)
// 异步调用所有订阅了指定事件的处理函数，但不等待它们完成
// 参数:
//   - eventName: 事件名称
//   - payload: 事件载荷
func (ctx *DefaultContext) AsyncEmit(eventName string, payload interface{})

// 异步触发事件并等待完成
// 异步调用所有订阅了指定事件的处理函数，并使用WaitGroup等待全部完成
// 参数:
//   - eventName: 事件名称
//   - payload: 事件载荷
func (ctx *DefaultContext) AsyncEmitAndWait(eventName string, payload interface{})
```

### 使用示例

```go
// 订阅事件
ctx.On("user.created", func(ctx sylph.Context, payload interface{}) {
    userData := payload.(UserData)
    ctx.Info("event.handler", "处理用户创建事件", sylph.H{
        "userId": userData.ID,
        "email": userData.Email,
    })
})

// 同步触发事件
userData := UserData{ID: "12345", Email: "user@example.com"}
ctx.Emit("user.created", userData)

// 异步触发事件(不等待)
ctx.AsyncEmit("user.created", userData)

// 异步触发事件(等待完成)
ctx.AsyncEmitAndWait("user.created", userData)
```

## HTTP服务 (Gin)

### 接口定义

```go
// GinServer HTTP服务器接口
type GinServer interface {
    Boot() error       // 启动服务
    Shutdown() error   // 关闭服务
    
    // Gin引擎方法
    GET(path string, handlers ...gin.HandlerFunc)
    POST(path string, handlers ...gin.HandlerFunc)
    PUT(path string, handlers ...gin.HandlerFunc)
    DELETE(path string, handlers ...gin.HandlerFunc)
    Use(middlewares ...gin.HandlerFunc)
    Group(path string, handlers ...gin.HandlerFunc) *gin.RouterGroup
}
```

### 创建HTTP服务

```go
// 创建新的Gin HTTP服务器
// 参数:
//   - addr: 监听地址，如 ":8080"
// 返回:
//   - GinServer: HTTP服务器实例
func NewGinServer(addr string) GinServer
```

### 使用示例

```go
// 创建HTTP服务器
server := sylph.NewGinServer(":8080")

// 添加中间件
server.Use(gin.Recovery())
server.Use(gin.Logger())

// 添加路由
server.GET("/api/users/:id", func(c *gin.Context) {
    // 从Gin上下文获取Sylph上下文
    ctx := sylph.FromGin(c)
    
    // 记录请求
    ctx.Info("api.users", "获取用户信息", sylph.H{
        "id": c.Param("id"),
    })
    
    // 业务逻辑...
    
    c.JSON(200, gin.H{
        "id": c.Param("id"),
        "name": "张三",
        "email": "zhangsan@example.com",
    })
})

// 创建项目并挂载HTTP服务
project := sylph.NewProject()
project.Mounts(server)

// 启动项目
if err := project.Boots(); err != nil {
    log.Fatalf("启动服务失败: %v", err)
}
```

## 消息队列 (RocketMQ)

### 接口定义

```go
// RocketProducer 消息生产者接口
type RocketProducer interface {
    SendSync(message *RocketMessage) error   // 同步发送消息
    SendAsync(message *RocketMessage) error  // 异步发送消息
    Close() error                           // 关闭生产者
}

// RocketConsumer 消息消费者接口
type RocketConsumer interface {
    Subscribe(topic, expression string, handler MessageHandler) error  // 订阅主题
    Start() error                                                     // 启动消费者
    Shutdown() error                                                  // 关闭消费者
}

// MessageHandler 消息处理函数类型
type MessageHandler func(ctx Context, msg *RocketMessage) ConsumeConcludeAction
```

### 消息结构

```go
// RocketMessage 消息结构体
type RocketMessage struct {
    Topic      string            // 主题
    Tag        string            // 标签
    Body       []byte            // 消息体
    Properties map[string]string // 消息属性
}
```

### 创建生产者和消费者

```go
// 创建新的RocketMQ生产者
// 参数:
//   - group: 生产者组名
//   - nameServers: NameServer地址列表
// 返回:
//   - RocketProducer: 生产者实例
func NewRocketProducer(group string, nameServers []string) RocketProducer

// 创建新的RocketMQ消费者
// 参数:
//   - group: 消费者组名
//   - nameServers: NameServer地址列表
// 返回:
//   - RocketConsumer: 消费者实例
func NewRocketConsumer(group string, nameServers []string) RocketConsumer
```

### 使用示例

```go
// 创建生产者
producer := sylph.NewRocketProducer("ProducerGroup", []string{"localhost:9876"})

// 创建消息
message := sylph.NewRocketMessage("TopicTest", "TagA", []byte(`{"id": "12345", "name": "张三"}`))

// 同步发送消息
err := producer.SendSync(message)
if err != nil {
    log.Fatalf("发送消息失败: %v", err)
}

// 创建消费者
consumer := sylph.NewRocketConsumer("ConsumerGroup", []string{"localhost:9876"})

// 订阅主题
consumer.Subscribe("TopicTest", "TagA", func(ctx sylph.Context, msg *sylph.RocketMessage) sylph.ConsumeConcludeAction {
    ctx.Info("consumer", "接收到消息", sylph.H{
        "topic": msg.Topic,
        "tag": msg.Tag,
        "body": string(msg.Body),
    })
    
    // 处理消息...
    
    return sylph.ConsumeSuccess
})

// 启动消费者
if err := consumer.Start(); err != nil {
    log.Fatalf("启动消费者失败: %v", err)
}

// 确保资源正确释放
defer func() {
    producer.Close()
    consumer.Shutdown()
}()
```

## 定时任务 (Cron)

### 接口定义

```go
// CronServer 定时任务服务接口
type CronServer interface {
    Boot() error      // 启动服务
    Shutdown() error  // 关闭服务
    AddJob(spec, name string, job CronJob) error  // 添加定时任务
    RemoveJob(name string) error                 // 移除定时任务
}

// CronJob 定时任务函数类型
type CronJob func(ctx Context)
```

### 创建定时任务服务

```go
// 创建新的定时任务服务
// 返回:
//   - CronServer: 定时任务服务实例
func NewCronServer() CronServer
```

### 使用示例

```go
// 创建定时任务服务
cronServer := sylph.NewCronServer()

// 添加定时任务 (每5分钟执行一次)
cronServer.AddJob("0 */5 * * * *", "数据同步", func(ctx sylph.Context) {
    ctx.Info("cron.sync", "开始数据同步", sylph.H{
        "time": time.Now().Format(time.RFC3339),
    })
    
    // 执行同步逻辑...
    
    ctx.Info("cron.sync", "数据同步完成", sylph.H{
        "duration": "2.5s",
    })
})

// 创建项目并挂载定时任务服务
project := sylph.NewProject()
project.Mounts(cronServer)

// 启动项目
if err := project.Boots(); err != nil {
    log.Fatalf("启动定时任务服务失败: %v", err)
}
```

## 项目管理 (Project)

### 接口定义

```go
// IServer 服务接口，所有可挂载到项目的服务都应实现此接口
type IServer interface {
    Boot() error      // 启动服务
    Shutdown() error  // 关闭服务
}

// Project 项目管理结构体
type Project struct {
    servers []IServer  // 服务列表
}
```

### 项目管理方法

```go
// 创建新的项目
// 返回:
//   - *Project: 项目实例
func NewProject() *Project

// 挂载服务到项目
// 参数:
//   - servers: 一个或多个服务实例
func (p *Project) Mounts(servers ...IServer)

// 启动所有服务
// 返回:
//   - error: 启动失败时返回错误
func (p *Project) Boots() error

// 关闭所有服务
// 返回:
//   - error: 关闭失败时返回错误
func (p *Project) Shutdowns() error
```

### 使用示例

```go
// 创建项目
project := sylph.NewProject()

// 创建各种服务
httpServer := sylph.NewGinServer(":8080")
cronServer := sylph.NewCronServer()
rocketConsumer := sylph.NewRocketConsumer("ConsumerGroup", []string{"localhost:9876"})

// 挂载服务
project.Mounts(httpServer, cronServer, rocketConsumer)

// 启动项目
if err := project.Boots(); err != nil {
    log.Fatalf("启动项目失败: %v", err)
}

// 注册信号处理，优雅关闭
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
<-c

// 关闭项目
if err := project.Shutdowns(); err != nil {
    log.Fatalf("关闭项目失败: %v", err)
}
```

## 工具函数 (Utils)

### 安全的Goroutine启动

```go
// SafeGo 安全地启动goroutine，捕获并记录panic
// 参数:
//   - ctx: 上下文
//   - location: 代码位置
//   - fn: 要执行的函数
func SafeGo(ctx Context, location string, fn func())
```

### 辅助类型

```go
// H 便捷的map[string]interface{}类型别名，用于构造结构化数据
type H map[string]interface{}
```

### 使用示例

```go
// 安全启动goroutine
sylph.SafeGo(ctx, "background.task", func() {
    // 可能会panic的代码
    result := riskyOperation()
    
    // 处理结果
    ctx.Info("background.task", "操作完成", sylph.H{
        "result": result,
    })
})

// 使用H类型构造结构化数据
data := sylph.H{
    "userId": "12345",
    "name": "张三",
    "age": 30,
    "roles": []string{"admin", "user"},
}
``` 