# 服务器管理器使用示例

## 基本用法

服务器管理器提供了统一管理所有服务器的接口，支持注册、获取、启动和关闭服务器。

### 获取管理器实例

```go
// 获取全局单例管理器
manager := sylph.GetManager()
```

### 注册服务器

```go
// 注册Gin服务器
ginServer := sylph.NewGinServer(sylph.GinOption{
    Name: "api",
    Host: "0.0.0.0",
    Port: 8080,
})
err := manager.Register(ginServer)
if err != nil {
    // 处理错误
}

// 注册Cron服务器
ctx := sylph.NewDefaultContext("cron", "cron-jobs")
cronServer := sylph.NewCronServer(ctx, sylph.CrontabNormalMode, configs)
err = manager.Register(cronServer)
if err != nil {
    // 处理错误
}

// 注册RocketMQ服务器
rocketServer := sylph.NewRocketConsumerServer(ctx, consumer, instance)
err = manager.Register(rocketServer)
if err != nil {
    // 处理错误
}
```

### 获取服务器

```go
// 根据名称获取服务器
server, err := manager.GetServer("api")
if err != nil {
    // 处理错误
}

// 获取特定类型的服务器
ginServer, err := manager.GetGinServer("api")
if err != nil {
    // 处理错误
}

cronServer, err := manager.GetCronServer("cron-server")
if err != nil {
    // 处理错误
}

rocketServer, err := manager.GetRocketServer("rocket-consumer")
if err != nil {
    // 处理错误
}

// 使用Must系列函数（如果获取失败会panic）
ginServer := sylph.MustGetGinServer("api")
```

### 启动和关闭所有服务器

```go
// 启动所有注册的服务器
err := manager.BootAll()
if err != nil {
    // 处理错误
}

// 关闭所有服务器
err = manager.ShutdownAll()
if err != nil {
    // 处理错误
}
```

## 项目集成示例

在项目中使用服务器管理器的典型流程：

```go
func main() {
    // 创建上下文
    ctx := sylph.NewDefaultContext("main", "app")
    
    // 获取服务器管理器
    manager := sylph.GetManager()
    
    // 加载配置
    configs := loadConfigs()
    
    // 注册HTTP服务器
    for _, opt := range configs.Http {
        server := sylph.NewGinServer(opt)
        registerRoutes(server) // 注册路由
        manager.Register(server)
    }
    
    // 注册定时任务服务器
    cronServer := sylph.NewCronServer(ctx, sylph.CrontabNormalMode, configs.Cron)
    registerCronTasks(cronServer) // 注册定时任务
    manager.Register(cronServer)
    
    // 注册消息队列服务器
    for _, consumer := range configs.Rocket.Consumers {
        server := sylph.NewRocketConsumerServer(ctx, consumer, configs.Rocket.Instance)
        registerRocketTasks(server) // 注册消息处理任务
        manager.Register(server)
    }
    
    // 启动所有服务器
    if err := manager.BootAll(); err != nil {
        log.Fatalf("启动服务器失败: %v", err)
    }
    
    // 等待信号优雅关闭
    waitForSignal()
    
    // 关闭所有服务器
    if err := manager.ShutdownAll(); err != nil {
        log.Errorf("关闭服务器失败: %v", err)
    }
}
``` 