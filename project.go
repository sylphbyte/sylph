package sylph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sylphbyte/pr"
)

// State 服务状态枚举
type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

var stateNames = map[State]string{
	StateStopped:  "Stopped",
	StateStarting: "Starting",
	StateRunning:  "Running",
	StateStopping: "Stopping",
}

// String 返回状态的文本表示
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", s)
}

// ProjectOption 项目配置选项
type ProjectOption func(*Project)

// WithShutdownTimeout 设置关闭超时时间
func WithShutdownTimeout(timeout time.Duration) ProjectOption {
	return func(p *Project) {
		if timeout > 0 {
			p.shutdownTimeout = timeout
		}
	}
}

// WithBootTimeout 设置启动超时时间
func WithBootTimeout(timeout time.Duration) ProjectOption {
	return func(p *Project) {
		if timeout > 0 {
			p.bootTimeout = timeout
		}
	}
}

// WithOrderedExecution 设置按顺序执行
func WithOrderedExecution(ordered bool) ProjectOption {
	return func(p *Project) {
		p.ordered = ordered
	}
}

// Project 实现 IProject
type Project struct {
	sync.RWMutex
	servers         []IServer                 // 服务列表
	states          map[IServer]*atomic.Int32 // 服务状态，使用原子操作访问
	shutdownTimeout time.Duration             // 关闭超时时间
	bootTimeout     time.Duration             // 启动超时时间
	ordered         bool                      // 是否按顺序启动和关闭
	serverNames     map[IServer]string        // 服务名称缓存，用于日志
}

// NewProject 构造函数
func NewProject(options ...ProjectOption) *Project {
	p := &Project{
		servers:         make([]IServer, 0, 8),           // 增加预分配空间
		states:          make(map[IServer]*atomic.Int32), // 状态映射
		serverNames:     make(map[IServer]string),        // 名称缓存
		shutdownTimeout: 30 * time.Second,                // 默认30秒超时
		bootTimeout:     60 * time.Second,                // 默认60秒启动超时
		ordered:         false,                           // 默认并行启动
	}

	// 应用选项
	for _, option := range options {
		option(p)
	}

	return p
}

// getServerName 获取服务名称（带缓存）
func (p *Project) getServerName(server IServer) string {
	// 先尝试从缓存获取
	p.RLock()
	name, ok := p.serverNames[server]
	p.RUnlock()

	if ok {
		return name
	}

	// 缓存未命中，生成名称并缓存
	name = fmt.Sprintf("%T", server)
	p.Lock()
	p.serverNames[server] = name
	p.Unlock()

	return name
}

// getServerState 获取服务状态
// 使用读锁减少锁竞争
func (p *Project) getServerState(server IServer) State {
	p.RLock()
	defer p.RUnlock()

	if state, ok := p.states[server]; ok {
		return State(state.Load())
	}
	return StateStopped
}

// setServerState 设置服务状态
// 使用写锁保护状态映射
func (p *Project) setServerState(server IServer, state State) {
	p.Lock()
	defer p.Unlock()

	if statePtr, ok := p.states[server]; ok {
		statePtr.Store(int32(state))
	} else {
		stateVal := atomic.Int32{}
		stateVal.Store(int32(state))
		p.states[server] = &stateVal
	}
}

// Mounts 装载服务
func (p *Project) Mounts(servers ...IServer) IProject {
	if len(servers) == 0 {
		return p
	}

	p.Lock()
	defer p.Unlock()

	// 预分配足够容量
	if cap(p.servers)-len(p.servers) < len(servers) {
		newServers := make([]IServer, len(p.servers), len(p.servers)+len(servers))
		copy(newServers, p.servers)
		p.servers = newServers
	}

	for _, s := range servers {
		if s != nil {
			p.servers = append(p.servers, s)

			// 初始化状态为已停止
			statePtr := &atomic.Int32{}
			statePtr.Store(int32(StateStopped))
			p.states[s] = statePtr
		}
	}
	return p
}

