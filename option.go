package sylph

const (
	defaultConfigType = "yaml"
)

var _ IOption = (*Option)(nil)

type IOption interface {
	IsEnvRewrite() bool
	TakeConfigName() string
	TakeConfigType() string
	TakeConfigPaths() []string
	IsPreserveKeyCase() bool
}

type Option struct {
	EnvRewrite      bool
	ConfigName      string
	ConfigType      string // 默认yaml
	ConfigPaths     []string
	PreserveKeyCase bool // 保持 map key 大小写
}

func (o Option) IsEnvRewrite() bool {
	return o.EnvRewrite
}

func (o Option) TakeConfigName() string {
	return o.ConfigName
}

func (o Option) TakeConfigType() string {
	if o.ConfigType == "" {
		return defaultConfigType
	}

	return o.ConfigType
}

func (o Option) TakeConfigPaths() []string {
	return o.ConfigPaths
}

func (o Option) IsPreserveKeyCase() bool {
	return o.PreserveKeyCase
}
