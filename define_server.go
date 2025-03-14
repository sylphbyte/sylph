package sylph

const (
	HttpKind Kind = iota + 1
	CrontabKind
	MessageQueueKind
	RedisQueueKind
)

type Kind int

const (
	Gin    Name = "gin"
	Echo   Name = "echo"
	Cron   Name = "cron"
	Rocket Name = "rocket"
	Kafka  Name = "kafka"
)

type Name string

// IProject 服务容器
type IProject interface {
	// Mounts 装载服务
	Mounts(servers ...IServer) IProject

	// Boots 启动服务
	Boots() error

	// Shutdowns 关闭服务
	Shutdowns() error
}

// IServer 服务定义
type IServer interface {

	// Boot 启动服务
	Boot() error

	// Shutdown 关闭服务
	Shutdown() error
}

//
//type Context interface {
//	Info(msg string, data any)
//	Trace(msg string, data any)
//	Debug(msg string, data any)
//	Warn(msg string, data any)
//	Fatal(msg string, data any)
//	Panic(msg string, data any)
//	Error(message string, err error, data any)
//	Clone() Context
//}
