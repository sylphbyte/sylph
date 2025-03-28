package sylph

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/sylphbyte/pr"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// MysqlConfig MySQL配置结构体
type MysqlConfig struct {
	Debug       bool        `yaml:"debug"`
	LogMode     int         `yaml:"log_mode"`
	Host        string      `yaml:"host"`
	Port        int         `yaml:"port"`
	Username    string      `yaml:"username"`
	Password    string      `yaml:"password"`
	Database    string      `yaml:"database"`
	Charset     string      `yaml:"charset"`
	MaxIdleConn int         `yaml:"max_idle_conn"`
	MaxOpenConn int         `yaml:"max_open_conn"`
	MaxLifeTime int         `yaml:"max_life_time"`
	Proxy       ProxyConfig `yaml:"proxy"` // 代理配置
}

// RedisConfig Redis配置结构体
type RedisConfig struct {
	Host     string      `yaml:"host"`
	Port     int         `yaml:"port"`
	Password string      `yaml:"password"`
	Database int         `yaml:"database"`
	Proxy    ProxyConfig `yaml:"proxy"` // 代理配置
}

// ESConfig Elasticsearch配置结构体
type ESConfig struct {
	Addresses    []string    `yaml:"addresses"`     // ES服务器地址，如 ["http://localhost:9200"]
	Username     string      `yaml:"username"`      // 用户名
	Password     string      `yaml:"password"`      // 密码
	CloudID      string      `yaml:"cloud_id"`      // 云ID，用于Elastic Cloud
	APIKey       string      `yaml:"api_key"`       // API密钥
	EnableHTTPS  bool        `yaml:"enable_https"`  // 是否启用HTTPS
	SkipVerify   bool        `yaml:"skip_verify"`   // 是否跳过证书验证
	MaxRetries   int         `yaml:"max_retries"`   // 最大重试次数
	RetryTimeout int         `yaml:"retry_timeout"` // 重试超时时间(秒)
	Proxy        ProxyConfig `yaml:"proxy"`         // 代理配置
}

// StorageConfigs 存储配置结构体
type StorageConfigs struct {
	MysqlGroup map[string]MysqlConfig `yaml:"mysql_group" mapstructure:"mysql_group"`
	RedisGroup map[string]RedisConfig `yaml:"redis_group" mapstructure:"redis_group"`
	ESGroup    map[string]ESConfig    `yaml:"es_group" mapstructure:"es_group"`
}

// Hash 计算MySQL配置的哈希值，用于标识相同配置
func (c MysqlConfig) Hash() string {
	return fmt.Sprintf("%s:%d:%s:%s:%s:%d:%d:%d:%s:%d:%s:%d:%s:%s",
		c.Host, c.Port, c.Username, c.Password, c.Database,
		c.MaxIdleConn, c.MaxOpenConn, c.MaxLifeTime, c.Charset, c.LogMode,
		c.Proxy.Type, c.Proxy.Port, c.Proxy.Username, c.Proxy.Host)
}

// Hash 计算Redis配置的哈希值，用于标识相同配置
func (c RedisConfig) Hash() string {
	return fmt.Sprintf("%s:%d:%s:%d:%s:%d:%s:%s",
		c.Host, c.Port, c.Password, c.Database,
		c.Proxy.Type, c.Proxy.Port, c.Proxy.Username, c.Proxy.Host)
}

// Hash 计算ES配置的哈希值，用于标识相同配置
func (c ESConfig) Hash() string {
	addresses := strings.Join(c.Addresses, ",")
	return fmt.Sprintf("%s:%s:%s:%s:%s:%t:%t:%d:%d:%s:%d:%s:%s",
		addresses, c.Username, c.Password, c.CloudID, c.APIKey,
		c.EnableHTTPS, c.SkipVerify, c.MaxRetries, c.RetryTimeout,
		c.Proxy.Type, c.Proxy.Port, c.Proxy.Username, c.Proxy.Host)
}

