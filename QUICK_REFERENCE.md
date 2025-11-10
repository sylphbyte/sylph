# Sylph 框架快速参考

## 📚 目录
1. [核心概念](#核心概念)
2. [快速开始](#快速开始)
3. [Context 使用](#context-使用)
4. [日志系统](#日志系统)
5. [存储管理](#存储管理)
6. [HTTP 服务](#http-服务)
7. [定时任务](#定时任务)
8. [消息队列](#消息队列)
9. [常见问题](#常见问题)

---

## 核心概念

### 1. Project - 项目容器
管理所有服务的生命周期

### 2. Server - 服务
实现 `IServer` 接口的服务实例（HTTP、Cron、RocketMQ）

### 3. Context - 上下文
请求级别的上下文，包含 TraceId、数据缓存、日志器

### 4. Storage - 存储
统一的存储抽象（MySQL、Redis、Elasticsearch）

---

## 快速开始

### 最小示例

```go
package main

import (
    "github.com/sylphbyte/sylph"
)

func main() {
    // 创建 HTTP 服务
    server := sylph.NewGinServer(
        sylph.WithPort(8080),
    )
    
    // 创建项目并启动
    project := sylph.NewProject()
    project.Mounts(server)
    
    if err := project.Boots(); err != nil {
        panic(err)
    }
    
    // 等待退出
    project.WaitForShutdown()
}
```

---

## Context 使用

### 创建 Context

```go
ctx := sylph.NewContext(sylph.HttpKind, "/api/users")
```

### 数据存取

```go
// 设置数据
ctx.Set("user_id", 123)
ctx.Set("username", "张三")

// 获取数据
userID, ok := ctx.Get("user_id")
if ok {
    id := userID.(int)
}
```

### 日志记录

```go
// 基础日志
ctx.Info("用户登录成功")
ctx.Error("处理失败", "error", err)

// 结构化日志
ctx.Info("订单创建", 
    "order_id", order.ID,
    "amount", order.Amount,
)
```

### Context 克隆

```go
// 克隆 Context（用于 goroutine）
newCtx := ctx.Clone()

go func() {
    newCtx.Info("异步处理")
}()
```

### 超时控制

```go
// 带超时的 Context
ctx, cancel := ctx.WithTimeout(5 * time.Second)
defer cancel()

select {
case <-ctx.Done():
    return ctx.Err() // 超时或取消
case result := <-ch:
    return result
}
```

---

## 日志系统

### 日志级别

```go
ctx.Debug("调试信息")
ctx.Info("普通信息")
ctx.Warn("警告信息")
ctx.Error("错误信息")
ctx.Fatal("致命错误") // 会终止程序
```

### 日志字段

```go
// 方式1: 键值对
ctx.Info("用户操作", 
    "action", "login",
    "user_id", 123,
    "ip", "1.2.3.4",
)

// 方式2: 结构化
ctx.WithFields(map[string]any{
    "action": "login",
    "user_id": 123,
}).Info("用户操作")
```

### 日志配置

```yaml
# logger.yaml
endpoints:
  api:
    level: info
    output: file
    file_path: /var/log/api.log
    max_size: 100
    max_backups: 10
```

---

## 存储管理

### 初始化存储

```go
storageManager, err := sylph.InitializeStorage(
    "./config/storage.yaml",
    map[string]bool{"main": true},  // MySQL
    map[string]bool{"main": true},  // Redis
    map[string]bool{"main": true},  // ES
)
```

### MySQL 使用

```go
// 获取数据库
db, err := storageManager.GetDB("main")
if err != nil {
    return err
}

// 直接使用 GORM
var user User
db.GetDB().Where("id = ?", 123).First(&user)

// 事务
err = db.WithTransaction(ctx, func(tx *gorm.DB) error {
    // 在事务中操作
    if err := tx.Create(&user).Error; err != nil {
        return err
    }
    return nil
})
```

### Redis 使用

```go
// 获取 Redis
rds, err := storageManager.GetRedis("main")
if err != nil {
    return err
}

// 直接使用 Redis 客户端
client := rds.GetClient()
client.Set(ctx, "key", "value", time.Hour)
val, err := client.Get(ctx, "key").Result()

// 管道操作
err = rds.WithPipeline(ctx, func(pipe redis.Pipeliner) error {
    pipe.Set(ctx, "key1", "val1", 0)
    pipe.Set(ctx, "key2", "val2", 0)
    _, err := pipe.Exec(ctx)
    return err
})
```

### Elasticsearch 使用

```go
// 获取 ES
es, err := storageManager.GetES("main")
if err != nil {
    return err
}

// 检查索引
exists, err := es.IndexExists(ctx, "users")

// 创建索引
mapping := `{"mappings": {...}}`
err = es.CreateIndex(ctx, "users", mapping)

// 直接使用 ES 客户端
client := es.GetClient()
res, err := client.Search(
    client.Search.WithIndex("users"),
    client.Search.WithBody(query),
)
```

### 健康检查

```go
// 检查所有存储健康状态
status := storageManager.HealthCheck(ctx)

for name, health := range status {
    fmt.Printf("%s: %s\n", name, health.State)
}
```

---

## HTTP 服务

### 创建 HTTP 服务

```go
server := sylph.NewGinServer(
    sylph.WithPort(8080),
    sylph.WithRoutes(
        sylph.GinRoute{
            Method:  "GET",
            Path:    "/api/users/:id",
            Handler: GetUser,
        },
        sylph.GinRoute{
            Method:  "POST",
            Path:    "/api/users",
            Handler: CreateUser,
        },
    ),
)
```

### 路由处理器

```go
func GetUser(c *gin.Context) {
    // 创建 Context
    ctx := sylph.NewContext(sylph.HttpKind, c.Request.URL.Path)
    
    // 获取参数
    userID := c.Param("id")
    
    // 业务逻辑
    user, err := userService.GetByID(ctx, userID)
    if err != nil {
        ctx.Error("查询用户失败", "error", err)
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    ctx.Info("查询用户成功", "user_id", userID)
    c.JSON(200, user)
}
```

### 中间件

```go
// 认证中间件
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        
        if token == "" {
            c.AbortWithStatus(401)
            return
        }
        
        // 验证 token
        userID, err := verifyToken(token)
        if err != nil {
            c.AbortWithStatus(401)
            return
        }
        
        c.Set("user_id", userID)
        c.Next()
    }
}

// 使用中间件
server.Engine().Use(AuthMiddleware())
```

---

## 定时任务

### 创建 Cron 服务

```go
server := sylph.NewCronServer(
    sylph.WithLocation("Asia/Shanghai"),
    sylph.WithJobs(
        sylph.CronJob{
            Name:     "daily-report",
            Spec:     "0 0 * * *",  // 每天零点
            Handler:  GenerateReport,
        },
        sylph.CronJob{
            Name:     "cleanup",
            Spec:     "0 2 * * *",  // 每天凌晨2点
            Handler:  CleanupOldData,
        },
    ),
)
```

### Cron 表达式

```
┌───────────── 分钟 (0 - 59)
│ ┌─────────── 小时 (0 - 23)
│ │ ┌───────── 日期 (1 - 31)
│ │ │ ┌─────── 月份 (1 - 12)
│ │ │ │ ┌───── 星期 (0 - 6, 0是周日)
│ │ │ │ │
* * * * *
```

常用示例：
- `0 0 * * *` - 每天零点
- `*/5 * * * *` - 每5分钟
- `0 */2 * * *` - 每2小时
- `0 0 * * 0` - 每周日零点
- `0 0 1 * *` - 每月1号零点

### 任务处理器

```go
func GenerateReport(ctx sylph.Context) error {
    ctx.Info("开始生成报表")
    
    // 业务逻辑
    report, err := reportService.Generate(ctx)
    if err != nil {
        ctx.Error("生成报表失败", "error", err)
        return err
    }
    
    ctx.Info("报表生成成功", "report_id", report.ID)
    return nil
}
```

---

## 消息队列

### 创建 RocketMQ 服务

```go
// 创建消费者服务
server := sylph.NewRocketServer(
    sylph.WithRocketEndpoints("127.0.0.1:9876"),
    sylph.WithRocketTopic("order-events"),
    sylph.WithRocketGroup("order-consumer-group"),
    sylph.WithRocketHandlers(
        sylph.RocketHandler{
            Tag:     "OrderCreated",
            Handler: HandleOrderCreated,
        },
        sylph.RocketHandler{
            Tag:     "OrderPaid",
            Handler: HandleOrderPaid,
        },
    ),
)
```

### 消息处理器

```go
func HandleOrderCreated(ctx sylph.Context, msg *sylph.RocketMessage) error {
    ctx.Info("收到订单创建消息", "msg_id", msg.GetMessageId())
    
    // 解析消息
    var order Order
    if err := json.Unmarshal(msg.GetBody(), &order); err != nil {
        ctx.Error("解析消息失败", "error", err)
        return err
    }
    
    // 业务处理
    if err := orderService.Process(ctx, &order); err != nil {
        ctx.Error("处理订单失败", "error", err)
        return err // 返回错误会触发重试
    }
    
    ctx.Info("订单处理成功", "order_id", order.ID)
    return nil
}
```

### 发送消息

```go
// 创建生产者
producer := sylph.NewRocketProducer(
    sylph.WithProducerEndpoints("127.0.0.1:9876"),
    sylph.WithProducerGroup("order-producer-group"),
)

// 发送消息
msg := &sylph.RocketMessage{
    Topic: "order-events",
    Tag:   "OrderCreated",
    Body:  orderJSON,
}

msgID, err := producer.Send(ctx, msg)
if err != nil {
    return err
}

ctx.Info("消息发送成功", "msg_id", msgID)
```

---

## 常见问题

### 1. 如何在 goroutine 中使用 Context?

```go
// ❌ 错误：直接使用会有并发问题
go func() {
    ctx.Info("异步处理")
}()

// ✅ 正确：先克隆
newCtx := ctx.Clone()
go func() {
    newCtx.Info("异步处理")
}()
```

### 2. 如何处理事务？

```go
err := db.WithTransaction(ctx, func(tx *gorm.DB) error {
    // 所有操作都在事务中
    if err := tx.Create(&order).Error; err != nil {
        return err // 自动回滚
    }
    
    if err := tx.Create(&orderItem).Error; err != nil {
        return err // 自动回滚
    }
    
    return nil // 自动提交
})
```

### 3. 如何设置日志级别？

```go
// 通过配置文件
logger:
  level: info  # debug, info, warn, error

// 或在代码中
logger.SetLevel(logrus.InfoLevel)
```

### 4. 如何优雅关闭？

```go
// Project 自动处理优雅关闭
project := sylph.NewProject(
    sylph.WithShutdownTimeout(10 * time.Second),
)

// 等待退出信号
project.WaitForShutdown()

// 或手动关闭
if err := project.Shutdowns(); err != nil {
    log.Error(err)
}
```

### 5. 如何添加中间件？

```go
// HTTP 中间件
server.Engine().Use(gin.Logger())
server.Engine().Use(gin.Recovery())
server.Engine().Use(CustomMiddleware())

// 或在创建时添加
server := sylph.NewGinServer(
    sylph.WithMiddlewares(
        gin.Logger(),
        gin.Recovery(),
    ),
)
```

### 6. 如何处理超时？

```go
// Context 超时
ctx, cancel := ctx.WithTimeout(5 * time.Second)
defer cancel()

// HTTP 请求超时
client := &http.Client{
    Timeout: 10 * time.Second,
}

// 数据库查询超时
ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
defer cancel()
db.WithContext(ctx).Find(&users)
```

### 7. 如何查看健康状态？

```go
// 存储健康检查
status := storageManager.HealthCheck(ctx)

for name, health := range status {
    fmt.Printf("%s: %s (latency: %dms)\n", 
        name, health.State, health.Latency)
}
```

### 8. 如何配置连接池？

```yaml
# storage.yaml
mysql_group:
  main:
    max_idle_conn: 32   # 最大空闲连接
    max_open_conn: 64   # 最大打开连接
    max_life_time: 1800 # 连接最大生命周期(秒)
```

---

## 性能提示

### 1. 使用异步日志
日志系统默认是异步的，不会阻塞业务

### 2. 复用 Context
在同一请求内复用 Context，避免重复创建

### 3. 使用连接池
合理配置数据库和 Redis 连接池大小

### 4. 批量操作
使用事务或批量操作减少网络往返

### 5. 缓存频繁查询
将热点数据缓存到 Redis

---

## 配置示例

### storage.yaml

```yaml
mysql_group:
  main:
    debug: true
    log_mode: 4
    host: 127.0.0.1
    port: 3306
    username: root
    password: password
    database: mydb
    charset: utf8mb4
    max_idle_conn: 32
    max_open_conn: 64
    max_life_time: 1800

redis_group:
  main:
    host: 127.0.0.1
    port: 6379
    password: ""
    database: 0

es_group:
  main:
    addresses:
      - http://127.0.0.1:9200
    username: ""
    password: ""
    max_retries: 3
    retry_timeout: 3
```

---

**快速上手 Sylph 框架！** 🚀

更多信息请查看：
- [框架分析](./FRAMEWORK_ANALYSIS.md)
- [架构文档](./ARCHITECTURE.md)

