package sylph

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// ProxyType 代理类型枚举
type ProxyType string

const (
	ProxyTypeDirect ProxyType = "direct" // 直连，不使用代理
	ProxyTypeSSH    ProxyType = "ssh"    // SSH隧道代理
	ProxyTypeSOCKS5 ProxyType = "socks5" // SOCKS5代理
	ProxyTypeHTTP   ProxyType = "http"   // HTTP代理
	ProxyTypeHTTPS  ProxyType = "https"  // HTTPS代理
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	Type     ProxyType `yaml:"type"`                           // 代理类型
	Host     string    `yaml:"host"`                           // 代理服务器地址
	Port     int       `yaml:"port"`                           // 代理服务器端口
	Username string    `yaml:"username"`                       // 代理认证用户名
	Password string    `yaml:"password"`                       // 代理认证密码
	SSHKey   string    `yaml:"ssh_key" mapstructure:"ssh_key"` // SSH代理私钥路径(当type=ssh时使用)
}

// Proxy 代理管理器接口
type Proxy interface {
	// GetDialer 获取支持代理的网络拨号器
	GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error)

	// CreateTunnel 创建到目标地址的隧道
	CreateTunnel(targetHost string, targetPort int) (localPort int, err error)

	// GetHTTPTransport 获取支持代理的HTTP传输
	GetHTTPTransport(enableTLS bool, skipVerify bool) (*http.Transport, error)

	// Close 关闭代理连接和资源
	Close() error
}

// ProxyFactory 创建代理实例的工厂函数
func ProxyFactory(config ProxyConfig) (Proxy, error) {
	if config.Type == "" || config.Type == ProxyTypeDirect {
		return &DirectProxy{}, nil
	}

	switch config.Type {
	case ProxyTypeSSH:
		proxy, err := NewSSHProxy(config)
		if err != nil {
			return nil, err
		}
		// 连接SSH服务器
		if err := proxy.Connect(); err != nil {
			return nil, errors.Wrap(err, "连接SSH服务器失败")
		}
		return proxy, nil
	case ProxyTypeSOCKS5:
		return NewSOCKS5Proxy(config)
	case ProxyTypeHTTP, ProxyTypeHTTPS:
		return NewHTTPProxy(config)
	default:
		return nil, errors.Errorf("不支持的代理类型: %s", config.Type)
	}
}

// DirectProxy 直连代理(不使用代理)
type DirectProxy struct{}

func (p *DirectProxy) GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	return (&net.Dialer{}).DialContext, nil
}

func (p *DirectProxy) CreateTunnel(targetHost string, targetPort int) (int, error) {
	return 0, errors.New("直连模式不支持隧道")
}

func (p *DirectProxy) GetHTTPTransport(enableTLS bool, skipVerify bool) (*http.Transport, error) {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipVerify,
		},
	}, nil
}

func (p *DirectProxy) Close() error {
	return nil
}

// SSHProxy SSH代理
type SSHProxy struct {
	config         ProxyConfig
	client         *ssh.Client
	tunnelListener map[string]net.Listener
	tunnels        map[string]int
	stopKeepAlive  chan struct{}
	lock           sync.RWMutex
	usePortForward bool // 是否使用端口转发
}

// NewSSHProxy 创建SSH代理
func NewSSHProxy(config ProxyConfig) (*SSHProxy, error) {
	if config.Host == "" {
		return nil, errors.New("SSH代理主机不能为空")
	}
	if config.Port <= 0 {
		return nil, errors.New("SSH代理端口无效")
	}

	return &SSHProxy{
		config:         config,
		tunnelListener: make(map[string]net.Listener),
		tunnels:        make(map[string]int),
		usePortForward: true, // 默认使用端口转发
	}, nil
}

