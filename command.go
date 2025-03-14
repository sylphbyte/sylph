package sylph

type Commands []Command

type Command struct {
	Name  string
	Value string
	Usage string
}
