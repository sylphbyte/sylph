package sylph

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
)

var (
	// defaultManager 服务器管理器的单例实例
	// 通过GetManager()函数访问
	defaultManager IServerManager

	// once 确保单例模式的线程安全
	once sync.Once
)

// GetManager 获取默认的服务器管理器实例
// 使用单例模式，确保整个应用程序只有一个服务器管理器实例
//
// 返回:
//   - IServerManager: 全局服务器管理器实例
//
// 使用示例:
//
//	manager := sylph.GetManager()
//	err := manager.Register(myServer)
func GetManager() IServerManager {
	once.Do(func() {
		defaultManager = NewServerManager()
	})
	return defaultManager
}

// ErrServerNotFound 服务器未找到错误
// 当尝试获取不存在的服务器时返回此错误
var ErrServerNotFound = errors.New("server not found")

// ErrServerExists 服务器已存在错误
// 当尝试注册已存在名称的服务器时返回此错误
var ErrServerExists = errors.New("server already exists")

// ErrServerTypeMismatch 服务器类型不匹配错误
// 当尝试将一个服务器转换为不兼容的类型时返回此错误
var ErrServerTypeMismatch = errors.New("server type mismatch")

// IServerManager 服务器管理器接口
// 提供服务器注册、获取、启动和关闭的统一管理功能
type IServerManager interface {
	// Register 注册一个服务器
	// 将服务器实例添加到管理器中，以便后续使用
	//
	// 参数:
	//   - server: 要注册的服务器实例，必须实现IServer接口
	//
	// 返回:
	//   - error: 注册过程中的错误，如服务器已存在或参数无效
	Register(server IServer) error

	// GetServer 根据名称获取一个服务器
	// 返回注册时指定名称的服务器实例
	//
	// 参数:
	//   - name: 服务器名称
	//
	// 返回:
	//   - IServer: 服务器实例
	//   - error: 获取过程中的错误，如服务器不存在
	GetServer(name string) (IServer, error)

	// GetGinServer 获取一个Gin服务器
	// 返回指定名称的Gin服务器实例，自动进行类型转换
	//
	// 参数:
	//   - name: 服务器名称
	//
	// 返回:
	//   - *GinServer: Gin服务器实例
	//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
	GetGinServer(name string) (*GinServer, error)

	// GetCronServer 获取一个Cron服务器
	// 返回指定名称的Cron服务器实例，自动进行类型转换
	//
	// 参数:
	//   - name: 服务器名称
	//
	// 返回:
	//   - *CronServer: Cron服务器实例
	//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
	GetCronServer(name string) (*CronServer, error)

	// GetRocketServer 获取一个Rocket消费者服务器
	// 返回指定名称的RocketMQ消费者服务器实例，自动进行类型转换
	//
	// 参数:
	//   - name: 服务器名称
	//
	// 返回:
	//   - *RocketConsumerServer: RocketMQ消费者服务器实例
	//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
	GetRocketServer(name string) (*RocketConsumerServer, error)

	// GetRocketProducer 获取一个Rocket生产者
	// 返回指定名称的RocketMQ生产者实例，自动进行类型转换
	//
	// 参数:
	//   - name: 服务器名称
	//
	// 返回:
	//   - IProducer: RocketMQ生产者实例
	//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
	GetRocketProducer(name string) (IProducer, error)

	// BootAll 启动所有服务器
	// 依次启动所有已注册的服务器
	//
	// 返回:
	//   - error: 启动过程中的第一个错误，如有服务器启动失败则终止后续启动
	BootAll() error

	// ShutdownAll 关闭所有服务器
	// 依次关闭所有已注册的服务器
	//
	// 返回:
	//   - error: 关闭过程中的最后一个错误，即使有服务器关闭失败也会尝试关闭所有服务器
	ShutdownAll() error

	// GetAllServers 获取所有服务器
	// 返回所有已注册的服务器实例列表
	//
	// 返回:
	//   - []IServer: 服务器实例列表
	GetAllServers() []IServer
}

// NewServerManager 创建一个新的服务器管理器
// 初始化一个空的服务器管理器实例
//
// 返回:
//   - IServerManager: 新创建的服务器管理器实例
//
// 使用示例:
//
//	manager := sylph.NewServerManager()
//	err := manager.Register(myServer)
func NewServerManager() IServerManager {
	return &serverManager{
		servers: make(map[string]IServer),
	}
}

// serverManager 服务器管理器实现
// 实现IServerManager接口，提供服务器的注册、获取、启动和关闭功能
type serverManager struct {
	mutex   sync.RWMutex       // 读写锁，保护并发访问
	servers map[string]IServer // 服务器映射表，键为服务器名称，值为服务器实例
}

