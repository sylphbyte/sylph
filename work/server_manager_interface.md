# 服务器管理器接口设计

## 需求
创建一个服务器管理器，能够：
1. 注册和管理不同类型的服务器（GinServer, CronServer, RocketServer, RocketProducer）
2. 通过名称获取服务器实例
3. 支持启动和关闭所有注册的服务器

## 接口定义
```go
// IServerManager 服务器管理器接口
type IServerManager interface {
    // Register 注册一个服务器
    Register(server IServer) error
    
    // GetServer 根据名称获取一个服务器
    GetServer(name string) (IServer, error)
    
    // GetGinServer 获取一个Gin服务器
    GetGinServer(name string) (*GinServer, error)
    
    // GetCronServer 获取一个Cron服务器
    GetCronServer(name string) (*CronServer, error)
    
    // GetRocketServer 获取一个Rocket消费者服务器
    GetRocketServer(name string) (*RocketConsumerServer, error)
    
    // GetRocketProducer 获取一个Rocket生产者
    GetRocketProducer(name string) (IProducer, error)
    
    // BootAll 启动所有服务器
    BootAll() error
    
    // ShutdownAll 关闭所有服务器
    ShutdownAll() error
    
    // GetAllServers 获取所有服务器
    GetAllServers() []IServer
}
```

## 实现思路
创建一个结构体实现上述接口，内部维护一个映射表来存储服务器实例，使用互斥锁保证并发安全。 