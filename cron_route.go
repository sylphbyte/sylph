package sylph

//mode_switch:
//normal: true
//skip: true
//delay: true

type ModeSwitch SwitchMode

type ServiceCrontabYaml struct {
	ModeSwitch ModeSwitch  `yaml:"mode_switch" mapstructure:"mode_switch"`
	Jobs       TaskConfigs `yaml:"jobs"`
}

func (s ServiceCrontabYaml) OpenService() bool {
	for _, open := range s.ModeSwitch {
		if open {
			return true
		}
	}

	return false
}

// TaskName 这里是能扩展的 任务可以给rocket 用 也可以给其他任务相关的地方用
type TaskName string

func (t TaskName) Name() string {
	return string(t)
}

type CrontabRouteFunc func(routes ICrontabRoute)

type ICrontabRoute interface {
	Register(name TaskName, task TaskHandler)
	//Receive(name Name) (task TaskHandler, ok bool)
}

type TaskManager struct {
	tasks map[TaskName]TaskHandler
}

//type ICrontabRoutes interface {
//
//	HasNext() bool
//	Next() *CrontabRoute
//}

type TaskConfigs map[CrontabModeName][]TaskConfig
type TaskConfig struct {
	Open bool     `yaml:"open"`
	Name TaskName `yaml:"name"`
	Desc string   `yaml:"desc"`
	Spec string   `yaml:"spec"`
	//Mode CrontabMode `yaml:"mode"`

}

type TaskHandler func(ctx Context) error
