package sylph

// mode_switch 配置示例：
// normal: true  - 启用普通模式（允许并发执行）
// skip: true    - 启用跳过模式（任务仍在执行时跳过新触发）
// delay: true   - 启用延迟模式（等待当前任务完成后执行新触发）

// ModeSwitch 模式开关配置
// 用于在配置文件中设置启用哪些任务执行模式
type ModeSwitch SwitchMode

// ServiceCrontabYaml 定时任务服务YAML配置结构
// 用于从配置文件加载定时任务服务的完整配置
type ServiceCrontabYaml struct {
	ModeSwitch ModeSwitch  `yaml:"mode_switch" mapstructure:"mode_switch"` // 模式开关配置
	Jobs       TaskConfigs `yaml:"jobs"`                                   // 任务配置集合
}

// OpenService 检查是否有任何模式被启用
//
// 返回:
//   - bool: 如果有任何模式被启用则返回true，否则返回false
//
// 使用示例:
//
//	if config.OpenService() {
//	  // 启动定时任务服务
//	}
func (s ServiceCrontabYaml) OpenService() bool {
	for _, open := range s.ModeSwitch {
		if open {
			return true
		}
	}

	return false
}

// TaskName 任务名称类型
// 这里是能扩展的任务标识符，可以给rocket等其他任务相关的地方用
type TaskName string

// Name 获取任务名称的字符串表示
//
// 返回:
//   - string: 任务名称字符串
func (t TaskName) Name() string {
	return string(t)
}

// CrontabRouteFunc 计划任务路由函数类型
// 用于注册和配置计划任务路由
type CrontabRouteFunc func(routes ICrontabRoute)

// ICrontabRoute 计划任务路由接口
// 提供任务注册功能
type ICrontabRoute interface {
	// Register 注册任务处理函数
	//
	// 参数:
	//   - name: 任务名称
	//   - task: 任务处理函数
	Register(name TaskName, task TaskHandler)
	//Receive(name Name) (task TaskHandler, ok bool)
}

// TaskManager 任务管理器
// 用于存储和管理任务处理函数
type TaskManager struct {
	tasks map[TaskName]TaskHandler // 任务名称到处理函数的映射
}

//type ICrontabRoutes interface {
//
//	HasNext() bool
//	Next() *CrontabRoute
//}

// TaskConfigs 任务配置映射
// 按执行模式分组的任务配置集合
type TaskConfigs map[CrontabModeName][]TaskConfig

// TaskConfig 单个任务配置结构
// 定义任务的基本属性和执行计划
type TaskConfig struct {
	Open bool     `yaml:"open"` // 是否启用该任务
	Name TaskName `yaml:"name"` // 任务名称
	Desc string   `yaml:"desc"` // 任务描述
	Spec string   `yaml:"spec"` // cron表达式，定义执行计划
	//Mode CrontabMode `yaml:"mode"` // 任务执行模式
}

// TaskHandler 任务处理函数类型
// 任务的具体执行逻辑
//
// 参数:
//   - ctx: 上下文，提供任务执行环境
//
// 返回:
//   - error: 任务执行错误，nil表示成功
type TaskHandler func(ctx Context) error
