package sylph

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Mock IServer 实现
// =============================================================================

type smMockServer struct {
	name      string
	bootErr   error
	shutErr   error
	bootCalls int
	shutCalls int
	mu        sync.Mutex
}

func newSMMockServer(name string) *smMockServer {
	return &smMockServer{name: name}
}

func (m *smMockServer) Name() string {
	return m.name
}

func (m *smMockServer) Boot() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bootCalls++
	return m.bootErr
}

func (m *smMockServer) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutCalls++
	return m.shutErr
}

func (m *smMockServer) setBootError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bootErr = err
}

func (m *smMockServer) setShutdownError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutErr = err
}

func (m *smMockServer) getBootCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bootCalls
}

func (m *smMockServer) getShutdownCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shutCalls
}

// =============================================================================
// 基础功能测试
// =============================================================================

// TestSMNewServerManager 测试创建 ServerManager
func TestSMNewServerManager(t *testing.T) {
	manager := NewServerManager()

	assert.NotNil(t, manager)
	assert.Empty(t, manager.GetAllServers())
}

// TestServerManagerRegister 测试注册服务器
func TestServerManagerRegister(t *testing.T) {
	manager := NewServerManager()
	server := newSMMockServer("test-server")

	err := manager.Register(server)
	assert.NoError(t, err)

	// 验证注册成功
	servers := manager.GetAllServers()
	assert.Len(t, servers, 1)
	assert.Equal(t, "test-server", servers[0].Name())
}

// TestServerManagerRegisterDuplicate 测试重复注册
func TestServerManagerRegisterDuplicate(t *testing.T) {
	manager := NewServerManager()
	server1 := newSMMockServer("duplicate")
	server2 := newSMMockServer("duplicate")

	// 第一次注册成功
	err := manager.Register(server1)
	assert.NoError(t, err)

	// 第二次注册应该失败
	err = manager.Register(server2)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrServerExists)
}

// TestServerManagerRegisterNil 测试注册 nil 服务器
func TestServerManagerRegisterNil(t *testing.T) {
	manager := NewServerManager()

	err := manager.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestServerManagerRegisterEmptyName 测试注册空名称服务器
func TestServerManagerRegisterEmptyName(t *testing.T) {
	manager := NewServerManager()
	server := newSMMockServer("")

	err := manager.Register(server)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

// TestServerManagerGetServer 测试获取服务器
func TestServerManagerGetServer(t *testing.T) {
	manager := NewServerManager()
	server := newSMMockServer("test")

	manager.Register(server)

	// 获取存在的服务器
	retrieved, err := manager.GetServer("test")
	assert.NoError(t, err)
	assert.Equal(t, server, retrieved)
}

// TestServerManagerGetServerNotFound 测试获取不存在的服务器
func TestServerManagerGetServerNotFound(t *testing.T) {
	manager := NewServerManager()

	retrieved, err := manager.GetServer("non-existent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
	assert.Nil(t, retrieved)
}

// TestServerManagerGetAllServersEmpty 测试获取空服务器列表
func TestServerManagerGetAllServersEmpty(t *testing.T) {
	manager := NewServerManager()

	servers := manager.GetAllServers()
	assert.NotNil(t, servers)
	assert.Empty(t, servers)
}

// TestServerManagerGetAllServersMultiple 测试获取多个服务器
func TestServerManagerGetAllServersMultiple(t *testing.T) {
	manager := NewServerManager()

	server1 := newSMMockServer("server1")
	server2 := newSMMockServer("server2")
	server3 := newSMMockServer("server3")

	manager.Register(server1)
	manager.Register(server2)
	manager.Register(server3)

	servers := manager.GetAllServers()
	assert.Len(t, servers, 3)

	// 验证所有服务器都在列表中
	names := make(map[string]bool)
	for _, s := range servers {
		names[s.Name()] = true
	}
	assert.True(t, names["server1"])
	assert.True(t, names["server2"])
	assert.True(t, names["server3"])
}

// =============================================================================
// 类型安全 Getter 测试
// =============================================================================

// TestServerManagerGetGinServer 测试获取 GinServer
func TestServerManagerGetGinServer(t *testing.T) {
	manager := NewServerManager()

	// 创建真实的 GinServer
	ginOpt := GinOption{
		Name: "gin-server",
		Host: "127.0.0.1",
		Port: 8080,
	}
	ginServer := NewGinServer(ginOpt)
	manager.Register(ginServer)

	// 获取 GinServer
	retrieved, err := manager.GetGinServer("gin-server")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, ginServer, retrieved)
}

// TestServerManagerGetGinServerTypeMismatch 测试类型不匹配
func TestServerManagerGetGinServerTypeMismatch(t *testing.T) {
	manager := NewServerManager()
	mockSrv := newSMMockServer("mock")
	manager.Register(mockSrv)

	// 尝试获取为 GinServer
	retrieved, err := manager.GetGinServer("mock")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrServerTypeMismatch)
	assert.Nil(t, retrieved)
}

// TestServerManagerGetGinServerNotFound 测试获取不存在的 GinServer
func TestServerManagerGetGinServerNotFound(t *testing.T) {
	manager := NewServerManager()

	retrieved, err := manager.GetGinServer("non-existent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
	assert.Nil(t, retrieved)
}

// TestServerManagerGetCronServer 测试获取 CronServer
func TestServerManagerGetCronServer(t *testing.T) {
	manager := NewServerManager()
	ctx := NewContext(Endpoint("cron"), "/test")

	// 创建真实的 CronServer
	cronServer := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})
	manager.Register(cronServer)

	// 获取 CronServer
	retrieved, err := manager.GetCronServer("cron-server")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, cronServer, retrieved)
}

