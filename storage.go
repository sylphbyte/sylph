package sylph

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// MysqlConfig MySQL配置结构体
type MysqlConfig struct {
	Debug       bool   `yaml:"debug" mapstructure:"debug"`
	LogMode     int    `yaml:"log_mode" mapstructure:"log_mode"`
	Host        string `yaml:"host" mapstructure:"host"`
	Port        int    `yaml:"port" mapstructure:"port"`
	Username    string `yaml:"username" mapstructure:"username"`
	Password    string `yaml:"password" mapstructure:"password"`
	Database    string `yaml:"database" mapstructure:"database"`
	Charset     string `yaml:"charset" mapstructure:"charset"`
	MaxIdleConn int    `yaml:"max_idle_conn" mapstructure:"max_idle_conn"`
	MaxOpenConn int    `yaml:"max_open_conn" mapstructure:"max_open_conn"`
	MaxLifeTime int    `yaml:"max_life_time" mapstructure:"max_life_time"`
}

// RedisConfig Redis配置结构体
type RedisConfig struct {
	Host             string `yaml:"host" mapstructure:"host"`
	Port             int    `yaml:"port" mapstructure:"port"`
	Password         string `yaml:"password" mapstructure:"password"`
	Database         int    `yaml:"database" mapstructure:"database"`
	Debug            bool   `yaml:"debug" mapstructure:"debug"`
	Protocol         int    `yaml:"protocol" mapstructure:"protocol"`                   // redis协议
	DisableIndentity bool   `yaml:"disable_indentity" mapstructure:"disable_indentity"` // 不发送CLIENT SETINFO
}

// ESConfig Elasticsearch配置结构体
type ESConfig struct {
	Addresses    []string `yaml:"addresses" mapstructure:"addresses"`         // ES服务器地址，如 ["http://localhost:9200"]
	Username     string   `yaml:"username" mapstructure:"username"`           // 用户名
	Password     string   `yaml:"password" mapstructure:"password"`           // 密码
	CloudID      string   `yaml:"cloud_id" mapstructure:"cloud_id"`           // 云ID，用于Elastic Cloud
	APIKey       string   `yaml:"api_key" mapstructure:"api_key"`             // API密钥
	EnableHTTPS  bool     `yaml:"enable_https" mapstructure:"enable_https"`   // 是否启用HTTPS
	SkipVerify   bool     `yaml:"skip_verify" mapstructure:"skip_verify"`     // 是否跳过证书验证
	MaxRetries   int      `yaml:"max_retries" mapstructure:"max_retries"`     // 最大重试次数
	RetryTimeout int      `yaml:"retry_timeout" mapstructure:"retry_timeout"` // 重试超时时间(秒)
}

// StorageConfigs 存储配置结构体
type StorageConfigs struct {
	MysqlGroup map[string]MysqlConfig `yaml:"mysql_group" mapstructure:"mysql_group"`
	RedisGroup map[string]RedisConfig `yaml:"redis_group" mapstructure:"redis_group"`
	ESGroup    map[string]ESConfig    `yaml:"es_group" mapstructure:"es_group"`
}