// CreateTunnel 创建隧道
func (p *SSHProxy) CreateTunnel(targetHost string, targetPort int) (int, error) {
	if p.client == nil {
		return 0, errors.New("SSH客户端未连接")
	}

	// 如果不使用端口转发，直接返回目标端口
	if !p.usePortForward {
		return targetPort, nil
	}

	// 生成本地监听地址
	addr := fmt.Sprintf("%s:%d", targetHost, targetPort)
	p.lock.Lock()
	if listener, exists := p.tunnelListener[addr]; exists {
		p.lock.Unlock()
		// 如果隧道已存在，返回本地端口
		localAddr := listener.Addr().(*net.TCPAddr)
		return localAddr.Port, nil
	}
	p.lock.Unlock()

	// 在本地随机端口上监听
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, errors.Wrap(err, "创建本地监听失败")
	}

	// 获取分配的本地端口
	localAddr := listener.Addr().(*net.TCPAddr)
	localPort := localAddr.Port

	// 保存监听器
	p.lock.Lock()
	p.tunnelListener[addr] = listener
	p.tunnels[addr] = 0
	p.lock.Unlock()

	pr.System("创建SSH隧道: 本地127.0.0.1:%d -> %s:%d", localPort, targetHost, targetPort)

	// 启动转发
	go p.forwardTunnel(targetHost, targetPort, listener)

	return localPort, nil
}

// GetDialer 获取SSH代理拨号器
func (p *SSHProxy) GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if p.client == nil {
		return nil, errors.New("SSH客户端未连接")
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 如果不使用端口转发，直接通过SSH客户端连接
		if !p.usePortForward {
			return p.client.Dial(network, addr)
		}

		// 解析目标地址
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, errors.Wrapf(err, "解析地址失败: %s", addr)
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, errors.Wrapf(err, "解析端口失败: %s", portStr)
		}

		// 创建隧道
		localPort, err := p.CreateTunnel(host, port)
		if err != nil {
			return nil, errors.Wrap(err, "创建隧道失败")
		}

		// 连接到本地端口
		return net.Dial(network, fmt.Sprintf("127.0.0.1:%d", localPort))
	}, nil
}

// Connect 连接到SSH服务器
func (p *SSHProxy) Connect() error {
	if p.client != nil {
		return nil
	}

	// 创建SSH配置
	config := &ssh.ClientConfig{
		User:            p.config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 设置认证方法
	if p.config.SSHKey != "" {
		// 使用SSH密钥认证
		pr.System("使用SSH密钥文件: %s", p.config.SSHKey)

		// 读取密钥文件
		keyData, err := os.ReadFile(p.config.SSHKey)
		if err != nil {
			return errors.Wrapf(err, "读取SSH密钥文件失败: %s", p.config.SSHKey)
		}
		pr.System("成功读取SSH密钥文件，大小: %d字节", len(keyData))

		// 解析密钥
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return errors.Wrap(err, "解析SSH私钥失败")
		}
		pr.System("成功解析SSH私钥")

		// 添加到认证方法
		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	} else if p.config.Password != "" {
		// 使用密码认证
		config.Auth = []ssh.AuthMethod{
			ssh.Password(p.config.Password),
		}
	} else {
		return errors.New("未提供SSH认证方法")
	}

	pr.System("SSH配置中的认证方法数量: %d", len(config.Auth))

	// 连接到SSH服务器
	pr.System("正在连接SSH服务器 %s:%d...", p.config.Host, p.config.Port)
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", p.config.Host, p.config.Port), config)
	if err != nil {
		return errors.Wrapf(err, "连接SSH服务器失败: %s:%d", p.config.Host, p.config.Port)
	}

	p.client = client
	pr.System("成功连接到SSH代理服务器: %s:%d", p.config.Host, p.config.Port)

	// 启动保活
	p.startKeepAlive()

	// 尝试端口转发，如果失败则禁用
	testListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.Wrap(err, "测试本地监听失败")
	}
	testPort := testListener.Addr().(*net.TCPAddr).Port
	testListener.Close()

	// 尝试通过SSH创建到本地端口的连接
	_, err = client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", testPort))
	if err != nil && strings.Contains(err.Error(), "administratively prohibited") {
		pr.Warning("SSH服务器不支持端口转发，将使用直接转发")
		p.usePortForward = false
	}

	return nil
}

