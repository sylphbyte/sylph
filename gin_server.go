package sylph

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sylphbyte/pr"

	"time"

	"github.com/gin-gonic/gin"
)

// GinOptions 是GinOption的切片类型
// 用于配置多个Gin服务器
type GinOptions []GinOption

// GinOption Gin服务器配置选项
// 定义Gin服务器的基本参数和行为设置
type GinOption struct {
	Name         string `yaml:"name"`                                       // 服务器名称，必须唯一
	Host         string `yaml:"host"`                                       // 监听主机地址，如"0.0.0.0"表示所有网卡
	Port         int    `yaml:"port"`                                       // 监听端口号
	ReadTimeout  int    `yaml:"read_timeout" mapstructure:"read_timeout"`   // HTTP读取超时时间(秒)
	WriteTimeout int    `yaml:"write_timeout" mapstructure:"write_timeout"` // HTTP写入超时时间(秒)
	MaxBytes     int    `yaml:"max_bytes" mapstructure:"max_bytes"`         // 请求体最大字节数
}

// 确保GinServer实现了IServer接口
var _ IServer = (*GinServer)(nil)

// NewGinServer 创建一个新的Gin服务器实例
//
// 参数:
//   - opt: Gin服务器配置选项
//
// 返回:
//   - *GinServer: 新创建的Gin服务器实例
//
// 使用示例:
//
//	ginOpt := sylph.GinOption{
//	    Name: "api",
//	    Host: "0.0.0.0",
//	    Port: 8080,
//	}
//	ginServer := sylph.NewGinServer(ginOpt)
//	sylph.GetManager().Register(ginServer)
func NewGinServer(opt GinOption) *GinServer {
	return &GinServer{
		opt:    opt,
		engine: gin.New(),
	}
}

// GinServer Gin HTTP服务器实现
// 基于Gin框架的HTTP服务器，实现IServer接口
type GinServer struct {
	opt         GinOption               // 服务器配置选项
	engine      *gin.Engine             // gin引擎，处理HTTP请求路由
	httpServer  *http.Server            // 标准库HTTP服务器，用于优雅启动和关闭
	routeHandle func(route *gin.Engine) // 路由注册函数
}

// Name 获取服务器名称
// 实现IServer接口的Name方法
//
// 返回:
//   - string: 服务器名称
func (g *GinServer) Name() string {
	return g.opt.Name
}

// RegisterRoute 注册路由处理函数
// 允许外部注册HTTP路由和处理器
//
// 参数:
//   - route: 路由注册函数，接收gin.Engine参数
//
// 使用示例:
//
//	ginServer.RegisterRoute(func(r *gin.Engine) {
//	    r.GET("/api/v1/users", handleGetUsers)
//	    r.POST("/api/v1/users", handleCreateUser)
//	})
func (g *GinServer) RegisterRoute(route func(route *gin.Engine)) {
	route(g.engine)
}

// Boot 启动服务器
// 实现IServer接口的Boot方法，非阻塞式启动HTTP服务
//
// 返回:
//   - error: 启动过程中的错误，成功启动返回nil
//
// 注意事项:
//   - 此方法为非阻塞式，会在后台goroutine中启动服务器
//   - 服务器实际启动失败不会立即返回错误，而是记录到日志
func (g *GinServer) Boot() error {
	pr.System("Starting Gin server on port %d...\n", g.opt.Port)

	// 创建 http.Server
	g.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", g.opt.Host, g.opt.Port),
		Handler: g.engine,
	}

	// 异步启动
	go func() {
		if err := g.httpServer.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			pr.Error("Gin server error: %v\n", err)
		}
	}()

	return nil
}

// Shutdown 优雅关闭服务器
// 实现IServer接口的Shutdown方法，等待现有请求处理完成后关闭服务
//
// 返回:
//   - error: 关闭过程中的错误，成功关闭返回nil
//
// 注意事项:
//   - 使用5秒超时上下文，确保服务器能在限定时间内关闭
//   - 如果服务器未启动(httpServer为nil)，则直接返回nil
//   - 此方法会阻塞等待所有请求处理完成或超时
func (g *GinServer) Shutdown() error {
	pr.System("Shutting down Gin server on port %d...\n", g.opt.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if g.httpServer != nil {
		return g.httpServer.Shutdown(ctx)
	}

	return nil
}
