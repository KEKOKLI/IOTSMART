package lora

type Connector struct{}

func (Connector) Name() string {
	return "lora"
}

func (Connector) Status() string {
	return "pending"
}