// prepareToStart 准备启动的服务
func (p *Project) prepareToStart() []IServer {
	p.Lock()
	defer p.Unlock()

	// 确定需要启动的服务
	serversToStart := make([]IServer, 0, len(p.servers))
	for _, srv := range p.servers {
		statePtr := p.states[srv]
		state := State(statePtr.Load())
		if state == StateStopped {
			serversToStart = append(serversToStart, srv)
			statePtr.Store(int32(StateStarting))
		}
	}
	return serversToStart
}

// bootServers 按指定方式启动服务
func (p *Project) bootServers(ctx context.Context, servers []IServer) []error {
	if len(servers) == 0 {
		return nil
	}

	// 创建错误存储
	errChan := make(chan error, len(servers))
	startedServers := make([]IServer, 0, len(servers))
	var startedMutex sync.Mutex

	// 串行启动
	if p.ordered {
		for _, srv := range servers {
			err := p.bootServer(ctx, srv)
			if err != nil {
				errChan <- fmt.Errorf("service %s failed to boot: %w", p.getServerName(srv), err)
				break
			}
			startedMutex.Lock()
			startedServers = append(startedServers, srv)
			startedMutex.Unlock()
		}
	} else {
		// 并行启动
		var wg sync.WaitGroup
		for _, srv := range servers {
			wg.Add(1)
			go func(s IServer) {
				defer wg.Done()
				err := p.bootServer(ctx, s)
				if err != nil {
					errChan <- fmt.Errorf("service %s failed to boot: %w", p.getServerName(s), err)
					return
				}
				startedMutex.Lock()
				startedServers = append(startedServers, s)
				startedMutex.Unlock()
			}(srv)
		}
		wg.Wait()
	}

	// 处理错误
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// 如果有错误，回滚已启动的服务
	if len(errs) > 0 {
		p.rollbackServers(startedServers)
	}

	return errs
}

// bootServer 启动单个服务
func (p *Project) bootServer(ctx context.Context, srv IServer) error {
	serverName := p.getServerName(srv)
	pr.System("Starting service %s...\n", serverName)

	// 创建带超时的上下文
	bootCtx, cancel := context.WithTimeout(ctx, p.bootTimeout)
	defer cancel()

	// 启动一个协程执行启动，并监听超时
	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errStr := fmt.Sprintf("panic in service %s boot: %v", serverName, r)
				pr.Error("%s\n", errStr)
				errCh <- errors.New(errStr)
			}
		}()

		err := srv.Boot()
		errCh <- err
	}()

	// 等待启动完成或超时
	select {
	case err := <-errCh:
		if err != nil {
			p.setServerState(srv, StateStopped)
			pr.Error("Failed to start service %s: %v\n", serverName, err)
			return err
		}

		p.setServerState(srv, StateRunning)
		pr.System("Service %s started successfully\n", serverName)
		return nil

	case <-bootCtx.Done():
		p.setServerState(srv, StateStopped)
		err := fmt.Errorf("service %s boot timeout after %v", serverName, p.bootTimeout)
		pr.Error("%v\n", err)
		return err
	}
}

// Boots 启动所有已停止的服务
func (p *Project) Boots() error {
	// 准备需要启动的服务
	serversToStart := p.prepareToStart()
	if len(serversToStart) == 0 {
		return nil
	}

	// 创建上下文
	ctx := context.Background()

	// 启动服务
	var errs []error
	if p.ordered {
		// 按顺序启动
		errs = p.orderedBoot(ctx, serversToStart)
	} else {
		// 并行启动
		errs = p.parallelBoot(ctx, serversToStart)
	}

	// 处理错误
	if len(errs) > 0 {
		errorMsg := make([]string, 0, len(errs))
		for _, err := range errs {
			errorMsg = append(errorMsg, err.Error())
		}
		return fmt.Errorf("boot failed: %s", strings.Join(errorMsg, "; "))
	}

	return nil
}

