# Sylph 框架架构设计

## 总览

Sylph 是一个专为高并发、高可用的微服务环境设计的 Go 框架。它的核心设计理念是提供一套全面的开发工具，让开发者能够快速构建稳定、高效、易于维护的云原生应用。

![架构图](assets/architecture.png)

## 设计理念

### 核心价值观

1. **简洁性** - 提供直观的 API，减少开发者的认知负担
2. **一致性** - 各组件之间保持一致的设计风格和使用模式
3. **可扩展性** - 核心功能高度模块化，便于扩展和定制
4. **高性能** - 精心优化的代码路径，支持高并发
5. **可靠性** - 稳定的错误处理，优雅降级策略

### 设计目标

Sylph 框架致力于解决微服务架构中的常见问题：

- **上下文传递** - 提供统一的上下文管理机制，简化分布式系统中的数据传递
- **请求跟踪** - 内置请求跟踪能力，便于追踪请求在系统中的流转路径
- **结构化日志** - 提供一致的日志记录机制，便于日志分析和问题定位
- **错误处理** - 统一的错误处理模式，减少开发者的心智负担
- **资源管理** - 自动管理资源的生命周期，减少资源泄漏
- **事件驱动** - 基于事件的通信机制，促进系统解耦

## 系统架构

Sylph 框架采用分层架构，各层之间的耦合度低，可以根据需要选择性地使用：

### 核心层 (Core Layer)

核心层提供框架的基础功能，是其他所有组件的基石：

1. **上下文 (Context)** - 提供请求上下文管理
2. **日志 (Logger)** - 结构化日志记录
3. **事件 (Event)** - 进程内事件发布与订阅
4. **通知 (Notification)** - 通知机制，用于异步通知

#### 上下文 (Context) 设计

上下文是 Sylph 框架的核心，它贯穿整个请求的生命周期：

- **不可变原则** - 上下文内的核心属性一经设置不可变更，保证数据一致性
- **数据共享** - 提供类型安全的数据共享机制，避免类型转换错误
- **生命周期管理** - 自动管理与上下文关联的资源，确保资源释放
- **链式继承** - 子上下文继承父上下文的属性，但不会反向影响父上下文

关键接口：
```go
type Context interface {
    // 上下文标识相关
    Service() string
    Path() string
    TakeHeader() ContextHeader
    
    // 日志相关
    Info(pos, message string, data interface{})
    Error(pos, message string, err error, data interface{})
    // ... 更多日志方法 ...
    
    // 数据共享相关
    Get(key string) (interface{}, bool)
    Set(key string, val interface{})
    
    // 事件相关
    On(topic string, handler EventHandler)
    Emit(topic string, payload interface{})
    // ... 更多事件方法 ...
    
    // JWT相关
    StoreJwtClaim(IJwtClaim)
    JwtClaim() IJwtClaim
    
    // 生命周期相关
    Done() <-chan struct{}
    OnDone(func())
}
```

#### 日志 (Logger) 设计

日志系统设计为高性能、可靠的结构化日志记录工具：

- **结构化日志** - 所有日志都以结构化的 JSON 格式记录，便于分析和检索
- **异步处理** - 支持异步日志记录，最小化对主逻辑的影响
- **多级日志** - 支持不同的日志级别，便于控制日志输出
- **上下文关联** - 日志自动关联上下文信息，便于追踪

关键接口：
```go
type ILogger interface {
    Info(pos, message string, data interface{})
    Debug(pos, message string, data interface{})
    Warn(pos, message string, data interface{})
    Error(pos, message string, err error, data interface{})
    Fatal(pos, message string, data interface{})
    Panic(pos, message string, data interface{})
    Close()
}
```

#### 事件 (Event) 设计

事件系统提供进程内的事件发布与订阅机制：

- **主题订阅** - 基于主题的发布-订阅模式
- **同异步支持** - 支持同步和异步的事件处理
- **类型安全** - 事件处理中的类型安全
- **上下文传递** - 事件处理过程中保持上下文传递

关键接口：
```go
type EventHandler func(Context, interface{})

type EventBus interface {
    On(topic string, handler EventHandler)
    Off(topic string, handler EventHandler)
    Emit(topic string, payload interface{})
    AsyncEmit(topic string, payload interface{})
    AsyncEmitAndWait(topic string, payload interface{})
}
```

### 基础设施层 (Infrastructure Layer)

基础设施层提供与外部系统的集成：

