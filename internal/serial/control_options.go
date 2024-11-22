package serial

// serialControlOptions is the options for SerialControl.
type serialControlOptions struct {
	cancelOnNew   cancelOnNewType
	queueCapacity int
}

type cancelOnNewType int

const (
	cancelOnNewNothing cancelOnNewType = iota
	cancelOnNewFormer
	cancelOnNewLater
)

var defaultSerialControlOptions = serialControlOptions{
	cancelOnNew:   cancelOnNewNothing,
	queueCapacity: 0,
}

// ControlOption is the option for SerialControl.
type ControlOption func(*serialControlOptions)

// WithCancelFormer cancels the former task when starting a new one.
func WithCancelFormer() ControlOption {
	return func(o *serialControlOptions) {
		o.cancelOnNew = cancelOnNewFormer
	}
}

// WithCancelLater cancels the later tasks if there is a task running.
func WithCancelLater() ControlOption {
	return func(o *serialControlOptions) {
		o.cancelOnNew = cancelOnNewLater
	}
}

// // WithoutCancel cancels nothing when starting a new one. (default)
// func WithoutCancel() SerialControlOption {
// 	return func(o *serialControlOptions) {
// 		o.cancelOnNew = cancelOnNewNothing
// 	}
// }

// WithOrdered makes the tasks executed in order.
// capcity is the capacity of the task queue, which means
// how many tasks can be accepted to maintain the order at a time.
func WithOrdered(capcity int) ControlOption {
	return func(o *serialControlOptions) {
		o.queueCapacity = capcity
	}
}
