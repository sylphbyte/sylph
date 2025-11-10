# Sylph 框架架构文档

## 📐 模块依赖关系图

```mermaid
graph TB
    subgraph "应用层"
        APP[业务应用]
    end
    
    subgraph "框架核心层"
        PROJECT[Project<br/>项目管理]
        SERVMGR[ServerManager<br/>服务管理器]
        CONTEXT[Context<br/>上下文]
    end
    
    subgraph "服务层"
        HTTP[GinServer<br/>HTTP服务]
        CRON[CronServer<br/>定时任务]
        ROCKET[RocketServer<br/>消息队列]
    end
    
    subgraph "基础设施层"
        LOGGER[Logger<br/>日志系统]
        STORAGE[StorageManager<br/>存储管理]
        UTILS[Utils<br/>工具类]
    end
    
    subgraph "第三方依赖"
        GIN[Gin框架]
        CRONLIB[Cron库]
        RMQ[RocketMQ SDK]
        GORM[GORM]
        REDIS[Redis SDK]
        ES[ES SDK]
    end
    
    APP --> PROJECT
    APP --> CONTEXT
    
    PROJECT --> SERVMGR
    PROJECT --> HTTP
    PROJECT --> CRON
    PROJECT --> ROCKET
    
    SERVMGR --> HTTP
    SERVMGR --> CRON
    SERVMGR --> ROCKET
    
    HTTP --> CONTEXT
    CRON --> CONTEXT
    ROCKET --> CONTEXT
    
    CONTEXT --> LOGGER
    CONTEXT --> STORAGE
    
    HTTP --> GIN
    CRON --> CRONLIB
    ROCKET --> RMQ
    
    STORAGE --> GORM
    STORAGE --> REDIS
    STORAGE --> ES
    
    LOGGER --> UTILS
    STORAGE --> UTILS
    
    style PROJECT fill:#4CAF50
    style SERVMGR fill:#4CAF50
    style CONTEXT fill:#4CAF50
    style LOGGER fill:#2196F3
    style STORAGE fill:#2196F3
    style HTTP fill:#FF9800
    style CRON fill:#FF9800
    style ROCKET fill:#FF9800
```

## 🔄 核心流程图

### 1. 服务启动流程

```mermaid
sequenceDiagram
    participant Main as 主程序
    participant Proj as Project
    participant SrvMgr as ServerManager
    participant Http as HttpServer
    participant Cron as CronServer
    participant Rocket as RocketServer
    
    Main->>Proj: NewProject()
    Main->>Proj: Mounts(servers...)
    
    Proj->>SrvMgr: 注册服务
    
    Main->>Proj: Boots()
    
    alt 并行启动
        par
            Proj->>Http: Boot()
            Http-->>Proj: 启动成功
        and
            Proj->>Cron: Boot()
            Cron-->>Proj: 启动成功
        and
            Proj->>Rocket: Boot()
            Rocket-->>Proj: 启动成功
        end
    else 串行启动
        Proj->>Http: Boot()
        Http-->>Proj: 启动成功
        Proj->>Cron: Boot()
        Cron-->>Proj: 启动成功
        Proj->>Rocket: Boot()
        Rocket-->>Proj: 启动成功
    end
    
    Proj-->>Main: 全部启动完成
```

### 2. HTTP 请求处理流程

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Gin as Gin引擎
    participant Middleware as 中间件
    participant Handler as 业务处理器
    participant Ctx as Context
    participant Logger as 日志系统
    participant Storage as 存储管理
    
    Client->>Gin: HTTP请求
    Gin->>Middleware: 拦截请求
    
    Middleware->>Ctx: NewContext()
    Note over Ctx: 生成 TraceId<br/>绑定日志器
    
    Middleware->>Handler: 调用处理器(ctx)
    
    Handler->>Ctx: 获取数据
    Handler->>Logger: 记录日志
    Handler->>Storage: 数据库操作
    
    Storage-->>Handler: 返回结果
    Handler-->>Middleware: 返回响应
    
    Middleware->>Logger: 记录访问日志
    Middleware-->>Gin: 返回响应
    Gin-->>Client: HTTP响应
