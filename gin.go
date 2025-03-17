package sylph

import (
	"context"
	"errors"
	"fmt"
	"github.com/sylphbyte/pr"
	"net/http"

	"time"

	"github.com/gin-gonic/gin"
)

type GinOptions []GinOption

type GinOption struct {
	Name         string `yaml:"name"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout" mapstructure:"write_timeout"`
	MaxBytes     int    `yaml:"max_bytes" mapstructure:"max_bytes"`
}

// GinServer 实现 IServer
type GinServer struct {
	opt         GinOption
	engine      *gin.Engine  // gin 引擎
	httpServer  *http.Server // 用于优雅关闭
	routeHandle func(route *gin.Engine)
}

func NewGin(opt GinOption) *GinServer {
	return &GinServer{
		opt:    opt,
		engine: gin.New(),
	}
}

func (g *GinServer) RegisterRoute(route func(route *gin.Engine)) {
	route(g.engine)
}

// Boot 启动服务
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

// Shutdown 优雅关闭服务
func (g *GinServer) Shutdown() error {
	pr.System("Shutting down Gin server on port %d...\n", g.opt.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if g.httpServer != nil {
		return g.httpServer.Shutdown(ctx)
	}

	return nil
}