// InitializeStorageConfigs 初始化存储服务
func InitializeStorageConfigs(configs *StorageConfigs, enabledMysql, enabledRedis, enabledES map[string]bool) (*StorageManagerImpl, error) {
	// 创建存储管理器
	storageManager := NewStorageManager()

	for name, config := range configs.MysqlGroup {
		// 检查是否启用
		if enabled, exists := enabledMysql[name]; !exists || !enabled {
			printWarning("MySQL数据库 %s 未启用", name)
			continue
		}

		db, err := InitMysql(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化MySQL连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterDB(name, NewMysqlStorage(name, db)); err != nil {
			printError("初始化MySQL连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("MySQL数据库 %s 已注册到存储管理器", name)
	}

	for name, config := range configs.RedisGroup {
		// 检查是否启用
		if enabled, exists := enabledRedis[name]; !exists || !enabled {
			printWarning("Redis %s 未启用", name)
			continue
		}

		// 从连接管理器获取连接
		rds, err := InitRedis(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Redis连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterRedis(name, NewRedisStorage(name, rds)); err != nil {
			printError("初始化Redis连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("Redis %s 已注册到存储管理器", name)
	}

	// 初始化Elasticsearch连接
	for name, config := range configs.ESGroup {
		// 检查是否启用
		if enabled, exists := enabledES[name]; !exists || !enabled {
			printWarning("Elasticsearch %s 未启用", name)
			continue
		}

		// 从连接管理器获取连接
		es, err := InitES(config)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Elasticsearch连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterES(name, NewESStorage(name, es)); err != nil {
			printError("初始化Elasticsearch连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("Elasticsearch %s 已注册到存储管理器", name)
	}

	return storageManager, nil
}

// InitializeStorage 初始化存储服务
func InitializeStorage(configPath string, enabledMysql, enabledRedis map[string]bool, enabledES map[string]bool) (*StorageManagerImpl, error) {
	printSystem("初始化存储服务，配置文件: %s", configPath)

	// 直接使用viper解析配置
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrapf(err, "读取配置文件失败: %s", configPath)
	}

	// 创建存储管理器
	storageManager := NewStorageManager()

	// 初始化MySQL连接
	mysqlConfigs := v.GetStringMap("mysql_group")
	for name := range mysqlConfigs {
		// 检查是否启用
		if enabled, exists := enabledMysql[name]; !exists || !enabled {
			printWarning("MySQL数据库 %s 未启用", name)
			continue
		}

		// 使用 UnmarshalKey 自动读取配置
		var mysqlConfig MysqlConfig
		if err := v.UnmarshalKey(fmt.Sprintf("mysql_group.%s", name), &mysqlConfig); err != nil {
			return nil, errors.Wrapf(err, "解析MySQL配置失败: %s", name)
		}

		// 从连接管理器获取连接
		db, err := InitMysql(mysqlConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化MySQL连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterDB(name, NewMysqlStorage(name, db)); err != nil {
			printError("初始化MySQL连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("MySQL数据库 %s 已注册到存储管理器", name)
	}

	// 初始化Redis连接
	redisConfigs := v.GetStringMap("redis_group")
	for name := range redisConfigs {
		// 检查是否启用
		if enabled, exists := enabledRedis[name]; !exists || !enabled {
			printWarning("Redis %s 未启用", name)
			continue
		}

		// 使用 UnmarshalKey 自动读取配置
		var redisConfig RedisConfig
		if err := v.UnmarshalKey(fmt.Sprintf("redis_group.%s", name), &redisConfig); err != nil {
			return nil, errors.Wrapf(err, "解析Redis配置失败: %s", name)
		}

		// 从连接管理器获取连接
		rds, err := InitRedis(redisConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Redis连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterRedis(name, NewRedisStorage(name, rds)); err != nil {
			printError("初始化Redis连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("Redis %s 已注册到存储管理器", name)
	}

	// 初始化Elasticsearch连接
	esConfigs := v.GetStringMap("es_group")
	for name := range esConfigs {
		// 检查是否启用
		if enabled, exists := enabledES[name]; !exists || !enabled {
			printWarning("Elasticsearch %s 未启用", name)
			continue
		}

		// 使用 UnmarshalKey 自动读取配置
		var esConfig ESConfig
		if err := v.UnmarshalKey(fmt.Sprintf("es_group.%s", name), &esConfig); err != nil {
			return nil, errors.Wrapf(err, "解析Elasticsearch配置失败: %s", name)
		}

		// 从连接管理器获取连接
		es, err := InitES(esConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "初始化Elasticsearch连接失败: %s", name)
		}

		// 注册到存储管理器
		if err = storageManager.RegisterES(name, NewESStorage(name, es)); err != nil {
			printError("初始化Elasticsearch连接失败: %s, err: %v", name, err)
			continue
		}

		printSystem("Elasticsearch %s 已注册到存储管理器", name)
	}

	return storageManager, nil
}

// InitMysql 初始化MySQL连接
func InitMysql(config MysqlConfig) (*gorm.DB, error) {
	printSystem("初始化MySQL连接: %s@%s:%d/%s", config.Username, config.Host, config.Port, config.Database)

	// 构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		config.Username, config.Password, config.Host, config.Port, config.Database, config.Charset)

	// 日志级别映射：1=Silent, 2=Error, 3=Warn, 4=Info
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

	printSystem("MySQL连接初始化成功: %s", config.Database)
	return db, nil
}

// InitRedis 初始化Redis连接
func InitRedis(config RedisConfig) (*redis.Client, error) {
	printSystem("初始化Redis连接: %s:%d/%d", config.Host, config.Port, config.Database)

	// 创建Redis客户端选项
	options := &redis.Options{
		Addr:             fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:         config.Password,
		DB:               config.Database,
		Protocol:         config.Protocol,         // 不尝试 HELLO，直接用 RESP2
		DisableIndentity: config.DisableIndentity, // 不发送 CLIENT SETINFO
	}

	// 创建Redis客户端
	redisClient := redis.NewClient(options)

	// 测试连接
	ctx := context.Background()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		return nil, errors.Wrapf(err, "连接Redis失败: %s", options.Addr)
	}

	if config.Debug {
		redisClient.AddHook(RedisDebugHook{})
	}

	printSystem("Redis连接初始化成功: %s", options.Addr)
	return redisClient, nil
}

// InitES 初始化Elasticsearch连接
func InitES(config ESConfig) (*elasticsearch.Client, error) {
	// 打印连接信息
	addressesStr := strings.Join(config.Addresses, ", ")
	printSystem("初始化Elasticsearch连接: %s", addressesStr)

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

	if config.EnableHTTPS || config.SkipVerify {
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
		return nil, errors.Wrap(err, "创建Elasticsearch客户端失败")
	}

	// 测试连接
	info, err := client.Info()
	if err != nil {
		return nil, errors.Wrap(err, "连接Elasticsearch失败")
	}
	defer info.Body.Close()

	printSystem("Elasticsearch连接初始化成功: %s", addressesStr)
	return client, nil
}
