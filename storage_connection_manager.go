package sylph

import (
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/sylphbyte/pr"
	"gorm.io/gorm"
)

// ConnectionState 表示连接状态
type ConnectionState int

const (
	// ConnectionStateActive 活跃状态
	ConnectionStateActive ConnectionState = iota
	// ConnectionStateIdle 空闲状态
	ConnectionStateIdle
	// ConnectionStateError 错误状态
	ConnectionStateError
)

// ConnectionMetadata 连接元数据
type ConnectionMetadata struct {
	Hash        string          // 配置哈希值
	CreateTime  time.Time       // 创建时间
	LastUseTime time.Time       // 最后使用时间
	UseCount    int64           // 使用次数
	State       ConnectionState // 当前状态
	Error       string          // 错误信息（如果有）
}

// StorageConnection 存储连接接口
type StorageConnection interface {
	// GetMetadata 获取连接元数据
	GetMetadata() *ConnectionMetadata

	// GetInstance 获取实际连接实例
	GetInstance() interface{}

	// Close 关闭连接
	Close() error
}

// MysqlConnection MySQL连接封装
type MysqlConnection struct {
	instance *gorm.DB
	metadata *ConnectionMetadata
}

// RedisConnection Redis连接封装
type RedisConnection struct {
	instance *redis.Client
	metadata *ConnectionMetadata
}

// ESConnection Elasticsearch连接封装
type ESConnection struct {
	instance *elasticsearch.Client
	metadata *ConnectionMetadata
}

// GetMetadata 获取MySQL连接元数据
func (c *MysqlConnection) GetMetadata() *ConnectionMetadata {
	return c.metadata
}

// GetInstance 获取MySQL连接实例
func (c *MysqlConnection) GetInstance() interface{} {
	return c.instance
}

// Close 关闭MySQL连接
func (c *MysqlConnection) Close() error {
	if c.instance == nil {
		return nil
	}

	sqlDB, err := c.instance.DB()
	if err != nil {
		return errors.Wrap(err, "获取SQL DB失败")
	}

	return sqlDB.Close()
}

// GetMetadata 获取Redis连接元数据
func (c *RedisConnection) GetMetadata() *ConnectionMetadata {
	return c.metadata
}

// GetInstance 获取Redis连接实例
func (c *RedisConnection) GetInstance() interface{} {
	return c.instance
}

// Close 关闭Redis连接
func (c *RedisConnection) Close() error {
	if c.instance == nil {
		return nil
	}

	return c.instance.Close()
}

// GetMetadata 获取ES连接元数据
func (c *ESConnection) GetMetadata() *ConnectionMetadata {
	return c.metadata
}

// GetInstance 获取ES连接实例
func (c *ESConnection) GetInstance() interface{} {
	return c.instance
}

// Close 关闭ES连接
func (c *ESConnection) Close() error {
	// Elasticsearch客户端没有显式的关闭方法
	return nil
}

// StorageConnectionManagerConfig 存储连接管理器配置
type StorageConnectionManagerConfig struct {
	// 最大空闲时间，超过此时间未使用的连接将被关闭
	MaxIdleTime time.Duration

	// 连接健康检查间隔
	HealthCheckInterval time.Duration

	// 是否启用连接统计
	EnableStats bool
}

// DefaultStorageConnectionManagerConfig 默认配置
var DefaultStorageConnectionManagerConfig = StorageConnectionManagerConfig{
	MaxIdleTime:         time.Minute * 30,
	HealthCheckInterval: time.Minute * 5,
	EnableStats:         true,
}

// StorageConnectionManager 存储连接管理器
// 负责管理并复用多种存储连接，自动检测和关闭空闲连接
type StorageConnectionManager struct {
	// 连接映射，按配置哈希分组
	mysqlConnections map[string]*MysqlConnection
	redisConnections map[string]*RedisConnection
	esConnections    map[string]*ESConnection

	// 配置信息
	config StorageConnectionManagerConfig

	// 锁，保证并发安全
	mutex sync.RWMutex

	// 统计信息
	stats struct {
		// 累计缓存命中次数
		CacheHits int64

		// 累计缓存未命中次数
		CacheMisses int64

		// 累计创建的连接数
		ConnectionsCreated int64

		// 累计关闭的连接数
		ConnectionsClosed int64
	}

	// 关闭标志
	closed     bool
	closedChan chan struct{}
}