// Register 注册一个服务器
// 将服务器实例添加到管理器中，以便后续使用
//
// 参数:
//   - server: 要注册的服务器实例，必须实现IServer接口
//
// 返回:
//   - error: 注册过程中的错误，如服务器已存在或参数无效
//
// 注意事项:
//   - 服务器名称必须唯一，不能注册同名服务器
//   - 服务器实例不能为nil
func (sm *serverManager) Register(server IServer) error {
	if server == nil {
		return errors.New("server cannot be nil")
	}

	name := server.Name()
	if name == "" {
		return errors.New("server name cannot be empty")
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 检查是否已存在
	if _, exists := sm.servers[name]; exists {
		return errors.Wrapf(ErrServerExists, "server with name %s already registered", name)
	}

	// 注册服务器
	sm.servers[name] = server
	pr.System("Server registered: %s\n", name)

	return nil
}

// GetServer 根据名称获取一个服务器
// 返回注册时指定名称的服务器实例
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - IServer: 服务器实例
//   - error: 获取过程中的错误，如服务器不存在
func (sm *serverManager) GetServer(name string) (IServer, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	server, exists := sm.servers[name]
	if !exists {
		return nil, errors.Wrapf(ErrServerNotFound, "server with name %s not found", name)
	}

	return server, nil
}

// GetGinServer 获取一个Gin服务器
// 返回指定名称的Gin服务器实例，自动进行类型转换
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *GinServer: Gin服务器实例
//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
//
// 使用示例:
//
//	ginServer, err := manager.GetGinServer("web")
//	if err != nil {
//	    return err
//	}
//	ginServer.GET("/api/test", handleTest)
func (sm *serverManager) GetGinServer(name string) (*GinServer, error) {
	server, err := sm.GetServer(name)
	if err != nil {
		return nil, err
	}

	ginServer, ok := server.(*GinServer)
	if !ok {
		return nil, errors.Wrapf(ErrServerTypeMismatch, "server %s is not a GinServer", name)
	}

	return ginServer, nil
}

// GetCronServer 获取一个Cron服务器
// 返回指定名称的Cron服务器实例，自动进行类型转换
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *CronServer: Cron服务器实例
//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
//
// 使用示例:
//
//	cronServer, err := manager.GetCronServer("scheduler")
//	if err != nil {
//	    return err
//	}
//	cronServer.AddTask("0 * * * *", myTask)
func (sm *serverManager) GetCronServer(name string) (*CronServer, error) {
	server, err := sm.GetServer(name)
	if err != nil {
		return nil, err
	}

	cronServer, ok := server.(*CronServer)
	if !ok {
		return nil, errors.Wrapf(ErrServerTypeMismatch, "server %s is not a CronServer", name)
	}

	return cronServer, nil
}

// GetRocketServer 获取一个Rocket消费者服务器
// 返回指定名称的RocketMQ消费者服务器实例，自动进行类型转换
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *RocketConsumerServer: RocketMQ消费者服务器实例
//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
//
// 使用示例:
//
//	rocketServer, err := manager.GetRocketServer("consumer")
//	if err != nil {
//	    return err
//	}
//	// 使用RocketMQ消费者服务器
func (sm *serverManager) GetRocketServer(name string) (*RocketConsumerServer, error) {
	server, err := sm.GetServer(name)
	if err != nil {
		return nil, err
	}

	rocketServer, ok := server.(*RocketConsumerServer)
	if !ok {
		return nil, errors.Wrapf(ErrServerTypeMismatch, "server %s is not a RocketConsumerServer", name)
	}

	return rocketServer, nil
}

// GetRocketProducer 获取一个Rocket生产者
// 返回指定名称的RocketMQ生产者实例，自动进行类型转换
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - IProducer: RocketMQ生产者实例
//   - error: 获取过程中的错误，如服务器不存在或类型不匹配
//
// 使用示例:
//
//	producer, err := manager.GetRocketProducer("producer")
//	if err != nil {
//	    return err
//	}
//	err = producer.SendMsg(msg)
func (sm *serverManager) GetRocketProducer(name string) (IProducer, error) {
	server, err := sm.GetServer(name)
	if err != nil {
		return nil, err
	}

	producer, ok := server.(IProducer)
	if !ok {
		return nil, errors.Wrapf(ErrServerTypeMismatch, "server %s is not a IProducer", name)
	}

	return producer, nil
}

// BootAll 启动所有服务器
// 依次启动所有已注册的服务器
//
// 返回:
//   - error: 启动过程中的第一个错误，如有服务器启动失败则终止后续启动
//
// 注意事项:
//   - 如果某个服务器启动失败，后续服务器将不会被启动
//   - 启动操作是阻塞的，直到所有服务器都启动完成
func (sm *serverManager) BootAll() error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for name, server := range sm.servers {
		pr.System("Starting server: %s\n", name)
		if err := server.Boot(); err != nil {
			return errors.Wrapf(err, "failed to boot server %s", name)
		}
		pr.Green("Server started: %s\n", name)
	}

	return nil
}