// startKeepAlive 启动SSH保活机制
func (p *SSHProxy) startKeepAlive() {
	if p.client == nil {
		return
	}

	// 如果已有保活goroutine在运行，停止它
	if p.stopKeepAlive != nil {
		close(p.stopKeepAlive)
	}

	p.stopKeepAlive = make(chan struct{})

	// 启动保活goroutine
	go func() {
		keepAliveTicker := time.NewTicker(30 * time.Second) // 每30秒发送一次保活包
		defer keepAliveTicker.Stop()

		for {
			select {
			case <-keepAliveTicker.C:
				// 发送保活请求
				_, _, err := p.client.SendRequest("keepalive@sylph", true, nil)
				if err != nil {
					pr.Warning("SSH保活失败: %v，尝试重新连接", err)
					// 尝试重新连接
					p.reconnect()
					return
				}
				pr.System("已发送SSH保活请求")
			case <-p.stopKeepAlive:
				pr.System("停止SSH保活")
				return
			}
		}
	}()
}

// reconnect 重新连接SSH服务器
func (p *SSHProxy) reconnect() {
	p.lock.Lock()
	defer p.lock.Unlock()

	// 停止保活
	if p.stopKeepAlive != nil {
		close(p.stopKeepAlive)
		p.stopKeepAlive = nil
	}

	// 关闭旧连接
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}

	// 尝试重新连接
	pr.System("尝试重新连接SSH服务器...")
	err := p.Connect()
	if err != nil {
		pr.Error("重新连接SSH服务器失败: %v", err)
	} else {
		pr.System("成功重新连接到SSH服务器")

		// 重建隧道
		p.rebuildTunnels()
	}
}

// rebuildTunnels 重建所有隧道
func (p *SSHProxy) rebuildTunnels() {
	// 保存原有隧道信息
	oldTunnels := make(map[string]int)
	for targetAddr, port := range p.tunnels {
		oldTunnels[targetAddr] = port
	}

	// 清空旧隧道
	for addr, listener := range p.tunnelListener {
		if listener != nil {
			listener.Close()
			pr.System("关闭隧道: %s", addr)
		}
	}
	p.tunnelListener = make(map[string]net.Listener)
	p.tunnels = make(map[string]int)

	// 重建隧道
	for addr, localPort := range oldTunnels {
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			continue
		}
		targetHost := parts[0]
		targetPort := 0
		fmt.Sscanf(parts[1], "%d", &targetPort)
		if targetPort <= 0 {
			continue
		}

		pr.System("重建隧道: 本地127.0.0.1:%d -> %s", localPort, addr)

		// 尝试在相同端口上重建
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if err != nil {
			pr.Warning("在原端口重建隧道失败(目标: %s): %v", addr, err)
			continue
		}

		// 保存隧道信息
		p.tunnels[addr] = localPort
		p.tunnelListener[addr] = listener

		// 启动转发
		go p.forwardTunnel(targetHost, targetPort, listener)
	}
}

// forwardTunnel 转发SSH隧道
func (p *SSHProxy) forwardTunnel(targetHost string, targetPort int, listener net.Listener) {
	defer listener.Close()

	// 创建一个通道来跟踪活动连接
	connDone := make(chan struct{})
	var activeConns sync.WaitGroup

	for {
		// 接受本地连接
		local, err := listener.Accept()
		if err != nil {
			// 检查是否因为关闭而退出
			if errors.Is(err, net.ErrClosed) {
				// 等待所有连接完成
				activeConns.Wait()
				close(connDone)
				return
			}
			pr.Error("接受本地连接失败: %v", err)
			continue
		}

		// 创建到目标地址的连接
		remote, err := p.client.Dial("tcp", fmt.Sprintf("%s:%d", targetHost, targetPort))
		if err != nil {
			local.Close()
			pr.Error("通过SSH创建到目标的连接失败: %s:%d, 错误: %v", targetHost, targetPort, err)
			continue
		}

		// 增加连接计数
		addr := fmt.Sprintf("%s:%d", targetHost, targetPort)
		p.lock.Lock()
		p.tunnels[addr]++
		count := p.tunnels[addr]
		p.lock.Unlock()
		pr.Info("隧道连接计数增加: %s -> %d", addr, count)

		// 增加活动连接计数
		activeConns.Add(1)

		// 双向转发数据
		go func() {
			defer func() {
				local.Close()
				remote.Close()

				// 减少连接计数
				p.lock.Lock()
				p.tunnels[addr]--
				count := p.tunnels[addr]
				p.lock.Unlock()
				pr.Info("隧道连接计数减少: %s -> %d", addr, count)

				// 减少活动连接计数
				activeConns.Done()
			}()

			// 使用WaitGroup等待两个方向的数据传输完成
			var wg sync.WaitGroup
			wg.Add(2)

			// 从本地到远程
			go func() {
				defer wg.Done()
				if _, err := io.Copy(remote, local); err != nil {
					if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
						pr.Error("本地到远程数据传输失败: %v", err)
					}
				}
				// 关闭远程写入端，通知对方我们已完成发送
				remote.(interface{ CloseWrite() error }).CloseWrite()
			}()

			// 从远程到本地
			go func() {
				defer wg.Done()
				if _, err := io.Copy(local, remote); err != nil {
					if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
						pr.Error("远程到本地数据传输失败: %v", err)
					}
				}
				// 关闭本地写入端，通知对方我们已完成发送
				local.(interface{ CloseWrite() error }).CloseWrite()
			}()

			// 等待两个方向的数据传输完成
			wg.Wait()
		}()
	}
}

