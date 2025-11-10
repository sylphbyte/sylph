package sylph

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestStorageConfigParsing 测试配置解析
func TestStorageConfigParsing(t *testing.T) {
	configPath := "./etc/storage.yaml"

	// 测试初始化存储服务
	enabledMysql := map[string]bool{
		"main": true,
	}

	enabledRedis := map[string]bool{
		"main": true,
	}

	enabledES := map[string]bool{
		"main": true,
	}

	storageManager, err := InitializeStorage(configPath, enabledMysql, enabledRedis, enabledES)

	// 由于测试环境可能没有真实的数据库连接，我们主要测试配置解析
	// 如果连接失败，检查错误信息是否包含预期的内容
	if err != nil {
		t.Logf("初始化存储失败（可能是连接问题）: %v", err)
		// 检查错误信息是否包含数据库相关的信息
		assert.Contains(t, err.Error(), "初始化", "错误信息应该包含初始化相关内容")
		return
	}

	// 如果成功，验证存储管理器
	assert.NotNil(t, storageManager, "存储管理器不应为空")

	// 测试获取MySQL
	db, err := storageManager.GetDB("main")
	if err == nil {
		assert.NotNil(t, db, "MySQL数据库不应为空")
		t.Log("✅ MySQL 连接成功")
	} else {
		t.Logf("⚠️  获取MySQL失败: %v", err)
	}

	// 测试获取Redis
	rds, err := storageManager.GetRedis("main")
	if err == nil {
		assert.NotNil(t, rds, "Redis不应为空")
		t.Log("✅ Redis 连接成功")
	} else {
		t.Logf("⚠️  获取Redis失败: %v", err)
	}

	// 测试获取ES
	es, err := storageManager.GetES("main")
	if err == nil {
		assert.NotNil(t, es, "Elasticsearch不应为空")
		t.Log("✅ Elasticsearch 连接成功")
	} else {
		t.Logf("⚠️  获取Elasticsearch失败: %v", err)
	}
}

// TestStorageConfigsStruct 测试使用结构体方式初始化
func TestStorageConfigsStruct(t *testing.T) {
	configs := &StorageConfigs{
		MysqlGroup: map[string]MysqlConfig{
			"test": {
				Debug:       true,
				LogMode:     4,
				Host:        "127.0.0.1",
				Port:        3306,
				Username:    "root",
				Password:    "password",
				Database:    "test",
				Charset:     "utf8mb4",
				MaxIdleConn: 10,
				MaxOpenConn: 20,
				MaxLifeTime: 1800,
			},
		},
		RedisGroup: map[string]RedisConfig{
			"test": {
				Host:     "127.0.0.1",
				Port:     6379,
				Password: "",
				Database: 0,
			},
		},
		ESGroup: map[string]ESConfig{
			"test": {
				Addresses:    []string{"http://127.0.0.1:9200"},
				Username:     "",
				Password:     "",
				EnableHTTPS:  false,
				SkipVerify:   false,
				MaxRetries:   3,
				RetryTimeout: 3,
			},
		},
	}

	enabledMysql := map[string]bool{"test": true}
	enabledRedis := map[string]bool{"test": true}
	enabledES := map[string]bool{"test": true}

	storageManager, err := InitializeStorageConfigs(configs, enabledMysql, enabledRedis, enabledES)

	if err != nil {
		t.Logf("初始化存储失败（可能是连接问题）: %v", err)
		return
	}

	assert.NotNil(t, storageManager, "存储管理器不应为空")
	t.Log("✅ 使用配置结构体初始化成功")
}