// InitializeStorageConfigs 初始化存储服务
func InitializeStorageConfigs(configs *StorageConfigs, enabledMysql, enabledRedis, enabledES map[string]bool) (*StorageManagerImpl, error) {
	// 创建存储管理器
	storageManager := NewStorageManager()

	// 获取连接管理器实例
	connManager := GetStorageConnectionManager()

	for name, config := range configs.MysqlGroup {
		// 检查是否启用
		if enabled, exists := enabledMysql[name]; !exists || !enabled {
			pr.Warning("MySQL数据库 %s 未启用", name)
			continue
		}

		// 从连接管理器获取连接
		db, err := connManager.GetMysqlConnection(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化MySQL连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterDB(name, NewMysqlStorage(name, db)); err != nil {
			pr.Error("初始化MySQL连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("MySQL数据库 %s 已注册到存储管理器", name)
	}

	for name, config := range configs.RedisGroup {
		// 检查是否启用
		if enabled, exists := enabledRedis[name]; !exists || !enabled {
			pr.Warning("Redis %s 未启用", name)
			continue
		}

		// 从连接管理器获取连接
		rds, err := connManager.GetRedisConnection(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Redis连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterRedis(name, NewRedisStorage(name, rds)); err != nil {
			pr.Error("初始化Redis连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("Redis %s 已注册到存储管理器", name)
	}

	// 初始化Elasticsearch连接
	for name, config := range configs.ESGroup {
		// 检查是否启用
		if enabled, exists := enabledES[name]; !exists || !enabled {
			pr.Warning("Elasticsearch %s 未启用", name)
			continue
		}

		// 从连接管理器获取连接
		es, err := connManager.GetESConnection(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Elasticsearch连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterES(name, NewESStorage(name, es)); err != nil {
			pr.Error("初始化Elasticsearch连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("Elasticsearch %s 已注册到存储管理器", name)
	}

	return storageManager, nil
}

// InitializeStorage 初始化存储服务
func InitializeStorage(configPath string, enabledMysql, enabledRedis map[string]bool, enabledES map[string]bool) (*StorageManagerImpl, error) {
	pr.System("初始化存储服务，配置文件: %s", configPath)

	// 直接使用viper解析配置
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrapf(err, "读取配置文件失败: %s", configPath)
	}

	// 创建存储管理器
	storageManager := NewStorageManager()

	// 获取连接管理器实例
	connManager := GetStorageConnectionManager()

	// 初始化MySQL连接
	mysqlConfigs := v.GetStringMap("mysql_group")
	for name, _ := range mysqlConfigs {
		// 检查是否启用
		if enabled, exists := enabledMysql[name]; !exists || !enabled {
			pr.Warning("MySQL数据库 %s 未启用", name)
			continue
		}

		// 获取MySQL配置
		mysqlConfig := MysqlConfig{
			Debug:       v.GetBool(fmt.Sprintf("mysql_group.%s.debug", name)),
			LogMode:     v.GetInt(fmt.Sprintf("mysql_group.%s.log_mode", name)),
			Host:        v.GetString(fmt.Sprintf("mysql_group.%s.host", name)),
			Port:        v.GetInt(fmt.Sprintf("mysql_group.%s.port", name)),
			Username:    v.GetString(fmt.Sprintf("mysql_group.%s.username", name)),
			Password:    v.GetString(fmt.Sprintf("mysql_group.%s.password", name)),
			Database:    v.GetString(fmt.Sprintf("mysql_group.%s.database", name)),
			Charset:     v.GetString(fmt.Sprintf("mysql_group.%s.charset", name)),
			MaxIdleConn: v.GetInt(fmt.Sprintf("mysql_group.%s.max_idle_conn", name)),
			MaxOpenConn: v.GetInt(fmt.Sprintf("mysql_group.%s.max_open_conn", name)),
			MaxLifeTime: v.GetInt(fmt.Sprintf("mysql_group.%s.max_life_time", name)),
			Proxy: ProxyConfig{
				Type:     ProxyType(v.GetString(fmt.Sprintf("mysql_group.%s.proxy.type", name))),
				Host:     v.GetString(fmt.Sprintf("mysql_group.%s.proxy.host", name)),
				Port:     v.GetInt(fmt.Sprintf("mysql_group.%s.proxy.port", name)),
				Username: v.GetString(fmt.Sprintf("mysql_group.%s.proxy.username", name)),
				Password: v.GetString(fmt.Sprintf("mysql_group.%s.proxy.password", name)),
				SSHKey:   v.GetString(fmt.Sprintf("mysql_group.%s.proxy.ssh_key", name)),
			},
		}

		// 从连接管理器获取连接
		db, err := connManager.GetMysqlConnection(mysqlConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化MySQL连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterDB(name, NewMysqlStorage(name, db)); err != nil {
			pr.Error("初始化MySQL连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("MySQL数据库 %s 已注册到存储管理器", name)
	}

	// 初始化Redis连接
	redisConfigs := v.GetStringMap("redis_group")
	for name, _ := range redisConfigs {
		// 检查是否启用
		if enabled, exists := enabledRedis[name]; !exists || !enabled {
			pr.Warning("Redis %s 未启用", name)
			continue
		}

		// 获取Redis配置
		redisConfig := RedisConfig{
			Host:     v.GetString(fmt.Sprintf("redis_group.%s.host", name)),
			Port:     v.GetInt(fmt.Sprintf("redis_group.%s.port", name)),
			Password: v.GetString(fmt.Sprintf("redis_group.%s.password", name)),
			Database: v.GetInt(fmt.Sprintf("redis_group.%s.database", name)),
			Proxy: ProxyConfig{
				Type:     ProxyType(v.GetString(fmt.Sprintf("redis_group.%s.proxy.type", name))),
				Host:     v.GetString(fmt.Sprintf("redis_group.%s.proxy.host", name)),
				Port:     v.GetInt(fmt.Sprintf("redis_group.%s.proxy.port", name)),
				Username: v.GetString(fmt.Sprintf("redis_group.%s.proxy.username", name)),
				Password: v.GetString(fmt.Sprintf("redis_group.%s.proxy.password", name)),
				SSHKey:   v.GetString(fmt.Sprintf("redis_group.%s.proxy.ssh_key", name)),
			},
		}

		// 从连接管理器获取连接
		rds, err := connManager.GetRedisConnection(redisConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Redis连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterRedis(name, NewRedisStorage(name, rds)); err != nil {
			pr.Error("初始化Redis连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("Redis %s 已注册到存储管理器", name)
	}

	// 初始化Elasticsearch连接
	esConfigs := v.GetStringMap("es_group")
	for name, _ := range esConfigs {
		// 检查是否启用
		if enabled, exists := enabledES[name]; !exists || !enabled {
			pr.Warning("Elasticsearch %s 未启用", name)
			continue
		}

		// 获取ES配置
		var addresses []string
		if addrs := v.GetStringSlice(fmt.Sprintf("es_group.%s.addresses", name)); len(addrs) > 0 {
			addresses = addrs
		} else {
			// 兼容单个地址的配置
			if addr := v.GetString(fmt.Sprintf("es_group.%s.address", name)); addr != "" {
				addresses = []string{addr}
			}
		}

		esConfig := ESConfig{
			Addresses:    addresses,
			Username:     v.GetString(fmt.Sprintf("es_group.%s.username", name)),
			Password:     v.GetString(fmt.Sprintf("es_group.%s.password", name)),
			CloudID:      v.GetString(fmt.Sprintf("es_group.%s.cloud_id", name)),
			APIKey:       v.GetString(fmt.Sprintf("es_group.%s.api_key", name)),
			EnableHTTPS:  v.GetBool(fmt.Sprintf("es_group.%s.enable_https", name)),
			SkipVerify:   v.GetBool(fmt.Sprintf("es_group.%s.skip_verify", name)),
			MaxRetries:   v.GetInt(fmt.Sprintf("es_group.%s.max_retries", name)),
			RetryTimeout: v.GetInt(fmt.Sprintf("es_group.%s.retry_timeout", name)),
			Proxy: ProxyConfig{
				Type:     ProxyType(v.GetString(fmt.Sprintf("es_group.%s.proxy.type", name))),
				Host:     v.GetString(fmt.Sprintf("es_group.%s.proxy.host", name)),
				Port:     v.GetInt(fmt.Sprintf("es_group.%s.proxy.port", name)),
				Username: v.GetString(fmt.Sprintf("es_group.%s.proxy.username", name)),
				Password: v.GetString(fmt.Sprintf("es_group.%s.proxy.password", name)),
				SSHKey:   v.GetString(fmt.Sprintf("es_group.%s.proxy.ssh_key", name)),
			},
		}

		// 从连接管理器获取连接
		es, err := connManager.GetESConnection(esConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Elasticsearch连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterES(name, NewESStorage(name, es)); err != nil {
			pr.Error("初始化Elasticsearch连接失败: %s, err: %v", name, err)
			continue
		}

		pr.System("Elasticsearch %s 已注册到存储管理器", name)
	}

	return storageManager, nil
}

// InitMysql 初始化MySQL连接
func InitMysql(config MysqlConfig) (*gorm.DB, error) {
	pr.System("初始化MySQL连接: %s@%s:%d/%s", config.Username, config.Host, config.Port, config.Database)

	// 构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		config.Username, config.Password, config.Host, config.Port, config.Database, config.Charset)

	// 创建代理
	if config.Proxy.Type != "" && config.Proxy.Type != ProxyTypeDirect {
		pr.System("MySQL使用代理连接: %s://%s:%d", config.Proxy.Type, config.Proxy.Host, config.Proxy.Port)

		proxy, err := ProxyFactory(config.Proxy)
		if err != nil {
			return nil, errors.Wrap(err, "创建代理失败")
		}
		defer proxy.Close()

		// 对于SSH代理，创建隧道并更改连接地址
		if config.Proxy.Type == ProxyTypeSSH {
			localPort, err := proxy.CreateTunnel(config.Host, config.Port)
			if err != nil {
				return nil, errors.Wrap(err, "创建代理隧道失败")
			}

			// 更新DSN使用本地端口
			dsn = fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s?charset=%s&parseTime=True&loc=Local",
				config.Username, config.Password, localPort, config.Database, config.Charset)

			pr.System("MySQL通过SSH隧道连接: 127.0.0.1:%d -> %s:%d", localPort, config.Host, config.Port)
		}
		// 其他代理类型目前不支持MySQL
	}

	// 日志级别设置
	var logLevel logger.LogLevel
	switch config.LogMode {
	case 1:
		logLevel = logger.Silent
	case 2:
		logLevel = logger.Error
	case 3:
		logLevel = logger.Warn
	case 4:
		logLevel = logger.Info
	default:
		logLevel = logger.Error
	}

	// GORM配置
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 使用单数表名
		},
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "连接MySQL数据库失败: %s", dsn)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "获取底层SQL DB失败")
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(config.MaxIdleConn)
	sqlDB.SetMaxOpenConns(config.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(config.MaxLifeTime) * time.Second)

	pr.System("MySQL连接初始化成功: %s", config.Database)
	return db, nil
}

// InitRedis 初始化Redis连接
func InitRedis(config RedisConfig) (*redis.Client, error) {
	pr.System("初始化Redis连接: %s:%d/%d", config.Host, config.Port, config.Database)

	// 创建Redis客户端选项
	options := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.Database,
	}

	var proxy Proxy
	// 创建代理
	if config.Proxy.Type != "" && config.Proxy.Type != ProxyTypeDirect {
		pr.System("Redis使用代理连接: %s://%s:%d", config.Proxy.Type, config.Proxy.Host, config.Proxy.Port)

		var err error
		proxy, err = ProxyFactory(config.Proxy)
		if err != nil {
			return nil, errors.Wrap(err, "创建代理失败")
		}

		// 根据代理类型设置连接选项
		if config.Proxy.Type == ProxyTypeSSH {
			// 使用SSH隧道
			localPort, err := proxy.CreateTunnel(config.Host, config.Port)
			if err != nil {
				proxy.Close()
				return nil, errors.Wrap(err, "创建SSH隧道失败")
			}
			options.Addr = fmt.Sprintf("127.0.0.1:%d", localPort)
			pr.System("Redis通过SSH隧道连接: 127.0.0.1:%d -> %s:%d", localPort, config.Host, config.Port)
		} else {
			// 使用拨号器
			dialer, err := proxy.GetDialer()
			if err != nil {
				proxy.Close()
				return nil, errors.Wrap(err, "获取代理拨号器失败")
			}
			options.Dialer = dialer
		}
	}

	// 创建Redis客户端
	redisClient := redis.NewClient(options)

	// 如果使用了代理，添加关闭钩子
	if proxy != nil {
		redisClient.AddHook(&RedisProxyHook{proxy: proxy})
	}

	// 测试连接
	ctx := context.Background()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		if proxy != nil {
			proxy.Close()
		}
		return nil, errors.Wrapf(err, "连接Redis失败: %s", options.Addr)
	}

	pr.System("Redis连接初始化成功: %s", options.Addr)
	return redisClient, nil
}

// InitES 初始化Elasticsearch连接
func InitES(config ESConfig) (*elasticsearch.Client, error) {
	// 打印连接信息
	addressesStr := strings.Join(config.Addresses, ", ")
	pr.System("初始化Elasticsearch连接: %s", addressesStr)

	// 创建ES配置
	esConfig := elasticsearch.Config{
		Addresses: config.Addresses,
	}

	// 设置基本认证
	if config.Username != "" && config.Password != "" {
		esConfig.Username = config.Username
		esConfig.Password = config.Password
	}

	// 设置Cloud ID
	if config.CloudID != "" {
		esConfig.CloudID = config.CloudID
	}

	// 设置API Key
	if config.APIKey != "" {
		esConfig.APIKey = config.APIKey
	}

	var proxy Proxy
	// 配置HTTP传输
	if config.Proxy.Type != "" && config.Proxy.Type != ProxyTypeDirect {
		pr.System("Elasticsearch使用代理连接: %s://%s:%d", config.Proxy.Type, config.Proxy.Host, config.Proxy.Port)

		var err error
		proxy, err = ProxyFactory(config.Proxy)
		if err != nil {
			return nil, errors.Wrap(err, "创建代理失败")
		}

		// 获取代理HTTP传输
		transport, err := proxy.GetHTTPTransport(config.EnableHTTPS, config.SkipVerify)
		if err != nil {
			proxy.Close()
			return nil, errors.Wrap(err, "获取代理HTTP传输失败")
		}
		esConfig.Transport = transport
	} else if config.EnableHTTPS || config.SkipVerify {
		// 不使用代理但需要TLS配置
		esConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.SkipVerify,
			},
		}
	}

	// 设置重试选项
	if config.MaxRetries > 0 {
		esConfig.MaxRetries = config.MaxRetries
	}

	if config.RetryTimeout > 0 {
		esConfig.RetryBackoff = func(attempt int) time.Duration {
			return time.Duration(attempt*config.RetryTimeout) * time.Second
		}
	}

	// 创建ES客户端
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		if proxy != nil {
			proxy.Close()
		}
		return nil, errors.Wrap(err, "创建Elasticsearch客户端失败")
	}

	// 测试连接
	info, err := client.Info()
	if err != nil {
		if proxy != nil {
			proxy.Close()
		}
		return nil, errors.Wrap(err, "连接Elasticsearch失败")
	}
	defer info.Body.Close()

	// 注意：这里没有处理代理的关闭问题，在实际使用中可能需要一个管理机制

	pr.System("Elasticsearch连接初始化成功: %s", addressesStr)
	return client, nil
}