// 全局唯一的存储连接管理器实例
var (
	defaultConnectionManager     *StorageConnectionManager
	defaultConnectionManagerOnce sync.Once
)

// GetStorageConnectionManager 获取存储连接管理器实例
func GetStorageConnectionManager() *StorageConnectionManager {
	defaultConnectionManagerOnce.Do(func() {
		defaultConnectionManager = NewStorageConnectionManager(DefaultStorageConnectionManagerConfig)
	})
	return defaultConnectionManager
}

// NewStorageConnectionManager 创建新的存储连接管理器
func NewStorageConnectionManager(config StorageConnectionManagerConfig) *StorageConnectionManager {
	manager := &StorageConnectionManager{
		mysqlConnections: make(map[string]*MysqlConnection),
		redisConnections: make(map[string]*RedisConnection),
		esConnections:    make(map[string]*ESConnection),
		config:           config,
		closedChan:       make(chan struct{}),
	}

	// 启动后台健康检查和清理任务
	go manager.backgroundCleanup()

	return manager
}

// 后台清理任务
func (m *StorageConnectionManager) backgroundCleanup() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupIdleConnections()
		case <-m.closedChan:
			return
		}
	}
}

// 清理空闲连接
func (m *StorageConnectionManager) cleanupIdleConnections() {
	now := time.Now()
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 清理MySQL连接
	for hash, conn := range m.mysqlConnections {
		if conn.metadata.State == ConnectionStateIdle &&
			now.Sub(conn.metadata.LastUseTime) > m.config.MaxIdleTime {
			if err := conn.Close(); err != nil {
				pr.Warning("关闭MySQL连接失败: %v", err)
			}
			delete(m.mysqlConnections, hash)
			pr.System("已关闭空闲MySQL连接: %s", hash)
			m.stats.ConnectionsClosed++
		}
	}

	// 清理Redis连接
	for hash, conn := range m.redisConnections {
		if conn.metadata.State == ConnectionStateIdle &&
			now.Sub(conn.metadata.LastUseTime) > m.config.MaxIdleTime {
			if err := conn.Close(); err != nil {
				pr.Warning("关闭Redis连接失败: %v", err)
			}
			delete(m.redisConnections, hash)
			pr.System("已关闭空闲Redis连接: %s", hash)
			m.stats.ConnectionsClosed++
		}
	}

	// 清理ES连接
	for hash, conn := range m.esConnections {
		if conn.metadata.State == ConnectionStateIdle &&
			now.Sub(conn.metadata.LastUseTime) > m.config.MaxIdleTime {
			if err := conn.Close(); err != nil {
				pr.Warning("关闭ES连接失败: %v", err)
			}
			delete(m.esConnections, hash)
			pr.System("已关闭空闲ES连接: %s", hash)
			m.stats.ConnectionsClosed++
		}
	}
}

// Close 关闭连接管理器及所有连接
func (m *StorageConnectionManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	close(m.closedChan)

	// 关闭所有MySQL连接
	for hash, conn := range m.mysqlConnections {
		if err := conn.Close(); err != nil {
			pr.Warning("关闭MySQL连接失败: %v", err)
		}
		delete(m.mysqlConnections, hash)
		m.stats.ConnectionsClosed++
	}

	// 关闭所有Redis连接
	for hash, conn := range m.redisConnections {
		if err := conn.Close(); err != nil {
			pr.Warning("关闭Redis连接失败: %v", err)
		}
		delete(m.redisConnections, hash)
		m.stats.ConnectionsClosed++
	}

	// 关闭所有ES连接
	for hash, conn := range m.esConnections {
		if err := conn.Close(); err != nil {
			pr.Warning("关闭ES连接失败: %v", err)
		}
		delete(m.esConnections, hash)
		m.stats.ConnectionsClosed++
	}

	return nil
}