```

### 3. 日志处理流程

```mermaid
sequenceDiagram
    participant App as 应用代码
    participant Logger as Logger
    participant Async as AsyncLogger
    participant Buffer as 缓冲队列
    participant Writer as WriterManager
    participant File as 文件系统
    
    App->>Logger: Info(msg, fields)
    Logger->>Async: 写入日志
    
    Async->>Buffer: 放入缓冲
    Note over Buffer: 批量缓冲<br/>异步处理
    
    alt 缓冲满或定时触发
        Async->>Buffer: 取出日志批次
        Async->>Writer: BatchWrite(logs)
        
        par 多写入器
            Writer->>File: 写入文件
        and
            Writer->>File: 写入控制台
        end
    end
    
    File-->>Writer: 写入完成
    Writer-->>Async: 刷新完成
```

### 4. 存储访问流程

```mermaid
sequenceDiagram
    participant App as 应用代码
    participant Ctx as Context
    participant StoMgr as StorageManager
    participant Adapter as StorageAdapter
    participant DB as 数据库
    
    App->>StoMgr: GetDB("main")
    StoMgr-->>App: DBStorage
    
    App->>Adapter: WithTransaction(fn)
    Adapter->>DB: BEGIN
    
    App->>DB: 执行SQL
    DB-->>App: 返回结果
    
    alt 成功
        Adapter->>DB: COMMIT
    else 失败
        Adapter->>DB: ROLLBACK
    end
    
    Adapter->>Ctx: 记录日志
    Adapter-->>App: 返回结果
```

## 🎨 模块详细设计

### 1. Project 模块

```go
// 项目管理核心
type Project struct {
    servers          []IServer       // 服务列表
    shutdownTimeout  time.Duration   // 关闭超时
    bootTimeout      time.Duration   // 启动超时
    orderedExecution bool            // 串行/并行
    state            atomic.Int32    // 原子状态
    mu               sync.RWMutex    // 读写锁
}

// 状态机
StateStopped -> StateStarting -> StateRunning -> StateStopping -> StateStopped
```

**设计亮点:**
- ✅ 原子操作保证状态安全
- ✅ 优雅关闭机制
- ✅ 灵活的启动策略
- ✅ 完善的错误处理

### 2. Context 模块

```go
// 请求上下文
type DefaultContext struct {
    ctxInternal context.Context    // 标准上下文
    dataCache   map[string]any     // 数据缓存
    rwMutex     sync.RWMutex       // 读写锁
    Header      *Header            // 请求头
    logger      *Logger            // 日志器
}
```

**设计亮点:**
- ✅ 线程安全的数据存储
- ✅ 支持标准 context.Context
- ✅ 内置日志绑定
- ✅ Clone 支持

### 3. Logger 模块

```go
// 异步日志核心
type AsyncLogger struct {
    buffer       chan *LoggerMessage  // 异步缓冲
    flushTicker  *time.Ticker        // 定时刷新
    batchSize    int                 // 批量大小
    writers      []io.Writer         // 多写入器
}

// 日志管理器
type LoggerManager struct {
    loggers map[Endpoint]*Logger    // 端点隔离
    builder *LoggerBuilder          // 构建器
}
```

**设计亮点:**
- ✅ 异步批量写入
- ✅ 多端点隔离
- ✅ 灵活的格式化
- ✅ 可扩展的钩子

### 4. Storage 模块

```go
// 存储管理器
type StorageManagerImpl struct {
    dbMap    map[string]DBStorage    // MySQL实例
    redisMap map[string]RedisStorage // Redis实例
    esMap    map[string]ESStorage    // ES实例
    mu       sync.RWMutex            // 读写锁
}

// 统一的存储接口
type Storage interface {
    GetType() StorageType
    GetName() string
    IsConnected() bool
    Connect(ctx Context) error
    Disconnect(ctx Context) error
    Ping(ctx Context) error
    GetHealthStatus() *HealthStatus
}
```

**设计亮点:**
- ✅ 统一的抽象接口
- ✅ 多实例管理
- ✅ 健康检查机制
- ✅ 连接池管理

## 🔐 设计模式应用

### 1. **接口驱动设计**
```go
// 统一的服务接口
type IServer interface {
    Name() string
    Boot() error
    Shutdown() error
}
```

### 2. **函数式选项模式**
```go
// 灵活的配置
project := NewProject(
    WithShutdownTimeout(10*time.Second),
    WithBootTimeout(30*time.Second),
    WithOrderedExecution(true),
)
```

### 3. **适配器模式**
```go
// 存储适配器
type MysqlStorageAdapter struct {
    name string
    db   *gorm.DB
}