// ShutdownAll 关闭所有服务器
// 依次关闭所有已注册的服务器
//
// 返回:
//   - error: 关闭过程中的最后一个错误，即使有服务器关闭失败也会尝试关闭所有服务器
//
// 注意事项:
//   - 即使某个服务器关闭失败，也会继续尝试关闭其他服务器
//   - 返回最后一个遇到的错误
//   - 关闭操作是阻塞的，直到所有服务器都关闭完成
func (sm *serverManager) ShutdownAll() error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var lastErr error
	for name, server := range sm.servers {
		pr.System("Shutting down server: %s\n", name)
		if err := server.Shutdown(); err != nil {
			lastErr = errors.Wrapf(err, "failed to shutdown server %s", name)
			pr.Error("Failed to shutdown server %s: %v\n", name, err)
		} else {
			pr.Green("Server shutdown: %s\n", name)
		}
	}

	return lastErr
}

// GetAllServers 获取所有服务器
// 返回所有已注册的服务器实例列表
//
// 返回:
//   - []IServer: 服务器实例列表
//
// 使用示例:
//
//	servers := manager.GetAllServers()
//	for _, server := range servers {
//	    fmt.Println(server.Name())
//	}
func (sm *serverManager) GetAllServers() []IServer {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	servers := make([]IServer, 0, len(sm.servers))
	for _, server := range sm.servers {
		servers = append(servers, server)
	}

	return servers
}

// MustGetServer 必须获取一个服务器，如果不存在则panic
// 是GetServer方法的包装，适用于确定服务器存在的场景
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - IServer: 服务器实例
//
// 注意事项:
//   - 如果服务器不存在，将导致程序panic
//   - 仅在确信服务器必定存在的情况下使用此方法
//
// 使用示例:
//
//	server := sylph.MustGetServer("web")
//	// 使用server，确信不会是nil
func MustGetServer(name string) IServer {
	server, err := GetManager().GetServer(name)
	if err != nil {
		panic(fmt.Sprintf("必须获取服务器失败: %v", err))
	}
	return server
}

// MustGetGinServer 必须获取一个Gin服务器，如果不存在则panic
// 是GetGinServer方法的包装，适用于确定Gin服务器存在的场景
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *GinServer: Gin服务器实例
//
// 注意事项:
//   - 如果服务器不存在或类型不匹配，将导致程序panic
//   - 仅在确信服务器存在且类型正确的情况下使用此方法
func MustGetGinServer(name string) *GinServer {
	server, err := GetManager().GetGinServer(name)
	if err != nil {
		panic(fmt.Sprintf("必须获取Gin服务器失败: %v", err))
	}
	return server
}

// MustGetCronServer 必须获取一个Cron服务器，如果不存在则panic
// 是GetCronServer方法的包装，适用于确定Cron服务器存在的场景
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *CronServer: Cron服务器实例
//
// 注意事项:
//   - 如果服务器不存在或类型不匹配，将导致程序panic
//   - 仅在确信服务器存在且类型正确的情况下使用此方法
func MustGetCronServer(name string) *CronServer {
	server, err := GetManager().GetCronServer(name)
	if err != nil {
		panic(fmt.Sprintf("必须获取Cron服务器失败: %v", err))
	}
	return server
}

// MustGetRocketServer 必须获取一个Rocket消费者服务器，如果不存在则panic
// 是GetRocketServer方法的包装，适用于确定RocketMQ消费者服务器存在的场景
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - *RocketConsumerServer: RocketMQ消费者服务器实例
//
// 注意事项:
//   - 如果服务器不存在或类型不匹配，将导致程序panic
//   - 仅在确信服务器存在且类型正确的情况下使用此方法
func MustGetRocketServer(name string) *RocketConsumerServer {
	server, err := GetManager().GetRocketServer(name)
	if err != nil {
		panic(fmt.Sprintf("必须获取RocketMQ消费者服务器失败: %v", err))
	}
	return server
}

// MustGetRocketProducer 必须获取一个Rocket生产者，如果不存在则panic
// 是GetRocketProducer方法的包装，适用于确定RocketMQ生产者存在的场景
//
// 参数:
//   - name: 服务器名称
//
// 返回:
//   - IProducer: RocketMQ生产者实例
//
// 注意事项:
//   - 如果服务器不存在或类型不匹配，将导致程序panic
//   - 仅在确信服务器存在且类型正确的情况下使用此方法
func MustGetRocketProducer(name string) IProducer {
	producer, err := GetManager().GetRocketProducer(name)
	if err != nil {
		panic(fmt.Sprintf("必须获取RocketMQ生产者失败: %v", err))
	}
	return producer
}