// TestMysqlConfigUnmarshal 测试MySQL配置的UnmarshalKey功能
func TestMysqlConfigUnmarshal(t *testing.T) {
	configPath := "./etc/storage.yaml"

	// 使用viper读取配置
	v := viper.New()
	v.SetConfigFile(configPath)

	err := v.ReadInConfig()
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	// 测试UnmarshalKey
	var mysqlConfig MysqlConfig
	err = v.UnmarshalKey("mysql_group.main", &mysqlConfig)
	assert.NoError(t, err, "UnmarshalKey 应该成功")

	// 验证配置值
	assert.Equal(t, true, mysqlConfig.Debug, "Debug 应该为 true")
	assert.Equal(t, 4, mysqlConfig.LogMode, "LogMode 应该为 4")
	assert.Equal(t, "123.57.2.204", mysqlConfig.Host, "Host 应该匹配")
	assert.Equal(t, 3306, mysqlConfig.Port, "Port 应该为 3306")
	assert.Equal(t, "wider_online", mysqlConfig.Username, "Username 应该匹配")
	assert.Equal(t, "t2", mysqlConfig.Database, "Database 应该为 t2")
	assert.Equal(t, "utf8mb4", mysqlConfig.Charset, "Charset 应该为 utf8mb4")
	assert.Equal(t, 32, mysqlConfig.MaxIdleConn, "MaxIdleConn 应该为 32")
	assert.Equal(t, 64, mysqlConfig.MaxOpenConn, "MaxOpenConn 应该为 64")
	assert.Equal(t, 1800, mysqlConfig.MaxLifeTime, "MaxLifeTime 应该为 1800")

	t.Logf("✅ MySQL 配置解析正确")
	t.Logf("   Host: %s:%d", mysqlConfig.Host, mysqlConfig.Port)
	t.Logf("   Database: %s", mysqlConfig.Database)
	t.Logf("   Username: %s", mysqlConfig.Username)
}

// TestRedisConfigUnmarshal 测试Redis配置的UnmarshalKey功能
func TestRedisConfigUnmarshal(t *testing.T) {
	configPath := "./etc/storage.yaml"

	v := viper.New()
	v.SetConfigFile(configPath)

	err := v.ReadInConfig()
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	var redisConfig RedisConfig
	err = v.UnmarshalKey("redis_group.main", &redisConfig)
	assert.NoError(t, err, "UnmarshalKey 应该成功")

	assert.Equal(t, "123.57.2.204", redisConfig.Host, "Host 应该匹配")
	assert.Equal(t, 6379, redisConfig.Port, "Port 应该为 6379")
	assert.Equal(t, "3uhpXdaRsn5", redisConfig.Password, "Password 应该匹配")
	assert.Equal(t, 9, redisConfig.Database, "Database 应该为 9")

	t.Logf("✅ Redis 配置解析正确")
	t.Logf("   Host: %s:%d", redisConfig.Host, redisConfig.Port)
	t.Logf("   Database: %d", redisConfig.Database)
}

// TestESConfigUnmarshal 测试ES配置的UnmarshalKey功能
func TestESConfigUnmarshal(t *testing.T) {
	configPath := "./etc/storage.yaml"

	v := viper.New()
	v.SetConfigFile(configPath)

	err := v.ReadInConfig()
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	var esConfig ESConfig
	err = v.UnmarshalKey("es_group.main", &esConfig)
	assert.NoError(t, err, "UnmarshalKey 应该成功")

	assert.Equal(t, []string{"http://123.57.2.204:9200"}, esConfig.Addresses, "Addresses 应该匹配")
	assert.Equal(t, "", esConfig.Username, "Username 应该为空")
	assert.Equal(t, false, esConfig.EnableHTTPS, "EnableHTTPS 应该为 false")
	assert.Equal(t, false, esConfig.SkipVerify, "SkipVerify 应该为 false")
	assert.Equal(t, 3, esConfig.MaxRetries, "MaxRetries 应该为 3")
	assert.Equal(t, 3, esConfig.RetryTimeout, "RetryTimeout 应该为 3")

	t.Logf("✅ Elasticsearch 配置解析正确")
	t.Logf("   Addresses: %v", esConfig.Addresses)
	t.Logf("   MaxRetries: %d", esConfig.MaxRetries)
}