func (a *MysqlStorageAdapter) GetDB() *gorm.DB {
    return a.db
}
```

### 4. **构建器模式**
```go
// 日志构建器
logger := NewLoggerBuilder().
    WithEndpoint("api").
    WithLevel(logrus.InfoLevel).
    WithOutput(os.Stdout).
    Build()
```

### 5. **管理器模式**
```go
// 统一管理
type StorageManager interface {
    GetDB(name ...string) (DBStorage, error)
    GetRedis(name ...string) (RedisStorage, error)
    GetES(name ...string) (ESStorage, error)
}
```

## 📊 性能优化策略

### 1. **异步处理**
- 日志异步批量写入
- 消息异步处理
- 事件异步分发

### 2. **连接池管理**
- MySQL 连接池
- Redis 连接池
- HTTP 连接复用

### 3. **内存优化**
- ✅ 已移除 sync.Pool（不必要的优化）
- 使用 sync.RWMutex 减少锁竞争
- 批量操作减少内存分配

### 4. **并发控制**
- goroutine 池管理
- 信号量控制
- Context 超时控制

## 🔒 线程安全保证

### 1. **Context 数据访问**
```go
// 使用 RWMutex 保护
type DefaultContext struct {
    dataCache map[string]any
    rwMutex   sync.RWMutex
}

func (d *DefaultContext) Set(key string, value any) {
    d.rwMutex.Lock()
    defer d.rwMutex.Unlock()
    d.dataCache[key] = value
}
```

### 2. **服务状态管理**
```go
// 使用 atomic 原子操作
type Project struct {
    state atomic.Int32
}

func (p *Project) setState(state State) {
    atomic.StoreInt32(&p.state, int32(state))
}
```

### 3. **存储管理器**
```go
// 使用 RWMutex 保护映射
type StorageManagerImpl struct {
    dbMap map[string]DBStorage
    mu    sync.RWMutex
}
```

## 🚀 扩展指南

### 1. **添加新的服务类型**

```go
// 1. 实现 IServer 接口
type MyServer struct {
    // 你的字段
}

func (s *MyServer) Name() string {
    return "my-server"
}

func (s *MyServer) Boot() error {
    // 启动逻辑
    return nil
}

func (s *MyServer) Shutdown() error {
    // 关闭逻辑
    return nil
}

// 2. 注册到 Project
project.Mounts(myServer)
```

### 2. **添加新的存储类型**

```go
// 1. 实现 Storage 接口
type MongoStorage struct {
    // MongoDB 相关
}

// 2. 实现接口方法
func (m *MongoStorage) GetType() StorageType {
    return "mongodb"
}

// 3. 注册到 StorageManager
storageManager.Register("main", mongoStorage)
```

### 3. **自定义日志格式化**

```go
// 实现 Formatter 接口
type MyFormatter struct {}

func (f *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
    // 自定义格式化
    return []byte(entry.Message), nil
}

// 使用自定义格式化
logger.SetFormatter(&MyFormatter{})
```

## 📝 使用示例

### 完整应用示例

```go
package main

import (
    "github.com/sylphbyte/sylph"
    "log"
)

func main() {
    // 1. 初始化存储
    storage, err := sylph.InitializeStorage(
        "./config/storage.yaml",
        map[string]bool{"main": true},
        map[string]bool{"main": true},
        map[string]bool{"main": true},
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. 创建 HTTP 服务
    httpServer := sylph.NewGinServer(
        sylph.WithPort(8080),
        sylph.WithRoutes(routes...),
    )
    
    // 3. 创建 Cron 服务
    cronServer := sylph.NewCronServer(
        sylph.WithLocation("Asia/Shanghai"),
        sylph.WithJobs(jobs...),
    )
    
    // 4. 创建项目
    project := sylph.NewProject(
        sylph.WithShutdownTimeout(10 * time.Second),
    )
    
    // 5. 装载服务
    project.Mounts(httpServer, cronServer)
    
    // 6. 启动服务
    if err := project.Boots(); err != nil {
        log.Fatal(err)
    }
    
    // 7. 等待退出信号
    project.WaitForShutdown()
    
    // 8. 优雅关闭
    if err := project.Shutdowns(); err != nil {
        log.Error(err)
    }
}
```

---

**Sylph 框架提供了清晰的架构和灵活的扩展能力，是构建微服务应用的理想选择！** 🎉

