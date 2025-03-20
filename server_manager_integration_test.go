package sylph

import (
	"testing"
	"time"

	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// 集成测试：测试管理器与GinServer集成
func TestServerManager_WithGinServer(t *testing.T) {
	// 跳过此测试，除非明确要求运行
	t.Skip("集成测试，默认跳过。使用 -test.run=TestServerManager_WithGinServer 运行")

	// 加载HTTP配置
	data, err := ioutil.ReadFile("etc/service_http.yaml")
	if err != nil {
		t.Fatalf("读取HTTP配置失败: %v", err)
	}

	var config struct {
		Https []GinOption `yaml:"https"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("解析HTTP配置失败: %v", err)
	}

	// 创建服务器管理器
	manager := NewServerManager()

	// 基于配置创建GinServer并注册
	for _, opt := range config.Https {
		if opt.Name == "" {
			continue
		}

		// 创建Gin服务器
		server := NewGinServer(opt)

		// 配置路由
		server.RegisterRoute(func(r *gin.Engine) {
			r.GET("/health", func(c *gin.Context) {
				c.JSON(200, gin.H{
					"status": "ok",
					"server": opt.Name,
				})
			})
		})

		// 注册到管理器
		if err := manager.Register(server); err != nil {
			t.Fatalf("注册Gin服务器失败: %v", err)
		}
	}

	// 启动所有服务器
	if err := manager.BootAll(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}

	// 延迟关闭
	defer func() {
		if err := manager.ShutdownAll(); err != nil {
			t.Errorf("关闭服务器失败: %v", err)
		}
	}()

	// 给服务器一些启动时间
	time.Sleep(1 * time.Second)

	// 测试获取特定的Gin服务器
	server, err := manager.GetGinServer("api")
	if err != nil {
		t.Fatalf("获取api服务器失败: %v", err)
	}
	assert.Equal(t, "api", server.Name())

	// 可以在这里添加HTTP客户端测试代码，验证服务器功能
}

// 集成测试：测试管理器与CronServer集成
func TestServerManager_WithCronServer(t *testing.T) {
	// 跳过此测试，除非明确要求运行
	t.Skip("集成测试，默认跳过。使用 -test.run=TestServerManager_WithCronServer 运行")

	// 加载Cron配置
	data, err := ioutil.ReadFile("etc/service_crontab.yaml")
	if err != nil {
		t.Fatalf("读取Cron配置失败: %v", err)
	}

	var config struct {
		ModeSwitch struct {
			Normal bool `yaml:"normal"`
			Skip   bool `yaml:"skip"`
			Delay  bool `yaml:"delay"`
		} `yaml:"mode_switch"`
		Jobs struct {
			Normal []TaskConfig `yaml:"normal"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("解析Cron配置失败: %v", err)
	}

	// 确定模式
	var mode CrontabMode = CrontabNormalMode
	if config.ModeSwitch.Skip {
		mode = CrontabSkipMode
	} else if config.ModeSwitch.Delay {
		mode = CrontabDelayMode
	}

	// 创建服务器管理器
	manager := NewServerManager()

	// 创建上下文
	ctx := NewContext("test", "cron-test")

	// 创建Cron服务器
	server := NewCronServer(ctx, mode, config.Jobs.Normal)

	// 注册示例任务
	server.Register("normal-test", func(ctx Context) error {
		// 仅为测试，不执行实际操作
		return nil
	})

	// 注册到管理器
	if err := manager.Register(server); err != nil {
		t.Fatalf("注册Cron服务器失败: %v", err)
	}

	// 启动所有服务器
	if err := manager.BootAll(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}

	// 运行一小段时间
	time.Sleep(2 * time.Second)

	// 关闭服务器
	if err := manager.ShutdownAll(); err != nil {
		t.Errorf("关闭服务器失败: %v", err)
	}

	// 验证获取服务器
	_, err = manager.GetCronServer("cron-server")
	assert.NoError(t, err)
}

// 集成测试：测试BootAll和ShutdownAll
func TestServerManager_BootAndShutdownAll(t *testing.T) {
	// 创建服务器管理器
	manager := NewServerManager()

	// 创建多个测试服务器
	server1 := &smTestServer{serverName: "test-server-1"}
	server2 := &smTestServer{serverName: "test-server-2"}
	server3 := &smTestServer{serverName: "test-server-3"}

	// 注册服务器
	_ = manager.Register(server1)
	_ = manager.Register(server2)
	_ = manager.Register(server3)

	// 启动所有服务器
	err := manager.BootAll()
	assert.NoError(t, err)
	assert.True(t, server1.bootFlag)
	assert.True(t, server2.bootFlag)
	assert.True(t, server3.bootFlag)

	// 关闭所有服务器
	err = manager.ShutdownAll()
	assert.NoError(t, err)
	assert.True(t, server1.shutFlag)
	assert.True(t, server2.shutFlag)
	assert.True(t, server3.shutFlag)
}