// GetMysqlConnection 获取MySQL连接
func (m *StorageConnectionManager) GetMysqlConnection(config MysqlConfig) (*gorm.DB, error) {
	configHash := config.Hash()

	// 先尝试从缓存获取
	m.mutex.RLock()
	conn, exists := m.mysqlConnections[configHash]
	m.mutex.RUnlock()

	if exists {
		// 更新元数据
		m.mutex.Lock()
		conn.metadata.LastUseTime = time.Now()
		conn.metadata.UseCount++
		conn.metadata.State = ConnectionStateActive
		m.mutex.Unlock()

		if m.config.EnableStats {
			m.stats.CacheHits++
		}

		return conn.instance, nil
	}

	// 缓存未命中，创建新连接
	if m.config.EnableStats {
		m.stats.CacheMisses++
	}

	// 初始化MySQL连接
	db, err := InitMysql(config)
	if err != nil {
		return nil, errors.Wrapf(err, "初始化MySQL连接失败")
	}

	// 创建连接元数据
	metadata := &ConnectionMetadata{
		Hash:        configHash,
		CreateTime:  time.Now(),
		LastUseTime: time.Now(),
		UseCount:    1,
		State:       ConnectionStateActive,
	}

	// 创建连接包装
	newConn := &MysqlConnection{
		instance: db,
		metadata: metadata,
	}

	// 保存到缓存
	m.mutex.Lock()
	m.mysqlConnections[configHash] = newConn
	m.stats.ConnectionsCreated++
	m.mutex.Unlock()

	pr.System("创建新的MySQL连接，配置哈希: %s", configHash)
	return db, nil
}

// GetRedisConnection 获取Redis连接
func (m *StorageConnectionManager) GetRedisConnection(config RedisConfig) (*redis.Client, error) {
	configHash := config.Hash()

	// 先尝试从缓存获取
	m.mutex.RLock()
	conn, exists := m.redisConnections[configHash]
	m.mutex.RUnlock()

	if exists {
		// 更新元数据
		m.mutex.Lock()
		conn.metadata.LastUseTime = time.Now()
		conn.metadata.UseCount++
		conn.metadata.State = ConnectionStateActive
		m.mutex.Unlock()

		if m.config.EnableStats {
			m.stats.CacheHits++
		}

		return conn.instance, nil
	}

	// 缓存未命中，创建新连接
	if m.config.EnableStats {
		m.stats.CacheMisses++
	}

	// 初始化Redis连接
	client, err := InitRedis(config)
	if err != nil {
		return nil, errors.Wrapf(err, "初始化Redis连接失败")
	}

	// 创建连接元数据
	metadata := &ConnectionMetadata{
		Hash:        configHash,
		CreateTime:  time.Now(),
		LastUseTime: time.Now(),
		UseCount:    1,
		State:       ConnectionStateActive,
	}

	// 创建连接包装
	newConn := &RedisConnection{
		instance: client,
		metadata: metadata,
	}

	// 保存到缓存
	m.mutex.Lock()
	m.redisConnections[configHash] = newConn
	m.stats.ConnectionsCreated++
	m.mutex.Unlock()

	pr.System("创建新的Redis连接，配置哈希: %s", configHash)
	return client, nil
}

