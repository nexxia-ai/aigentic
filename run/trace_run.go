package run

type Trace interface {
	Interceptor
	RecordError(err error) error
	Close() error
}
