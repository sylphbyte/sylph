package sylph

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
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
		return NewSSHProxy(config)
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

// SSHProxy SSH隧道代理
type SSHProxy struct {
	config         ProxyConfig
	client         *ssh.Client
	tunnels        map[string]int // 目标地址 -> 本地端口
	tunnelListener map[string]net.Listener
	lock           sync.Mutex
	minPort        int // 最小可用端口
	maxPort        int // 最大可用端口
}

// NewSSHProxy 创建SSH代理
func NewSSHProxy(config ProxyConfig) (*SSHProxy, error) {
	// 验证配置
	if config.Host == "" {
		return nil, errors.New("SSH代理主机不能为空")
	}
	if config.Port <= 0 {
		config.Port = 22 // 默认SSH端口
	}

	proxy := &SSHProxy{
		config:         config,
		tunnels:        make(map[string]int),
		tunnelListener: make(map[string]net.Listener),
		minPort:        10000, // 默认端口范围
		maxPort:        20000,
	}

	// 连接SSH服务器
	if err := proxy.connect(); err != nil {
		return nil, err
	}

	return proxy, nil
}

// connect 连接到SSH服务器
func (p *SSHProxy) connect() error {
	// 创建SSH配置
	sshConfig := &ssh.ClientConfig{
		User:            p.config.Username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 在生产环境中应使用ssh.FixedHostKey或其他安全方法
		Timeout:         30 * time.Second,
	}

	// 添加认证方式
	authMethodsAdded := false

	if p.config.Password != "" {
		pr.System("使用密码认证方式")
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(p.config.Password))
		authMethodsAdded = true
	}

	// 如果提供了密钥文件，尝试使用密钥认证
	if p.config.SSHKey != "" {
		pr.System("使用SSH密钥文件: %s", p.config.SSHKey)

		// 检查密钥文件是否存在
		_, err := os.Stat(p.config.SSHKey)
		if os.IsNotExist(err) {
			// 尝试向后兼容，检查配置文件中是否使用了错误的字段名
			pr.Warning("SSH密钥文件不存在: %s，尝试检查配置", p.config.SSHKey)
			return errors.Errorf("SSH密钥文件不存在: %s，请检查配置文件中的ssh_key字段", p.config.SSHKey)
		} else if err != nil {
			return errors.Wrapf(err, "检查SSH密钥文件状态失败: %s", p.config.SSHKey)
		}

		key, err := ioutil.ReadFile(p.config.SSHKey)
		if err != nil {
			return errors.Wrapf(err, "读取SSH密钥文件失败: %s", p.config.SSHKey)
		}
		pr.System("成功读取SSH密钥文件，大小: %d字节", len(key))

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return errors.Wrapf(err, "解析SSH私钥失败: %s", p.config.SSHKey)
		}
		pr.System("成功解析SSH私钥")

		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
		authMethodsAdded = true
	} else {
		pr.Warning("未指定SSH密钥文件，将尝试使用其他认证方式")
	}

	if !authMethodsAdded {
		return errors.New("没有提供任何认证方式（密码或SSH密钥）")
	}

	pr.System("SSH配置中的认证方法数量: %d", len(sshConfig.Auth))

	// 连接到SSH服务器
	pr.System("正在连接SSH服务器 %s:%d...", p.config.Host, p.config.Port)
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", p.config.Host, p.config.Port), sshConfig)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no supported methods remain") {
			return errors.Wrapf(err, "SSH认证失败: 服务器不支持配置的认证方法，请检查密钥文件或认证方式")
		}
		if strings.Contains(errMsg, "unable to authenticate") {
			return errors.Wrapf(err, "SSH认证失败: 认证凭据无效，请检查用户名、密码或密钥是否正确")
		}
		return errors.Wrapf(err, "连接SSH服务器失败: %s:%d", p.config.Host, p.config.Port)
	}

	p.client = client
	pr.System("成功连接到SSH代理服务器: %s:%d", p.config.Host, p.config.Port)
	return nil
}

// GetDialer 获取SSH代理拨号器
func (p *SSHProxy) GetDialer() (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if p.client == nil {
		return nil, errors.New("SSH客户端未连接")
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// 通过SSH连接创建到目标地址的连接
		return p.client.Dial(network, addr)
	}, nil
}

// CreateTunnel 创建SSH隧道
func (p *SSHProxy) CreateTunnel(targetHost string, targetPort int) (int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// 生成目标地址标识
	targetAddr := fmt.Sprintf("%s:%d", targetHost, targetPort)

	// 检查是否已有隧道
	if localPort, exists := p.tunnels[targetAddr]; exists {
		return localPort, nil
	}

	// 查找可用的本地端口
	var listener net.Listener
	var localPort int
	var err error

	// 尝试在端口范围内找到可用端口
	for port := p.minPort; port <= p.maxPort; port++ {
		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			localPort = port
			break
		}
	}

	if listener == nil {
		return 0, errors.New("无法找到可用的本地端口创建SSH隧道")
	}

	// 保存隧道信息
	p.tunnels[targetAddr] = localPort
	p.tunnelListener[targetAddr] = listener

	// 启动转发
	go p.forwardTunnel(targetHost, targetPort, listener)

	pr.System("创建SSH隧道: 本地127.0.0.1:%d -> %s:%d", localPort, targetHost, targetPort)
	return localPort, nil
}

// forwardTunnel 转发SSH隧道
func (p *SSHProxy) forwardTunnel(targetHost string, targetPort int, listener net.Listener) {
	defer listener.Close()

	for {
		// 接受本地连接
		local, err := listener.Accept()
		if err != nil {
			// 检查是否因为关闭而退出
			if errors.Is(err, net.ErrClosed) {
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

		// 双向转发数据
		go func() {
			defer local.Close()
			defer remote.Close()

			// 从本地到远程
			go io.Copy(remote, local)
			// 从远程到本地
			io.Copy(local, remote)
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

	// 关闭所有隧道监听器
	for addr, listener := range p.tunnelListener {
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
