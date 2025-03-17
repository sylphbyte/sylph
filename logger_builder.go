package sylph

import (
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// 全局时间格式
const (
	defaultTimestampFormat = "2006-01-02 15:04:05.000"
)

// 格式化器缓存
var formatterCache = sync.Map{}

// 获取共享的格式化器实例
func getSharedFormatter(timestampFormat string) logrus.Formatter {
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	// 使用时间格式作为缓存键
	if cached, ok := formatterCache.Load(timestampFormat); ok {
		return cached.(logrus.Formatter)
	}

	// 创建新的格式化器
	formatter := &XLoggerFormatter{TimestampFormat: timestampFormat}
	formatterCache.Store(timestampFormat, formatter)
	return formatter
}

// LoggerBuilder 日志构建器，负责创建和配置logrus.Logger实例
type LoggerBuilder struct {
	name          string            // 日志记录器名称
	opt           *LoggerConfig     // 日志配置选项
	Formatter     logrus.Formatter  // 日志格式化器
	Hooks         logrus.LevelHooks // 日志钩子
	cachedLoggers sync.Map          // 缓存已创建的日志记录器
	timestampFmt  string            // 时间戳格式
	setupOnce     sync.Once         // 确保初始化只执行一次
	presetHooks   []logrus.Hook     // 预设钩子
}

// LoggerBuilderOption 是Logger构建器的可选配置函数
type LoggerBuilderOption func(*LoggerBuilder)

// WithTimestampFormat 设置时间戳格式
func WithTimestampFormat(format string) LoggerBuilderOption {
	return func(lb *LoggerBuilder) {
		lb.timestampFmt = format
	}
}

// WithPresetHooks 预设钩子
func WithPresetHooks(hooks ...logrus.Hook) LoggerBuilderOption {
	return func(lb *LoggerBuilder) {
		lb.presetHooks = append(lb.presetHooks, hooks...)
	}
}

// WithFormatter 设置自定义格式化器
func WithFormatter(formatter logrus.Formatter) LoggerBuilderOption {
	return func(lb *LoggerBuilder) {
		lb.Formatter = formatter
	}
}

// getCacheKey 生成缓存键
func (l *LoggerBuilder) getCacheKey() string {
	return fmt.Sprintf("%s:%s:%s", l.name, l.opt.Path, l.opt.Level)
}

// NewLoggerBuilder 创建新的日志构建器
func NewLoggerBuilder(name string, opt *LoggerConfig, options ...LoggerBuilderOption) *LoggerBuilder {
	builder := &LoggerBuilder{
		name:         name,
		opt:          opt,
		Hooks:        make(logrus.LevelHooks),
		timestampFmt: defaultTimestampFormat,
	}

	// 应用选项
	for _, option := range options {
		option(builder)
	}

	return builder
}

// setup 执行一次性初始化
func (l *LoggerBuilder) setup() {
	l.setupOnce.Do(func() {
		// 如果没有指定格式化器，则使用共享实例
		if l.Formatter == nil {
			l.Formatter = getSharedFormatter(l.timestampFmt)
		}

		// 添加预设钩子
		for _, hook := range l.presetHooks {
			l.Hooks.Add(hook)
		}

		// 创建标准钩子
		l.MakeHooks()
	})
}

// InjectHook 注入日志钩子
func (l *LoggerBuilder) InjectHook(hook logrus.Hook) {
	if hook != nil {
		l.Hooks.Add(hook)
	}
}

// MakeHooks 创建标准日志钩子
func (l *LoggerBuilder) MakeHooks() {
	// 只有在配置了路径并且缓冲钩子不存在时才创建
	if l.opt.Path != "" {
		// 创建日志缓冲钩子
		l.Hooks.Add(NewLoggerBufferHook(l.name, l.opt))
	}
}

// Make 创建或获取缓存的logrus.Logger实例
func (l *LoggerBuilder) Make() (entity *logrus.Logger) {
	// 执行初始化
	l.setup()

	// 检查缓存
	cacheKey := l.getCacheKey()
	if cached, ok := l.cachedLoggers.Load(cacheKey); ok {
		return cached.(*logrus.Logger)
	}

	// 创建新的Logger实例
	entity = &logrus.Logger{
		Out:       l.opt.stdout(),
		Formatter: l.Formatter,
		Hooks:     l.Hooks,
		Level:     l.opt.level(),
		ExitFunc:  os.Exit,
	}

	// 缓存创建的实例
	l.cachedLoggers.Store(cacheKey, entity)
	return entity
}

// Reset 重置构建器状态
func (l *LoggerBuilder) Reset() {
	l.cachedLoggers = sync.Map{}
	l.Hooks = make(logrus.LevelHooks)
	l.setupOnce = sync.Once{}
}