// rollbackServers 回滚启动的服务
func (p *Project) rollbackServers(started []IServer) {
	if len(started) == 0 {
		return
	}

	pr.System("Rolling back %d started services...\n", len(started))

	// 并行关闭以加速回滚
	var wg sync.WaitGroup
	for i := len(started) - 1; i >= 0; i-- {
		srv := started[i]
		if p.getServerState(srv) == StateRunning {
			wg.Add(1)
			go func(s IServer) {
				defer wg.Done()

				serverName := p.getServerName(s)
				pr.System("Rolling back service %s...\n", serverName)
				p.setServerState(s, StateStopping)

				// 安全关闭，忽略错误
				shutdownErr := s.Shutdown()
				if shutdownErr != nil {
					pr.Warning("Ignoring error during rollback of %s: %v\n", serverName, shutdownErr)
				}

				p.setServerState(s, StateStopped)
			}(srv)
		}
	}

	wg.Wait()
	pr.System("Rollback completed\n")
}

// prepareToStop 准备停止的服务
func (p *Project) prepareToStop() []IServer {
	p.Lock()
	defer p.Unlock()

	// 确定需要停止的服务（逆序）
	serversToStop := make([]IServer, 0, len(p.servers))
	for i := len(p.servers) - 1; i >= 0; i-- {
		srv := p.servers[i]
		statePtr := p.states[srv]
		state := State(statePtr.Load())
		if state == StateRunning {
			serversToStop = append(serversToStop, srv)
			statePtr.Store(int32(StateStopping))
		}
	}
	return serversToStop
}

// Shutdowns 依次关闭所有已启用的服务(逆序)
func (p *Project) Shutdowns() error {
	// 准备需要停止的服务
	serversToStop := p.prepareToStop()
	if len(serversToStop) == 0 {
		return nil
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer cancel()

	// 关闭服务
	var errs []error
	if p.ordered {
		// 串行关闭
		errs = p.orderedShutdown(ctx, serversToStop)
	} else {
		// 并行关闭
		errs = p.parallelShutdown(ctx, serversToStop)
	}

	// 处理错误
	if len(errs) > 0 {
		errorMsg := make([]string, 0, len(errs))
		for _, err := range errs {
			errorMsg = append(errorMsg, err.Error())
		}
		return fmt.Errorf("shutdown failed: %s", strings.Join(errorMsg, "; "))
	}

	return nil
}

// WaitForShutdown 等待优雅关闭信号
func (p *Project) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待关闭信号
	sig := <-sigChan
	pr.System("Received signal: %v, shutting down...\n", sig)

	// 打印当前服务状态
	states := p.GetServerStates()
	pr.System("Current service states before shutdown:\n")
	for name, state := range states {
		pr.System("  %s: %s\n", name, state)
	}

	// 执行关闭
	startTime := time.Now()
	if err := p.Shutdowns(); err != nil {
		pr.Red("Shutdown failed after %v: %v\n", time.Since(startTime), err)
		os.Exit(1)
	}

	pr.System("All services shutdown gracefully in %v\n", time.Since(startTime))
	os.Exit(0)
}

// parallelBoot 改进的并行启动方法
// 使用errChan立即收集错误，而不是等待所有服务启动
func (p *Project) parallelBoot(ctx context.Context, servers []IServer) []error {
	var (
		wg      sync.WaitGroup
		errChan = make(chan error, len(servers))
		done    = make(chan struct{})
		errors  = make([]error, 0)
		started = make([]IServer, 0, len(servers))
		mu      sync.Mutex // 保护started切片
	)

	// 启动所有服务
	for _, server := range servers {
		wg.Add(1)
		go func(srv IServer) {
			defer wg.Done()

			name := p.getServerName(srv)
			pr.System("Starting service %s...\n", name)
			p.setServerState(srv, StateStarting)

			// 启动服务
			err := srv.Boot()
			if err != nil {
				errChan <- fmt.Errorf("failed to boot %s: %v", name, err)
				p.setServerState(srv, StateStopped)
				return
			}

			// 更新状态并记录已启动的服务
			p.setServerState(srv, StateRunning)
			mu.Lock()
			started = append(started, srv)
			mu.Unlock()

			pr.System("Service %s started\n", name)
		}(server)
	}

	// 等待所有启动完成
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待完成或超时
	select {
	case <-done:
		// 所有服务已处理，检查是否有错误
		close(errChan)
		for err := range errChan {
			errors = append(errors, err)
		}

	case <-time.After(p.bootTimeout):
		// 超时
		errors = append(errors, fmt.Errorf("boot timeout after %v", p.bootTimeout))
	}

	// 如果有错误，回滚已启动的服务
	if len(errors) > 0 {
		p.rollbackServers(started)
	}

	return errors
}