// GetHTTPTransport 获取支持代理的HTTP传输
func (p *SSHProxy) GetHTTPTransport(enableTLS bool, skipVerify bool) (*http.Transport, error) {
	if p.client == nil {
		return nil, errors.New("SSH客户端未连接")
	}

	// 获取拨号器
	dialer, err := p.GetDialer()
	if err != nil {
		return nil, err
	}

	// 创建传输
	transport := &http.Transport{
		DialContext: dialer,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipVerify,
		},
	}

	return transport, nil
}

// Close 关闭SSH连接和隧道
func (p *SSHProxy) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// 停止保活
	if p.stopKeepAlive != nil {
		close(p.stopKeepAlive)
		p.stopKeepAlive = nil
	}

	// 等待一段时间再关闭隧道，确保连接有时间完成
	time.Sleep(500 * time.Millisecond)

	// 关闭所有隧道监听器
	for addr, listener := range p.tunnelListener {
		// 检查是否有活动连接
		if count := p.tunnels[addr]; count > 0 {
			pr.Info("等待隧道连接完成: %s (活动连接: %d)", addr, count)
			// 等待更长时间，让连接有机会完成
			time.Sleep(2 * time.Second)
		}
		listener.Close()
		delete(p.tunnelListener, addr)
		delete(p.tunnels, addr)
	}

	// 关闭SSH客户端
	if p.client != nil {
		err := p.client.Close()
		p.client = nil
		return err
	}

	return nil
}

// SOCKS5Proxy SOCKS5代理
type SOCKS5Proxy struct {
	config ProxyConfig
	dialer proxy.Dialer
}

// NewSOCKS5Proxy 创建SOCKS5代理
func NewSOCKS5Proxy(config ProxyConfig) (*SOCKS5Proxy, error) {
	if config.Host == "" {
		return nil, errors.New("SOCKS5代理主机不能为空")
	}
	if config.Port <= 0 {
		return nil, errors.New("SOCKS5代理端口无效")
	}

	// 创建SOCKS5代理
	auth := &proxy.Auth{}
	if config.Username != "" && config.Password != "" {
		auth.User = config.Username
		auth.Password = config.Password
	} else {
		auth = nil
	}

	// 创建SOCKS5拨号器
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port), auth, proxy.Direct)
	if err != nil {
		return nil, errors.Wrap(err, "创建SOCKS5代理失败")
	}

	return &SOCKS5Proxy{
		config: config,
		dialer: dialer,
	}, nil
}

// GetDialer 获取SOCKS5代理拨号器
func (p *SOCKS5Proxy) GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if p.dialer == nil {
		return nil, errors.New("SOCKS5代理未初始化")
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 使用上下文超时
		if _, ok := ctx.Deadline(); ok {
			return p.dialer.(proxy.ContextDialer).DialContext(ctx, network, addr)
		}

		// 无上下文超时，使用普通拨号
		return p.dialer.Dial(network, addr)
	}, nil
}

// CreateTunnel 创建隧道
func (p *SOCKS5Proxy) CreateTunnel(targetHost string, targetPort int) (int, error) {
	return 0, errors.New("SOCKS5代理不支持创建隧道，请直接使用代理拨号器")
}