// TestServerManagerGetCronServerTypeMismatch 测试 CronServer 类型不匹配
func TestServerManagerGetCronServerTypeMismatch(t *testing.T) {
	manager := NewServerManager()
	mockSrv := newSMMockServer("mock")
	manager.Register(mockSrv)

	// 尝试获取为 CronServer
	retrieved, err := manager.GetCronServer("mock")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrServerTypeMismatch)
	assert.Nil(t, retrieved)
}

// =============================================================================
// 批量操作测试
// =============================================================================

// TestServerManagerBootAll 测试启动所有服务器
func TestServerManagerBootAll(t *testing.T) {
	manager := NewServerManager()

	server1 := newSMMockServer("server1")
	server2 := newSMMockServer("server2")
	server3 := newSMMockServer("server3")

	manager.Register(server1)
	manager.Register(server2)
	manager.Register(server3)

	// 启动所有服务器
	err := manager.BootAll()
	assert.NoError(t, err)

	// 验证所有服务器都被调用了 Boot
	assert.Equal(t, 1, server1.getBootCalls())
	assert.Equal(t, 1, server2.getBootCalls())
	assert.Equal(t, 1, server3.getBootCalls())
}

// TestServerManagerBootAllWithError 测试启动失败
func TestServerManagerBootAllWithError(t *testing.T) {
	manager := NewServerManager()

	server1 := newSMMockServer("server1")
	server2 := newSMMockServer("server2")
	server3 := newSMMockServer("server3")

	// server2 设置启动失败
	server2.setBootError(errors.New("boot failed"))

	manager.Register(server1)
	manager.Register(server2)
	manager.Register(server3)

	// 启动应该失败
	err := manager.BootAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boot failed")

	// 由于启动失败，server3 可能没被启动（取决于 map 遍历顺序）
	// 只验证 server1 和 server2 被调用
	assert.GreaterOrEqual(t, server1.getBootCalls()+server2.getBootCalls(), 2)
}

// TestServerManagerShutdownAll 测试关闭所有服务器
func TestServerManagerShutdownAll(t *testing.T) {
	manager := NewServerManager()

	server1 := newSMMockServer("server1")
	server2 := newSMMockServer("server2")
	server3 := newSMMockServer("server3")

	manager.Register(server1)
	manager.Register(server2)
	manager.Register(server3)

	// 关闭所有服务器
	err := manager.ShutdownAll()
	assert.NoError(t, err)

	// 验证所有服务器都被调用了 Shutdown
	assert.Equal(t, 1, server1.getShutdownCalls())
	assert.Equal(t, 1, server2.getShutdownCalls())
	assert.Equal(t, 1, server3.getShutdownCalls())
}

// TestServerManagerShutdownAllWithError 测试关闭失败（继续执行）
func TestServerManagerShutdownAllWithError(t *testing.T) {
	manager := NewServerManager()

	server1 := newSMMockServer("server1")
	server2 := newSMMockServer("server2")
	server3 := newSMMockServer("server3")

	// server2 设置关闭失败
	server2.setShutdownError(errors.New("shutdown failed"))

	manager.Register(server1)
	manager.Register(server2)
	manager.Register(server3)

	// 关闭应该返回错误，但会继续执行
	err := manager.ShutdownAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown failed")

	// 所有服务器都应该被调用了 Shutdown（即使有错误也继续）
	assert.Equal(t, 1, server1.getShutdownCalls())
	assert.Equal(t, 1, server2.getShutdownCalls())
	assert.Equal(t, 1, server3.getShutdownCalls())
}

// =============================================================================
// 单例模式测试
// =============================================================================

// TestSMGetManager 测试单例模式
func TestSMGetManager(t *testing.T) {
	// 获取两次
	manager1 := GetManager()
	manager2 := GetManager()

	// 应该是同一个实例
	assert.NotNil(t, manager1)
	assert.NotNil(t, manager2)
	assert.Same(t, manager1, manager2)
}

// =============================================================================
// Must* 系列测试
// =============================================================================

