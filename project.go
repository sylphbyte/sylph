package sylph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

// Project 实现 IProject
type Project struct {
	sync.RWMutex
	servers []IServer
	states  map[IServer]State
}

// NewProject 构造函数
func NewProject() *Project {
	return &Project{
		servers: make([]IServer, 0, 4), // 预分配空间
		states:  make(map[IServer]State),
	}
}

// Mounts 装载服务
func (c *Project) Mounts(servers ...IServer) IProject {
	c.Lock()
	defer c.Unlock()

	for _, s := range servers {
		if s != nil {
			c.servers = append(c.servers, s)
			c.states[s] = StateStopped
		}
	}
	return c
}

// Boots 启动所有已启用的服务
func (c *Project) Boots() error {
	c.Lock()
	// 初始化状态
	serversToStart := make([]IServer, 0, len(c.servers))
	for _, srv := range c.servers {
		if c.states[srv] == StateStopped {
			serversToStart = append(serversToStart, srv)
			c.states[srv] = StateStarting
		}
	}
	c.Unlock()

	if len(serversToStart) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(serversToStart))
	stateLock := &sync.Mutex{}
	startedServers := make([]IServer, 0, len(serversToStart))

	for _, srv := range serversToStart {
		wg.Add(1)
		go func(s IServer) {
			defer wg.Done()

			pr.System("Starting service %T...\n", s)
			err := s.Boot()

			stateLock.Lock()
			defer stateLock.Unlock()

			if err != nil {
				c.states[s] = StateStopped
				pr.Error("Failed to start service %T: %v\n", s, err)
				errChan <- fmt.Errorf("service %T failed to boot: %w", s, err)
				return
			}

			c.states[s] = StateRunning
			startedServers = append(startedServers, s)
			pr.System("Service %T started successfully\n", s)
		}(srv)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		c.rollback(startedServers)
		return errors.Join(errs...)
	}

	return nil
}

// rollback 用于当启动失败时，按照逆序关闭已成功启动的服务
func (c *Project) rollback(started []IServer) {
	c.Lock()
	defer c.Unlock()

	for i := len(started) - 1; i >= 0; i-- {
		srv := started[i]
		if c.states[srv] == StateRunning {
			pr.System("Rolling back service %T...\n", srv)
			c.states[srv] = StateStopping
			c.Unlock()         // 解锁以允许并发关闭
			_ = srv.Shutdown() // 忽略关闭错误
			c.Lock()           // 重新加锁以更新状态
			c.states[srv] = StateStopped
		}
	}
}

// Shutdowns 依次关闭所有已启用的服务(逆序)
func (c *Project) Shutdowns() error {
	c.Lock()
	// 初始化状态
	serversToStop := make([]IServer, 0, len(c.servers))
	for i := len(c.servers) - 1; i >= 0; i-- {
		srv := c.servers[i]
		if c.states[srv] == StateRunning {
			serversToStop = append(serversToStop, srv)
			c.states[srv] = StateStopping
		}
	}
	c.Unlock()

	if len(serversToStop) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(serversToStop))
	stateLock := &sync.Mutex{}

	for _, srv := range serversToStop {
		wg.Add(1)
		go func(s IServer) {
			defer wg.Done()

			pr.System("Shutting down service %T...\n", s)
			err := s.Shutdown()

			stateLock.Lock()
			defer stateLock.Unlock()

			if err != nil {
				c.states[s] = StateRunning
				pr.Error("Failed to shutdown service %T: %v\n", s, err)
				errChan <- fmt.Errorf("service %T failed to shutdown: %w", s, err)
				return
			}

			c.states[s] = StateStopped
			pr.System("Service %T shutdown successfully\n", s)
		}(srv)
	}

	// 等待所有协程完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有服务正常关闭
	case <-ctx.Done():
		// 超时强制关闭
		pr.Error("Shutdown timeout, forcing exit...\n")
	}

	// 收集错误
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// WaitForShutdown 等待优雅关闭信号
func (c *Project) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待关闭信号
	sig := <-sigChan
	pr.System("Received signal: %v, shutting down...\n", sig)

	// 执行关闭
	if err := c.Shutdowns(); err != nil {
		pr.Red("Shutdown failed: %v\n", err)
		os.Exit(1)
	}

	pr.System("All services shutdown gracefully\n")
	os.Exit(0)
}