// GetESConnection 获取ES连接
func (m *StorageConnectionManager) GetESConnection(config ESConfig) (*elasticsearch.Client, error) {
	configHash := config.Hash()

	// 先尝试从缓存获取
	m.mutex.RLock()
	conn, exists := m.esConnections[configHash]
	m.mutex.RUnlock()

	if exists {
		// 更新元数据
		m.mutex.Lock()
		conn.metadata.LastUseTime = time.Now()
		conn.metadata.UseCount++
		conn.metadata.State = ConnectionStateActive
		m.mutex.Unlock()

		if m.config.EnableStats {
			m.stats.CacheHits++
		}

		return conn.instance, nil
	}

	// 缓存未命中，创建新连接
	if m.config.EnableStats {
		m.stats.CacheMisses++
	}

	// 初始化ES连接
	client, err := InitES(config)
	if err != nil {
		return nil, errors.Wrapf(err, "初始化ES连接失败")
	}

	// 创建连接元数据
	metadata := &ConnectionMetadata{
		Hash:        configHash,
		CreateTime:  time.Now(),
		LastUseTime: time.Now(),
		UseCount:    1,
		State:       ConnectionStateActive,
	}

	// 创建连接包装
	newConn := &ESConnection{
		instance: client,
		metadata: metadata,
	}

	// 保存到缓存
	m.mutex.Lock()
	m.esConnections[configHash] = newConn
	m.stats.ConnectionsCreated++
	m.mutex.Unlock()

	pr.System("创建新的ES连接，配置哈希: %s", configHash)
	return client, nil
}

// MarkConnectionIdle 将连接标记为空闲状态
func (m *StorageConnectionManager) MarkConnectionIdle(configHash string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查MySQL连接
	if conn, exists := m.mysqlConnections[configHash]; exists {
		conn.metadata.State = ConnectionStateIdle
		return
	}

	// 检查Redis连接
	if conn, exists := m.redisConnections[configHash]; exists {
		conn.metadata.State = ConnectionStateIdle
		return
	}

	// 检查ES连接
	if conn, exists := m.esConnections[configHash]; exists {
		conn.metadata.State = ConnectionStateIdle
		return
	}
}

// Stats 获取统计信息
type ConnectionStats struct {
	TotalConnections   int     // 当前连接总数
	MySQLConnections   int     // MySQL连接数
	RedisConnections   int     // Redis连接数
	ESConnections      int     // ES连接数
	ConnectionsCreated int64   // 累计创建的连接数
	ConnectionsClosed  int64   // 累计关闭的连接数
	CacheHits          int64   // 缓存命中次数
	CacheMisses        int64   // 缓存未命中次数
	HitRatio           float64 // 命中率
}

// GetStats 获取连接管理器统计信息
func (m *StorageConnectionManager) GetStats() ConnectionStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := ConnectionStats{
		MySQLConnections:   len(m.mysqlConnections),
		RedisConnections:   len(m.redisConnections),
		ESConnections:      len(m.esConnections),
		ConnectionsCreated: m.stats.ConnectionsCreated,
		ConnectionsClosed:  m.stats.ConnectionsClosed,
		CacheHits:          m.stats.CacheHits,
		CacheMisses:        m.stats.CacheMisses,
	}

	stats.TotalConnections = stats.MySQLConnections + stats.RedisConnections + stats.ESConnections

	// 计算命中率
	totalRequests := stats.CacheHits + stats.CacheMisses
	if totalRequests > 0 {
		stats.HitRatio = float64(stats.CacheHits) / float64(totalRequests)
	}

	return stats
}

// RemoveConnection 从管理器中移除指定的连接
func (m *StorageConnectionManager) RemoveConnection(configHash string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查并关闭MySQL连接
	if conn, exists := m.mysqlConnections[configHash]; exists {
		err := conn.Close()
		delete(m.mysqlConnections, configHash)
		m.stats.ConnectionsClosed++
		pr.System("已移除MySQL连接: %s", configHash)
		return err
	}

	// 检查并关闭Redis连接
	if conn, exists := m.redisConnections[configHash]; exists {
		err := conn.Close()
		delete(m.redisConnections, configHash)
		m.stats.ConnectionsClosed++
		pr.System("已移除Redis连接: %s", configHash)
		return err
	}

	// 检查并关闭ES连接
	if conn, exists := m.esConnections[configHash]; exists {
		err := conn.Close()
		delete(m.esConnections, configHash)
		m.stats.ConnectionsClosed++
		pr.System("已移除ES连接: %s", configHash)
		return err
	}

	return errors.Errorf("未找到哈希为 %s 的连接", configHash)
}

// CloseAllConnections 关闭所有连接
func (m *StorageConnectionManager) CloseAllConnections() error {
	return m.Close()
}