// TestMustGetServerSuccess 测试 MustGetServer 成功
func TestMustGetServerSuccess(t *testing.T) {
	// 注意：这里使用 NewServerManager 而不是 GetManager
	// 因为 MustGetServer 使用全局单例
	manager := GetManager()
	server := newSMMockServer("must-test")
	manager.Register(server)

	assert.NotPanics(t, func() {
		retrieved := MustGetServer("must-test")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "must-test", retrieved.Name())
	})
}

// TestMustGetServerPanic 测试 MustGetServer panic
func TestMustGetServerPanic(t *testing.T) {
	assert.Panics(t, func() {
		MustGetServer("non-existent-must")
	})
}

// TestMustGetGinServerSuccess 测试 MustGetGinServer 成功
func TestMustGetGinServerSuccess(t *testing.T) {
	manager := GetManager()
	ginOpt := GinOption{
		Name: "gin-server-must",
		Host: "127.0.0.1",
		Port: 8081,
	}
	ginServer := NewGinServer(ginOpt)
	manager.Register(ginServer)

	assert.NotPanics(t, func() {
		retrieved := MustGetGinServer("gin-server-must")
		assert.NotNil(t, retrieved)
	})
}

// TestMustGetGinServerPanic 测试 MustGetGinServer panic
func TestMustGetGinServerPanic(t *testing.T) {
	assert.Panics(t, func() {
		MustGetGinServer("non-existent-gin")
	})
}

// TestMustGetCronServerSuccess 测试 MustGetCronServer 成功
func TestMustGetCronServerSuccess(t *testing.T) {
	manager := GetManager()
	ctx := NewContext(Endpoint("cron"), "/must-test")
	cronServer := NewCronServer(ctx, CrontabNormalMode, []TaskConfig{})
	manager.Register(cronServer)

	assert.NotPanics(t, func() {
		retrieved := MustGetCronServer("cron-server")
		assert.NotNil(t, retrieved)
	})
}

// TestMustGetCronServerPanic 测试 MustGetCronServer panic
func TestMustGetCronServerPanic(t *testing.T) {
	assert.Panics(t, func() {
		MustGetCronServer("non-existent-cron")
	})
}

// =============================================================================
// 并发测试
// =============================================================================

// TestServerManagerConcurrentRegister 测试并发注册
func TestServerManagerConcurrentRegister(t *testing.T) {
	manager := NewServerManager()
	var wg sync.WaitGroup

	// 并发注册 100 个服务器
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			server := newSMMockServer(fmt.Sprintf("concurrent-%d", n))
			err := manager.Register(server)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// 验证所有服务器都注册成功
	servers := manager.GetAllServers()
	assert.Len(t, servers, 100)
}

// TestServerManagerConcurrentGet 测试并发获取
func TestServerManagerConcurrentGet(t *testing.T) {
	manager := NewServerManager()

	// 先注册 10 个服务器
	for i := 0; i < 10; i++ {
		server := newSMMockServer(fmt.Sprintf("server-%d", i))
		manager.Register(server)
	}

	var wg sync.WaitGroup
	successCount := 0
	var countMutex sync.Mutex

	// 并发获取 100 次
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			serverName := fmt.Sprintf("server-%d", n%10)
			_, err := manager.GetServer(serverName)
			if err == nil {
				countMutex.Lock()
				successCount++
				countMutex.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 所有获取都应该成功
	assert.Equal(t, 100, successCount)
}

// TestServerManagerConcurrentRegisterAndGet 测试并发注册和获取
func TestServerManagerConcurrentRegisterAndGet(t *testing.T) {
	manager := NewServerManager()
	var wg sync.WaitGroup

	// 并发注册
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			server := newSMMockServer(fmt.Sprintf("mixed-%d", n))
			manager.Register(server)
		}(i)
	}

	// 并发获取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			serverName := fmt.Sprintf("mixed-%d", n)
			_, _ = manager.GetServer(serverName)
		}(i)
	}

	wg.Wait()

	// 所有服务器都应该注册
	servers := manager.GetAllServers()
	assert.Len(t, servers, 50)
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkServerManagerRegister 注册性能
func BenchmarkServerManagerRegister(b *testing.B) {
	manager := NewServerManager()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := newSMMockServer(fmt.Sprintf("bench-%d", i))
		manager.Register(server)
	}
}

// BenchmarkServerManagerGetServer 获取性能
func BenchmarkServerManagerGetServer(b *testing.B) {
	manager := NewServerManager()
	for i := 0; i < 100; i++ {
		server := newSMMockServer(fmt.Sprintf("server-%d", i))
		manager.Register(server)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetServer(fmt.Sprintf("server-%d", i%100))
	}
}

// BenchmarkServerManagerGetAllServers 获取所有服务器性能
func BenchmarkServerManagerGetAllServers(b *testing.B) {
	manager := NewServerManager()
	for i := 0; i < 100; i++ {
		server := newSMMockServer(fmt.Sprintf("server-%d", i))
		manager.Register(server)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetAllServers()
	}
}

// BenchmarkNewServerManager 创建管理器性能
func BenchmarkNewServerManager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewServerManager()
	}
}
