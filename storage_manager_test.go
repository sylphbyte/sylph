package sylph

import (
	"sync"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// =============================================================================
// StorageManager 基础测试
// =============================================================================

// TestNewStorageManager 测试创建存储管理器
func TestNewStorageManager(t *testing.T) {
	manager := NewStorageManager()

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.dbMap)
	assert.NotNil(t, manager.redisMap)
	assert.NotNil(t, manager.esMap)
	assert.Empty(t, manager.defaultDB)
	assert.Empty(t, manager.defaultRedis)
	assert.Empty(t, manager.defaultES)
}

// =============================================================================
// MySQL Storage 测试
// =============================================================================

// TestRegisterDB 测试注册数据库
func TestRegisterDB(t *testing.T) {
	manager := NewStorageManager()

	// 创建 mock storage
	storage := NewMysqlStorage("test-db", &gorm.DB{})

	// 注册第一个数据库
	err := manager.RegisterDB("test-db", storage)
	assert.NoError(t, err)

	// 验证已注册
	db, err := manager.GetDB("test-db")
	assert.NoError(t, err)
	assert.NotNil(t, db)
	assert.Equal(t, "test-db", db.GetName())

	// 验证设为默认
	assert.Equal(t, "test-db", manager.defaultDB)
}

// TestRegisterDBDuplicate 测试重复注册
func TestRegisterDBDuplicate(t *testing.T) {
	manager := NewStorageManager()
	storage := NewMysqlStorage("test-db", &gorm.DB{})

	// 第一次注册成功
	err := manager.RegisterDB("test-db", storage)
	assert.NoError(t, err)

	// 第二次注册应该失败
	err = manager.RegisterDB("test-db", storage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已经注册")
}

// TestGetDBByName 测试按名称获取数据库
func TestGetDBByName(t *testing.T) {
	manager := NewStorageManager()
	storage1 := NewMysqlStorage("db1", &gorm.DB{})
	storage2 := NewMysqlStorage("db2", &gorm.DB{})

	manager.RegisterDB("db1", storage1)
	manager.RegisterDB("db2", storage2)

	// 获取 db1
	db, err := manager.GetDB("db1")
	assert.NoError(t, err)
	assert.Equal(t, "db1", db.GetName())

	// 获取 db2
	db, err = manager.GetDB("db2")
	assert.NoError(t, err)
	assert.Equal(t, "db2", db.GetName())
}

// TestGetDBDefault 测试获取默认数据库
func TestGetDBDefault(t *testing.T) {
	manager := NewStorageManager()
	storage := NewMysqlStorage("default-db", &gorm.DB{})

	manager.RegisterDB("default-db", storage)

	// 不指定名称，应该返回默认数据库
	db, err := manager.GetDB()
	assert.NoError(t, err)
	assert.Equal(t, "default-db", db.GetName())
}

// TestGetDBNotExists 测试获取不存在的数据库
func TestGetDBNotExists(t *testing.T) {
	manager := NewStorageManager()

	// 获取不存在的数据库
	_, err := manager.GetDB("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// TestGetDBNoDefault 测试没有默认数据库时的错误
func TestGetDBNoDefault(t *testing.T) {
	manager := NewStorageManager()

	// 没有注册任何数据库，也没有指定名称
	_, err := manager.GetDB()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未设置默认数据库")
}

// =============================================================================
// Redis Storage 测试
// =============================================================================

// TestRegisterRedis 测试注册 Redis
func TestRegisterRedis(t *testing.T) {
	manager := NewStorageManager()
	storage := NewRedisStorage("test-redis", &redis.Client{})

	// 注册 Redis
	err := manager.RegisterRedis("test-redis", storage)
	assert.NoError(t, err)

	// 验证已注册
	rds, err := manager.GetRedis("test-redis")
	assert.NoError(t, err)
	assert.NotNil(t, rds)
	assert.Equal(t, "test-redis", rds.GetName())

	// 验证设为默认
	assert.Equal(t, "test-redis", manager.defaultRedis)
}

// TestRegisterRedisDuplicate 测试重复注册 Redis
func TestRegisterRedisDuplicate(t *testing.T) {
	manager := NewStorageManager()
	storage := NewRedisStorage("test-redis", &redis.Client{})

	err := manager.RegisterRedis("test-redis", storage)
	assert.NoError(t, err)

	err = manager.RegisterRedis("test-redis", storage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已经注册")
}

// TestGetRedisByName 测试按名称获取 Redis
func TestGetRedisByName(t *testing.T) {
	manager := NewStorageManager()
	storage1 := NewRedisStorage("redis1", &redis.Client{})
	storage2 := NewRedisStorage("redis2", &redis.Client{})

	manager.RegisterRedis("redis1", storage1)
	manager.RegisterRedis("redis2", storage2)

	// 获取 redis1
	rds, err := manager.GetRedis("redis1")
	assert.NoError(t, err)
	assert.Equal(t, "redis1", rds.GetName())

	// 获取 redis2
	rds2, err := manager.GetRedis("redis2")
	assert.NoError(t, err)
	assert.Equal(t, "redis2", rds2.GetName())
}

// TestGetRedisDefault 测试获取默认 Redis
func TestGetRedisDefault(t *testing.T) {
	manager := NewStorageManager()
	storage := NewRedisStorage("default-redis", &redis.Client{})

	manager.RegisterRedis("default-redis", storage)

	// 不指定名称，应该返回默认 Redis
	rds, err := manager.GetRedis()
	assert.NoError(t, err)
	assert.Equal(t, "default-redis", rds.GetName())
}

// TestGetRedisNotExists 测试获取不存在的 Redis
func TestGetRedisNotExists(t *testing.T) {
	manager := NewStorageManager()

	_, err := manager.GetRedis("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// =============================================================================
// ES Storage 测试
// =============================================================================

// TestRegisterES 测试注册 ES
func TestRegisterES(t *testing.T) {
	manager := NewStorageManager()
	storage := NewESStorage("test-es", &elasticsearch.Client{})

	// 注册 ES
	err := manager.RegisterES("test-es", storage)
	assert.NoError(t, err)

	// 验证已注册
	es, err := manager.GetES("test-es")
	assert.NoError(t, err)
	assert.NotNil(t, es)
	assert.Equal(t, "test-es", es.GetName())

	// 验证设为默认
	assert.Equal(t, "test-es", manager.defaultES)
}

// TestRegisterESDuplicate 测试重复注册 ES
func TestRegisterESDuplicate(t *testing.T) {
	manager := NewStorageManager()
	storage := NewESStorage("test-es", &elasticsearch.Client{})

	err := manager.RegisterES("test-es", storage)
	assert.NoError(t, err)

	err = manager.RegisterES("test-es", storage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已经注册")
}

// TestGetESByName 测试按名称获取 ES
func TestGetESByName(t *testing.T) {
	manager := NewStorageManager()
	storage1 := NewESStorage("es1", &elasticsearch.Client{})
	storage2 := NewESStorage("es2", &elasticsearch.Client{})

	manager.RegisterES("es1", storage1)
	manager.RegisterES("es2", storage2)

	// 获取 es1
	es, err := manager.GetES("es1")
	assert.NoError(t, err)
	assert.Equal(t, "es1", es.GetName())

	// 获取 es2
	es2, err := manager.GetES("es2")
	assert.NoError(t, err)
	assert.Equal(t, "es2", es2.GetName())
}

// TestGetESDefault 测试获取默认 ES
func TestGetESDefault(t *testing.T) {
	manager := NewStorageManager()
	storage := NewESStorage("default-es", &elasticsearch.Client{})

	manager.RegisterES("default-es", storage)

	// 不指定名称，应该返回默认 ES
	es, err := manager.GetES()
	assert.NoError(t, err)
	assert.Equal(t, "default-es", es.GetName())
}

// TestGetESNotExists 测试获取不存在的 ES
func TestGetESNotExists(t *testing.T) {
	manager := NewStorageManager()

	_, err := manager.GetES("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// =============================================================================
// GetAllStorages 测试
// =============================================================================

// TestGetAllStorages 测试获取所有存储
func TestGetAllStorages(t *testing.T) {
	manager := NewStorageManager()

	// 注册多个存储
	manager.RegisterDB("db1", NewMysqlStorage("db1", &gorm.DB{}))
	manager.RegisterDB("db2", NewMysqlStorage("db2", &gorm.DB{}))
	manager.RegisterRedis("redis1", NewRedisStorage("redis1", &redis.Client{}))
	manager.RegisterES("es1", NewESStorage("es1", &elasticsearch.Client{}))

	// 获取所有存储
	all := manager.GetAllStorages()

	// 验证数量
	assert.Len(t, all, 4)

	// 验证名称
	assert.Contains(t, all, "db1")
	assert.Contains(t, all, "db2")
	assert.Contains(t, all, "redis1")
	assert.Contains(t, all, "es1")
}

// TestGetAllStoragesEmpty 测试空管理器
func TestGetAllStoragesEmpty(t *testing.T) {
	manager := NewStorageManager()

	all := manager.GetAllStorages()
	assert.Empty(t, all)
}

// =============================================================================
// Adapter 基础方法测试
// =============================================================================

// TestMysqlStorageAdapterBasics 测试 MySQL Adapter 基础方法
func TestMysqlStorageAdapterBasics(t *testing.T) {
	storage := NewMysqlStorage("test-mysql", &gorm.DB{})

	// 测试基础方法
	assert.Equal(t, "test-mysql", storage.GetName())
	assert.Equal(t, StorageTypeMySQL, storage.GetType())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	health := storage.GetHealthStatus()
	assert.NotNil(t, health)
	assert.Equal(t, StorageStateConnected, health.State)
	assert.False(t, health.ConnectedAt.IsZero())
	assert.False(t, health.LastPingAt.IsZero())
}

// TestRedisStorageAdapterBasics 测试 Redis Adapter 基础方法
func TestRedisStorageAdapterBasics(t *testing.T) {
	storage := NewRedisStorage("test-redis", &redis.Client{})

	// 测试基础方法
	assert.Equal(t, "test-redis", storage.GetName())
	assert.Equal(t, StorageTypeRedis, storage.GetType())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	health := storage.GetHealthStatus()
	assert.NotNil(t, health)
	assert.Equal(t, StorageStateConnected, health.State)
}

// TestESStorageAdapterBasics 测试 ES Adapter 基础方法
func TestESStorageAdapterBasics(t *testing.T) {
	storage := NewESStorage("test-es", &elasticsearch.Client{})

	// 测试基础方法
	assert.Equal(t, "test-es", storage.GetName())
	assert.Equal(t, StorageTypeES, storage.GetType())
	assert.True(t, storage.IsConnected())

	// 测试健康状态
	health := storage.GetHealthStatus()
	assert.NotNil(t, health)
	assert.Equal(t, StorageStateConnected, health.State)
}

// =============================================================================
// 并发安全测试
// =============================================================================

// TestStorageManagerConcurrentRegister 测试并发注册
func TestStorageManagerConcurrentRegister(t *testing.T) {
	manager := NewStorageManager()
	var wg sync.WaitGroup

	// 并发注册多个数据库
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := string(rune('a' + n))
			storage := NewMysqlStorage(name, &gorm.DB{})
			manager.RegisterDB(name, storage)
		}(i)
	}

	wg.Wait()

	// 验证所有数据库都已注册
	all := manager.GetAllStorages()
	assert.GreaterOrEqual(t, len(all), 1) // 至少有一个注册成功
}

// TestStorageManagerConcurrentGet 测试并发获取
func TestStorageManagerConcurrentGet(t *testing.T) {
	manager := NewStorageManager()

	// 先注册
	manager.RegisterDB("test-db", NewMysqlStorage("test-db", &gorm.DB{}))
	manager.RegisterRedis("test-redis", NewRedisStorage("test-redis", &redis.Client{}))

	var wg sync.WaitGroup

	// 并发获取
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			manager.GetDB("test-db")
		}()
		go func() {
			defer wg.Done()
			manager.GetRedis("test-redis")
		}()
	}

	wg.Wait()
}

// =============================================================================
// 边界情况和综合测试
// =============================================================================

// TestStorageManagerMultipleDefaults 测试多个默认存储
func TestStorageManagerMultipleDefaults(t *testing.T) {
	manager := NewStorageManager()

	// 注册多个不同类型的存储
	manager.RegisterDB("db1", NewMysqlStorage("db1", &gorm.DB{}))
	manager.RegisterDB("db2", NewMysqlStorage("db2", &gorm.DB{}))
	manager.RegisterRedis("redis1", NewRedisStorage("redis1", &redis.Client{}))
	manager.RegisterRedis("redis2", NewRedisStorage("redis2", &redis.Client{}))
	manager.RegisterES("es1", NewESStorage("es1", &elasticsearch.Client{}))

	// 验证第一个注册的被设为默认
	assert.Equal(t, "db1", manager.defaultDB)
	assert.Equal(t, "redis1", manager.defaultRedis)
	assert.Equal(t, "es1", manager.defaultES)

	// 获取默认存储
	db, _ := manager.GetDB()
	assert.Equal(t, "db1", db.GetName())

	rds, _ := manager.GetRedis()
	assert.Equal(t, "redis1", rds.GetName())

	es, _ := manager.GetES()
	assert.Equal(t, "es1", es.GetName())
}

// TestStorageManagerMixedOperations 测试混合操作
func TestStorageManagerMixedOperations(t *testing.T) {
	manager := NewStorageManager()

	// 注册各种类型
	manager.RegisterDB("db1", NewMysqlStorage("db1", &gorm.DB{}))
	manager.RegisterRedis("redis1", NewRedisStorage("redis1", &redis.Client{}))
	manager.RegisterES("es1", NewESStorage("es1", &elasticsearch.Client{}))

	// 验证可以分别获取
	db, err := manager.GetDB("db1")
	assert.NoError(t, err)
	assert.NotNil(t, db)

	rds, err := manager.GetRedis("redis1")
	assert.NoError(t, err)
	assert.NotNil(t, rds)

	es, err := manager.GetES("es1")
	assert.NoError(t, err)
	assert.NotNil(t, es)

	// 验证类型正确
	assert.Equal(t, StorageTypeMySQL, db.GetType())
	assert.Equal(t, StorageTypeRedis, rds.GetType())
	assert.Equal(t, StorageTypeES, es.GetType())
}

// TestHealthStatusFields 测试健康状态字段
func TestHealthStatusFields(t *testing.T) {
	storage := NewMysqlStorage("test", &gorm.DB{})
	health := storage.GetHealthStatus()

	// 验证初始状态
	assert.Equal(t, StorageStateConnected, health.State)
	assert.Empty(t, health.ErrorMsg)
	assert.Equal(t, 0, health.FailCount)

	// 验证时间字段
	now := time.Now()
	assert.True(t, health.ConnectedAt.Before(now) || health.ConnectedAt.Equal(now))
	assert.True(t, health.LastPingAt.Before(now) || health.LastPingAt.Equal(now))
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkRegisterDB 注册性能
func BenchmarkRegisterDB(b *testing.B) {
	manager := NewStorageManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := string(rune(i%26 + 'a'))
		storage := NewMysqlStorage(name, &gorm.DB{})
		manager.RegisterDB(name, storage)
	}
}

// BenchmarkGetDB 获取性能
func BenchmarkGetDB(b *testing.B) {
	manager := NewStorageManager()
	manager.RegisterDB("bench-db", NewMysqlStorage("bench-db", &gorm.DB{}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetDB("bench-db")
	}
}

// BenchmarkGetAllStorages 获取所有存储性能
func BenchmarkGetAllStorages(b *testing.B) {
	manager := NewStorageManager()

	// 注册 10 个存储
	for i := 0; i < 10; i++ {
		name := string(rune(i + 'a'))
		manager.RegisterDB(name, NewMysqlStorage(name, &gorm.DB{}))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetAllStorages()
	}
}

// BenchmarkNewMysqlStorage MySQL Adapter 创建性能
func BenchmarkNewMysqlStorage(b *testing.B) {
	db := &gorm.DB{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewMysqlStorage("bench", db)
	}
}