1. **HTTP 服务 (HTTP Server)** - 基于 Gin 的 HTTP 服务器
2. **消息队列 (RocketMQ)** - RocketMQ 客户端集成
3. **定时任务 (Cron)** - 基于 Cron 表达式的定时任务
4. **项目管理 (Project)** - 统一的项目生命周期管理

#### HTTP 服务 (HTTP Server) 设计

HTTP 服务基于 Gin 框架构建：

- **上下文集成** - 与 Sylph 上下文无缝集成
- **中间件扩展** - 提供常用中间件和自定义中间件机制
- **路由管理** - 简洁的路由定义方式
- **错误处理** - 统一的错误处理机制

关键接口：
```go
type GinServer interface {
    Use(middlewares ...gin.HandlerFunc)
    Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup
    GET(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
    POST(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
    // ... 更多 HTTP 方法 ...
    
    Boot() error
    Shutdown() error
}
```

#### 消息队列 (RocketMQ) 设计

RocketMQ 集成提供可靠的消息队列功能：

- **生产者抽象** - 简化消息发送流程
- **消费者抽象** - 简化消息消费流程
- **上下文传递** - 消息处理过程中保持上下文传递
- **错误处理** - 统一的错误处理和重试机制

关键接口：
```go
type RocketProducer interface {
    SendSync(message *RocketMessage) error
    SendAsync(message *RocketMessage, callback SendCallback) error
    SendOneway(message *RocketMessage) error
    Close() error
}

type RocketConsumer interface {
    Subscribe(topic, expression string, handler ConsumeHandler) error
    Start() error
    Shutdown() error
}

type ConsumeHandler func(Context, *RocketMessage) ConsumeConcludeAction
```

#### 定时任务 (Cron) 设计

定时任务系统提供基于 Cron 表达式的任务调度：

- **Cron 表达式** - 支持标准 Cron 表达式
- **上下文传递** - 任务执行过程中提供上下文
- **错误处理** - 任务执行错误的处理机制
- **状态监控** - 任务执行状态的监控

关键接口：
```go
type CronServer interface {
    AddJob(spec, name string, job CronJob) error
    RemoveJob(name string) error
    ListJobs() []CronJobInfo
    Boot() error
    Shutdown() error
}

type CronJob func(Context)
```

#### 项目管理 (Project) 设计

项目管理提供统一的项目生命周期管理：

- **组件注册** - 统一注册各种组件
- **启动顺序** - 控制组件的启动顺序
- **关闭顺序** - 控制组件的关闭顺序
- **状态监控** - 监控项目各组件状态

关键接口：
```go
type Project interface {
    Mounts(components ...Component)
    Boots() error
    Shutdowns() error
}

type Component interface {
    Boot() error
    Shutdown() error
}
```

### 工具层 (Utility Layer)

工具层提供各种辅助功能：

1. **安全运行 (SafeGo)** - 安全的 Goroutine 启动
2. **GUID 生成** - 全局唯一 ID 生成
3. **错误处理 (ErrorX)** - 增强的错误处理
4. **数据结构 (H)** - 便捷的数据结构定义

#### 安全运行 (SafeGo) 设计

SafeGo 提供安全的 Goroutine 启动机制：

- **异常捕获** - 捕获并记录 Goroutine 中的异常
- **上下文关联** - 与上下文关联，保持日志一致性
- **资源管理** - 确保资源正确释放

用法示例：
```go
SafeGo(ctx, "background.task", func() {
    // 可能会引发异常的代码
})
```

#### GUID 生成设计

GUID 生成提供全局唯一 ID：

- **高性能** - 高效的 ID 生成算法
- **唯一性** - 保证生成 ID 的唯一性
- **格式化** - 支持多种格式化选项

用法示例：
```go
id := NewGUID() // 生成标准格式的 GUID
shortId := NewShortGUID() // 生成短格式的 GUID
```

## 数据流

下面描述 Sylph 框架中的主要数据流：

### HTTP 请求处理流程

```
HTTP 请求 → Gin 路由 → Sylph 中间件 → Sylph 上下文创建 → 请求处理函数 → 响应生成 → HTTP 响应
```

详细步骤：

1. 客户端发送 HTTP 请求到服务器
2. Gin 路由系统根据 URL 定位到对应的处理函数
3. Sylph 中间件拦截请求，创建 Sylph 上下文
4. 上下文注入到 Gin 上下文中
5. 执行请求处理函数，处理业务逻辑
6. 生成响应结果，返回给客户端
7. 请求结束后，触发上下文的 Done 回调，释放资源

