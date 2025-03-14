# HTTP服务示例

这个示例展示了如何使用Sylph框架构建一个功能完整的RESTful API服务。

## 功能特点

- 基于Gin的HTTP服务器配置
- 中间件实现（请求日志记录）
- RESTful API设计（CRUD操作）
- 事件系统的使用（同步和异步事件）
- 结构化日志记录
- 定时任务
- 优雅启动和关闭

## 运行示例

确保已安装所有依赖：

```bash
go mod tidy
```

然后运行示例：

```bash
go run main.go
```

服务将在 http://localhost:8080 上启动，可以访问以下端点：

- `GET /health` - 健康检查
- `GET /api/users` - 获取所有用户
- `GET /api/users/:id` - 获取指定用户
- `POST /api/users` - 创建新用户
- `PUT /api/users/:id` - 更新用户
- `DELETE /api/users/:id` - 删除用户

## API测试

### 获取所有用户

```bash
curl http://localhost:8080/api/users
```

### 获取单个用户

```bash
curl http://localhost:8080/api/users/1
```

### 创建用户

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"王五","email":"wangwu@example.com"}'
```

### 更新用户

```bash
curl -X PUT http://localhost:8080/api/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"张三（已更新）","email":"zhangsan_updated@example.com"}'
```

### 删除用户

```bash
curl -X DELETE http://localhost:8080/api/users/2
```

## 核心代码解析

### 服务创建和配置

```go
// 创建HTTP服务器
server := sylph.NewGinServer(":8080")

// 创建并启动项目
project := sylph.NewProject()
project.Mounts(server)

// 添加一个示例的定时任务
cronServer := sylph.NewCronServer()
cronServer.AddJob("*/30 * * * * *", "活跃用户统计", func(ctx sylph.Context) {
    // 任务逻辑...
})
project.Mounts(cronServer)
```

### 中间件实现

```go
// 添加自定义中间件：请求日志记录
server.Use(func(c *gin.Context) {
    // 获取开始时间
    startTime := time.Now()
    
    // 从Gin上下文获取Sylph上下文
    ctx := sylph.FromGin(c)
    
    // 记录请求开始
    ctx.Info("middleware.request", "收到请求", sylph.H{
        "method": c.Request.Method,
        "path":   c.Request.URL.Path,
        "ip":     c.ClientIP(),
    })
    
    // 处理请求
    c.Next()
    
    // 记录请求结束
    ctx.Info("middleware.response", "请求处理完成", sylph.H{
        "method":     c.Request.Method,
        "path":       c.Request.URL.Path,
        "status":     c.Writer.Status(),
        "duration":   time.Since(startTime).String(),
        "durationMs": time.Since(startTime).Milliseconds(),
    })
})
```

### 事件系统使用

```go
// 同步事件
ctx.Emit("users.listed", sylph.H{
    "count": len(users),
})

// 异步事件
ctx.AsyncEmit("user.created", sylph.H{
    "userId": user.ID,
    "name":   user.Name,
    "email":  user.Email,
})
```

## 最佳实践

1. **上下文传递**：请确保在所有处理函数中都正确获取并使用Sylph上下文
2. **结构化日志**：使用模块名称、消息和结构化数据记录日志
3. **事件命名**：采用`对象.动作`格式命名事件，如`user.created`
4. **优雅关闭**：实现信号处理以确保服务可以优雅关闭
5. **API设计**：遵循RESTful API设计原则 