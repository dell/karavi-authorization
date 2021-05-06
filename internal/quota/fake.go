package quota

import "github.com/go-redis/redis"

type FakeRedis struct {
	PingFn    func() (string, error)
	HExistsFn func(key, field string) (bool, error)
	EvalIntFn func(script string, keys []string, args ...interface{}) (int, error)
	HSetNXFn  func(key, field string, value interface{}) (bool, error)
	HGetFn    func(key, field string) (string, error)
	XRangeFn  func(stream, start, stop string) ([]redis.XMessage, error)
}

func (f *FakeRedis) Ping() (string, error) {
	return f.PingFn()
}

func (f *FakeRedis) HExists(key, field string) (bool, error) {
	return f.HExistsFn(key, field)
}

func (f *FakeRedis) HSetNX(key, field string, value interface{}) (bool, error) {
	return f.HSetNXFn(key, field, value)
}

func (f *FakeRedis) HGet(key, field string) (string, error) {
	return f.HGetFn(key, field)
}

func (f *FakeRedis) EvalInt(script string, keys []string, args ...interface{}) (int, error) {
	return f.EvalIntFn(script, keys, args...)
}

func (f *FakeRedis) XRange(stream, start, stop string) ([]redis.XMessage, error) {
	return f.XRangeFn(stream, start, stop)
}