### 事件处理流程

```
事件触发 → 事件总线 → 查找处理函数 → 串行/并行执行处理函数 → 处理结果
```

详细步骤：

1. 系统中某个组件触发事件 (Emit)
2. 事件总线接收事件，查找对应主题的处理函数
3. 根据事件类型 (同步/异步)，执行对应的处理策略
4. 为每个处理函数提供独立的上下文环境
5. 执行处理函数，处理事件数据
6. 汇总处理结果 (如果需要)

### 消息队列处理流程

```
消息发送 → RocketMQ 生产者 → RocketMQ 服务器 → RocketMQ 消费者 → 消息处理函数 → 处理结果
```

详细步骤：

1. 应用通过 RocketProducer 发送消息
2. 消息被发送到 RocketMQ 服务器
3. RocketMQ 消费者从服务器拉取消息
4. Sylph 框架为每个消息创建上下文
5. 执行消息处理函数，处理消息数据
6. 根据处理结果，决定消息是否需要重试

## 扩展点

Sylph 框架设计了多个扩展点，方便开发者根据自己的需求进行扩展：

### 中间件扩展

可以通过实现自定义中间件，扩展 HTTP 服务的功能：

```go
func CustomMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 前置处理
        
        c.Next()
        
        // 后置处理
    }
}

// 使用中间件
server.Use(CustomMiddleware())
```

### 日志扩展

可以通过实现 ILogger 接口，扩展日志系统：

```go
type CustomLogger struct {
    // 自定义字段
}

func (l *CustomLogger) Info(pos, message string, data interface{}) {
    // 自定义实现
}

// 实现其他方法...
```

### 事件处理扩展

可以通过注册自定义事件处理函数，扩展事件处理能力：

```go
ctx.On("custom.event", func(eventCtx Context, payload interface{}) {
    // 自定义事件处理
})
```

### 组件扩展

可以通过实现 Component 接口，创建自定义组件：

```go
type CustomComponent struct {
    // 自定义字段
}

func (c *CustomComponent) Boot() error {
    // 启动逻辑
    return nil
}

func (c *CustomComponent) Shutdown() error {
    // 关闭逻辑
    return nil
}

// 注册组件
project.Mounts(&CustomComponent{})
```

## 性能考量

Sylph 框架在设计时充分考虑了性能因素：

### 内存优化

- **对象池** - 使用对象池减少 GC 压力
- **零分配** - 关键路径上尽量避免内存分配
- **结构体嵌入** - 利用 Go 的结构体嵌入减少内存开销

### 并发优化

- **锁分段** - 使用分段锁减少锁竞争
- **无锁数据结构** - 在可能的情况下使用无锁数据结构
- **原子操作** - 适当使用原子操作替代互斥锁

### IO 优化

- **缓冲区重用** - 重用缓冲区减少内存分配
- **批处理** - 批量处理 IO 操作，提高吞吐量
- **异步 IO** - 使用异步 IO 减少阻塞

## 核心代码示例

以下是一些核心组件的实现示例，展示 Sylph 框架的设计理念：

### 上下文实现示例

```go
type defaultContext struct {
    service      string
    path         string
    header       ContextHeader
    valueManager *valueManager
    eventBus     EventBus
    logger       ILogger
    jwtClaim     IJwtClaim
    done         chan struct{}
    onDoneFuncs  []func()
    mu           sync.RWMutex
}

func (ctx *defaultContext) Service() string {
    return ctx.service
}

func (ctx *defaultContext) Path() string {
    return ctx.path
}

func (ctx *defaultContext) TakeHeader() ContextHeader {
    return ctx.header
}

// ... 更多方法实现 ...
```

### 日志实现示例

```go
type defaultLogger struct {
    name         string
    logrusLogger *logrus.Logger
    hooks        []logrus.Hook
    async        bool
    logChan      chan *logEntry
    wg           sync.WaitGroup
    closed       int32
}

func (l *defaultLogger) Info(pos, message string, data interface{}) {
    l.log(logrus.InfoLevel, pos, message, nil, data)
}

func (l *defaultLogger) Error(pos, message string, err error, data interface{}) {
    l.log(logrus.ErrorLevel, pos, message, err, data)
}

// ... 更多方法实现 ...
```

### 事件实现示例

