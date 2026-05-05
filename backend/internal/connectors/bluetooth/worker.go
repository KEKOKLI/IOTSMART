package bluetooth

type Worker struct{}

func (Worker) Name() string {
	return "bluetooth"
}

func (Worker) Status() string {
	return "pending"
}
