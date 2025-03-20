package sylph

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 为避免与现有mockServer冲突，创建新的测试结构体
type smTestServer struct {
	serverName string
	bootErr    error
	shutErr    error
	bootFlag   bool
	shutFlag   bool
}

func (m *smTestServer) Name() string {
	return m.serverName
}

func (m *smTestServer) Boot() error {
	m.bootFlag = true
	return m.bootErr
}

func (m *smTestServer) Shutdown() error {
	m.shutFlag = true
	return m.shutErr
}

// 模拟GinServer，用于测试类型断言
type smTestGinServer struct {
	smTestServer
	engine *gin.Engine
}

func newTestGinServer(name string) *smTestGinServer {
	return &smTestGinServer{
		smTestServer: smTestServer{serverName: name},
		engine:       gin.New(),
	}
}

// 测试创建服务器管理器
func TestNewServerManager(t *testing.T) {
	manager := NewServerManager()
	assert.NotNil(t, manager, "Manager不应该为nil")

	servers := manager.GetAllServers()
	assert.Empty(t, servers, "新创建的管理器不应该有服务器")
}

// 测试注册服务器
func TestServerManager_Register(t *testing.T) {
	manager := NewServerManager()

	// 注册一个服务器
	server1 := &smTestServer{serverName: "server1"}
	err := manager.Register(server1)
	assert.NoError(t, err, "注册server1失败")

	// 注册同名服务器
	server2 := &smTestServer{serverName: "server1"}
	err = manager.Register(server2)
	assert.Error(t, err, "注册同名服务器应该失败")
	assert.ErrorIs(t, err, ErrServerExists, "错误类型应该是ErrServerExists")

	// 注册nil服务器
	err = manager.Register(nil)
	assert.Error(t, err, "注册nil服务器应该失败")

	// 注册空名称服务器
	server3 := &smTestServer{serverName: ""}
	err = manager.Register(server3)
	assert.Error(t, err, "注册空名称服务器应该失败")
}

// 测试获取服务器
func TestServerManager_GetServer(t *testing.T) {
	manager := NewServerManager()

	// 注册一个服务器
	server1 := &smTestServer{serverName: "server1"}
	_ = manager.Register(server1)

	// 获取存在的服务器
	s, err := manager.GetServer("server1")
	assert.NoError(t, err, "获取server1应该成功")
	assert.Equal(t, server1, s, "获取到的服务器应该是server1")

	// 获取不存在的服务器
	s, err = manager.GetServer("not_exist")
	assert.Error(t, err, "获取不存在的服务器应该失败")
	assert.ErrorIs(t, err, ErrServerNotFound, "错误类型应该是ErrServerNotFound")
	assert.Nil(t, s, "不存在的服务器应该是nil")
}

// 测试获取特定类型服务器
func TestServerManager_GetSpecificServer(t *testing.T) {
	manager := NewServerManager()

	// 注册不同类型的服务器
	ginServer := newTestGinServer("ginServer")
	cronServer := &smTestServer{serverName: "cronServer"}

	_ = manager.Register(ginServer)
	_ = manager.Register(cronServer)

	// 测试GetGinServer
	gs, err := manager.GetGinServer("ginServer")
	assert.NoError(t, err, "获取ginServer应该成功")
	assert.Equal(t, ginServer, gs, "获取到的GinServer应该是ginServer")

	// 测试类型不匹配
	gs, err = manager.GetGinServer("cronServer")
	assert.Error(t, err, "获取类型不匹配的服务器应该失败")
	assert.ErrorIs(t, err, ErrServerTypeMismatch, "错误类型应该是ErrServerTypeMismatch")

	// 测试不存在的服务器
	gs, err = manager.GetGinServer("not_exist")
	assert.Error(t, err, "获取不存在的服务器应该失败")
	assert.ErrorIs(t, err, ErrServerNotFound, "错误类型应该是ErrServerNotFound")
}

// 测试启动和关闭所有服务器
func TestServerManager_BootAndShutdown(t *testing.T) {
	manager := NewServerManager()

	// 注册几个服务器
	server1 := &smTestServer{serverName: "server1"}
	server2 := &smTestServer{serverName: "server2"}

	_ = manager.Register(server1)
	_ = manager.Register(server2)

	// 测试启动所有服务器
	err := manager.BootAll()
	assert.NoError(t, err, "启动所有服务器应该成功")
	assert.True(t, server1.bootFlag, "server1应该被启动")
	assert.True(t, server2.bootFlag, "server2应该被启动")

	// 测试关闭所有服务器
	err = manager.ShutdownAll()
	assert.NoError(t, err, "关闭所有服务器应该成功")
	assert.True(t, server1.shutFlag, "server1应该被关闭")
	assert.True(t, server2.shutFlag, "server2应该被关闭")
}

// 测试全局单例
func TestGetManager(t *testing.T) {
	manager1 := GetManager()
	manager2 := GetManager()

	assert.NotNil(t, manager1, "Manager不应该为nil")
	assert.Same(t, manager1, manager2, "两次获取应返回相同的实例")
}

// 测试MustGet函数
func TestMustGetFunctions(t *testing.T) {
	manager := NewServerManager()
	defaultManager = manager // 替换单例实例用于测试

	// 注册一个服务器
	ginServer := newTestGinServer("ginServer")
	_ = manager.Register(ginServer)

	// 测试MustGetServer
	assert.NotPanics(t, func() {
		s := MustGetServer("ginServer")
		assert.Equal(t, ginServer, s)
	})

	// 测试MustGetGinServer
	assert.NotPanics(t, func() {
		s := MustGetGinServer("ginServer")
		assert.Equal(t, ginServer, s)
	})

	// 测试MustGetServer失败时应该panic
	assert.Panics(t, func() {
		_ = MustGetServer("not_exist")
	})
}