// parallelShutdown 改进的并行关闭方法
func (p *Project) parallelShutdown(ctx context.Context, servers []IServer) []error {
	var (
		wg      sync.WaitGroup
		errChan = make(chan error, len(servers))
		done    = make(chan struct{})
		errors  = make([]error, 0)
	)

	// 关闭所有服务
	for _, server := range servers {
		wg.Add(1)
		go func(srv IServer) {
			defer wg.Done()

			name := p.getServerName(srv)
			pr.System("Stopping service %s...\n", name)

			// 关闭服务
			err := srv.Shutdown()
			if err != nil {
				errChan <- fmt.Errorf("failed to shutdown %s: %v", name, err)
			} else {
				p.setServerState(srv, StateStopped)
				pr.System("Service %s stopped\n", name)
			}
		}(server)
	}

	// 等待所有关闭完成
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待完成或超时
	select {
	case <-done:
		// 所有服务已处理，检查是否有错误
		close(errChan)
		for err := range errChan {
			errors = append(errors, err)
		}

	case <-ctx.Done():
		// 超时
		errors = append(errors, fmt.Errorf("shutdown timeout after %v", p.shutdownTimeout))
	}

	return errors
}

// orderedBoot 按顺序启动服务
func (p *Project) orderedBoot(ctx context.Context, servers []IServer) []error {
	errs := make([]error, 0)
	started := make([]IServer, 0, len(servers))

	for _, srv := range servers {
		name := p.getServerName(srv)
		pr.System("Starting service %s...\n", name)
		p.setServerState(srv, StateStarting)

		err := srv.Boot()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to boot %s: %v", name, err))
			p.setServerState(srv, StateStopped)
			break // 失败时停止后续启动
		}

		p.setServerState(srv, StateRunning)
		started = append(started, srv)
		pr.System("Service %s started\n", name)
	}

	// 如果有错误，回滚已启动的服务
	if len(errs) > 0 {
		p.rollbackServers(started)
	}

	return errs
}

// orderedShutdown 按顺序关闭服务
func (p *Project) orderedShutdown(ctx context.Context, servers []IServer) []error {
	errs := make([]error, 0)

	for _, srv := range servers {
		name := p.getServerName(srv)
		pr.System("Stopping service %s...\n", name)

		// 使用带超时的上下文
		shutdownCtx, cancel := context.WithTimeout(ctx, p.shutdownTimeout/time.Duration(len(servers)))

		// 创建错误通道
		errCh := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errStr := fmt.Sprintf("panic in service %s shutdown: %v", name, r)
					errCh <- errors.New(errStr)
				}
			}()
			errCh <- srv.Shutdown()
		}()

		// 等待关闭完成或超时
		var err error
		select {
		case e := <-errCh:
			err = e
		case <-shutdownCtx.Done():
			err = fmt.Errorf("service %s shutdown timeout", name)
		}

		cancel() // 释放上下文资源

		if err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown %s: %v", name, err))
			// 继续关闭其他服务
		} else {
			p.setServerState(srv, StateStopped)
			pr.System("Service %s stopped\n", name)
		}
	}

	return errs
}

// GetServerStates 获取所有服务的状态
func (p *Project) GetServerStates() map[string]string {
	p.RLock()
	defer p.RUnlock()

	result := make(map[string]string, len(p.servers))
	for srv, statePtr := range p.states {
		state := State(statePtr.Load())
		result[p.getServerName(srv)] = state.String()
	}

	return result
}
