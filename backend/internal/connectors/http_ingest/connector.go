package http_ingest

type Connector struct{}

func (Connector) Name() string {
	return "http_ingest"
}

func (Connector) Status() string {
	return "ready"
}