// GetHTTPTransport 获取支持代理的HTTP传输
func (p *SOCKS5Proxy) GetHTTPTransport(enableTLS bool, skipVerify bool) (*http.Transport, error) {
	if p.dialer == nil {
		return nil, errors.New("SOCKS5代理未初始化")
	}

	// 获取拨号器
	dialContext, err := p.GetDialer()
	if err != nil {
		return nil, err
	}

	// 创建传输
	transport := &http.Transport{
		DialContext: dialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipVerify,
		},
	}

	return transport, nil
}

// Close 关闭资源
func (p *SOCKS5Proxy) Close() error {
	// SOCKS5代理没有需要关闭的资源
	p.dialer = nil
	return nil
}

// HTTPProxy HTTP代理
type HTTPProxy struct {
	config    ProxyConfig
	proxyURL  *url.URL
	transport *http.Transport
}

// NewHTTPProxy 创建HTTP代理
func NewHTTPProxy(config ProxyConfig) (*HTTPProxy, error) {
	if config.Host == "" {
		return nil, errors.New("HTTP代理主机不能为空")
	}
	if config.Port <= 0 {
		return nil, errors.New("HTTP代理端口无效")
	}

	// 创建代理URL
	scheme := "http"
	if config.Type == ProxyTypeHTTPS {
		scheme = "https"
	}

	proxyURLStr := fmt.Sprintf("%s://%s:%d", scheme, config.Host, config.Port)
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, errors.Wrap(err, "解析HTTP代理URL失败")
	}

	// 添加认证
	if config.Username != "" && config.Password != "" {
		proxyURL.User = url.UserPassword(config.Username, config.Password)
	}

	return &HTTPProxy{
		config:   config,
		proxyURL: proxyURL,
	}, nil
}

// GetDialer 获取HTTP代理拨号器
func (p *HTTPProxy) GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if p.proxyURL == nil {
		return nil, errors.New("HTTP代理未初始化")
	}

	// 创建传输(如果尚未创建)
	if p.transport == nil {
		var err error
		p.transport, err = p.GetHTTPTransport(false, false)
		if err != nil {
			return nil, err
		}
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 构建URL
		url := &url.URL{
			Scheme: "http",
			Host:   addr,
		}

		// 创建HTTP CONNECT请求
		req, err := http.NewRequestWithContext(ctx, http.MethodConnect, url.String(), nil)
		if err != nil {
			return nil, errors.Wrap(err, "创建HTTP CONNECT请求失败")
		}

		// 执行请求
		resp, err := p.transport.RoundTrip(req)
		if err != nil {
			return nil, errors.Wrap(err, "HTTP CONNECT失败")
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, errors.Errorf("HTTP CONNECT失败，状态码: %d", resp.StatusCode)
		}

		// 获取底层连接
		conn, ok := resp.Body.(net.Conn)
		if !ok {
			resp.Body.Close()
			return nil, errors.New("无法获取HTTP代理底层连接")
		}

		return conn, nil
	}, nil
}

// CreateTunnel 创建隧道
func (p *HTTPProxy) CreateTunnel(targetHost string, targetPort int) (int, error) {
	return 0, errors.New("HTTP代理不支持创建隧道，请直接使用代理拨号器")
}

// GetHTTPTransport 获取支持代理的HTTP传输
func (p *HTTPProxy) GetHTTPTransport(enableTLS bool, skipVerify bool) (*http.Transport, error) {
	if p.proxyURL == nil {
		return nil, errors.New("HTTP代理未初始化")
	}

	// 创建传输
	transport := &http.Transport{
		Proxy: http.ProxyURL(p.proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipVerify,
		},
	}

	p.transport = transport
	return transport, nil
}

// Close 关闭资源
func (p *HTTPProxy) Close() error {
	if p.transport != nil {
		p.transport.CloseIdleConnections()
		p.transport = nil
	}
	return nil
}

// RedisProxyHook Redis代理钩子
type RedisProxyHook struct {
	proxy Proxy
}

// 实现redis.Hook接口
func (h *RedisProxyHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *RedisProxyHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return nil
}

func (h *RedisProxyHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *RedisProxyHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}

func (h *RedisProxyHook) Close() error {
	if h.proxy != nil {
		return h.proxy.Close()
	}
	return nil
}