```go
type defaultEventBus struct {
    handlers map[string][]EventHandler
    mu       sync.RWMutex
}

func (bus *defaultEventBus) On(topic string, handler EventHandler) {
    bus.mu.Lock()
    defer bus.mu.Unlock()
    
    if bus.handlers == nil {
        bus.handlers = make(map[string][]EventHandler)
    }
    
    bus.handlers[topic] = append(bus.handlers[topic], handler)
}

func (bus *defaultEventBus) Emit(topic string, payload interface{}) {
    handlers := bus.getHandlers(topic)
    
    for _, handler := range handlers {
        handler(ctx, payload)
    }
}

// ... 更多方法实现 ...
```

## 部署架构

Sylph 框架支持多种部署模式，适应不同的应用场景：

### 单体部署

适用于小型应用，所有组件部署在同一个进程中：

```
+---------------------------+
|         Application       |
|                           |
|  +---------+  +--------+  |
|  |  HTTP   |  |  Cron  |  |
|  | Service |  | Server |  |
|  +---------+  +--------+  |
|                           |
|  +---------+  +--------+  |
|  | RocketMQ |  | Other  |  |
|  | Consumer |  |Components|
|  +---------+  +--------+  |
|                           |
+---------------------------+
```

### 微服务部署

适用于大型应用，各组件可以独立部署：

```
+-------------+   +-------------+   +-------------+
|   API       |   |   Worker    |   |   Scheduler |
|  Service    |   |   Service   |   |   Service   |
|             |   |             |   |             |
| +--------+  |   | +--------+  |   | +--------+  |
| |  HTTP  |  |   | |RocketMQ|  |   | |  Cron  |  |
| |Service |  |   | |Consumer|  |   | |Server  |  |
| +--------+  |   | +--------+  |   | +--------+  |
+-------------+   +-------------+   +-------------+
        |                |                |
        V                V                V
+--------------------------------------------------+
|                 Message Queue                    |
+--------------------------------------------------+
```

### Kubernetes 部署

适用于云原生环境，结合 Kubernetes 的服务发现和负载均衡：

```
+------------------+   +------------------+
|  API Service Pod |   |  API Service Pod |
|                  |   |                  |
|  +-----------+   |   |  +-----------+   |
|  | HTTP      |   |   |  | HTTP      |   |
|  | Service   |   |   |  | Service   |   |
|  +-----------+   |   |  +-----------+   |
+------------------+   +------------------+
         ^                     ^
         |                     |
         V                     V
+------------------------------------------+
|           Kubernetes Service             |
+------------------------------------------+
               ^
               |
               V
+------------------------------------------+
|              Ingress                     |
+------------------------------------------+
               ^
               |
               V
       External Traffic
```

## 配置管理

Sylph 框架支持多种配置方式，适应不同的应用场景：

### 环境变量配置

通过环境变量配置框架行为：

```go
// 从环境变量读取配置
logLevel := os.Getenv("SYLPH_LOG_LEVEL")
if logLevel == "" {
    logLevel = "info" // 默认值
}

// 创建日志配置
config := &LoggerConfig{
    Level: getLogLevelFromString(logLevel),
}
```

### 文件配置

通过配置文件配置框架行为：

```go
// 从配置文件读取配置
config, err := sylph.LoadConfig("config.yaml")
if err != nil {
    log.Fatal("加载配置失败:", err)
}

// 使用配置创建组件
server := sylph.NewGinServer(config.HTTP.Address)
```

### 代码配置

通过代码直接配置框架行为：

```go
// 创建自定义日志配置
loggerConfig := &sylph.LoggerConfig{
    Path:         "/var/log/app",
    Level:        logrus.InfoLevel,
    Format:       "json",
    ReportCaller: true,
    Async:        true,
    BufferSize:   1000,
}

// 创建日志记录器
logger := sylph.NewLogger("api-service", loggerConfig)
```

## 未来规划

Sylph 框架的未来发展方向：

1. **分布式追踪** - 集成 OpenTelemetry，提供分布式追踪能力
2. **服务网格集成** - 与 Istio 等服务网格解决方案集成
3. **更多数据库支持** - 提供更多数据库的集成
4. **更多消息队列支持** - 支持 Kafka、RabbitMQ 等消息队列
5. **更多缓存支持** - 提供 Redis、Memcached 等缓存的集成
6. **GraphQL 支持** - 提供 GraphQL API 的支持
7. **gRPC 支持** - 提供 gRPC 服务的支持 