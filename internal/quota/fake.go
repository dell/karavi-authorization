// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quota

import "github.com/go-redis/redis"

// FakeRedis is used for mocking out commonly used functions for
// the Redis client.
type FakeRedis struct {
	PingFn    func() (string, error)
	HExistsFn func(key, field string) (bool, error)
	EvalIntFn func(script string, keys []string, args ...interface{}) (int, error)
	HSetNXFn  func(key, field string, value interface{}) (bool, error)
	HGetFn    func(key, field string) (string, error)
	XRangeFn  func(stream, start, stop string) ([]redis.XMessage, error)
}

// Ping delegates to the PingFn function field.
func (f *FakeRedis) Ping() (string, error) {
	return f.PingFn()
}

// HExists delegates to the HExistsFn function field.
func (f *FakeRedis) HExists(key, field string) (bool, error) {
	return f.HExistsFn(key, field)
}

// HSetNX delegates to the HSetNXFn function field.
func (f *FakeRedis) HSetNX(key, field string, value interface{}) (bool, error) {
	return f.HSetNXFn(key, field, value)
}

// HGet delegates to the HGetFn function field.
func (f *FakeRedis) HGet(key, field string) (string, error) {
	return f.HGetFn(key, field)
}

// EvalInt delegates to the EvalIntFn function field.
func (f *FakeRedis) EvalInt(script string, keys []string, args ...interface{}) (int, error) {
	return f.EvalIntFn(script, keys, args...)
}

// XRange delegates to the XRangeFn function field.
func (f *FakeRedis) XRange(stream, start, stop string) ([]redis.XMessage, error) {
	return f.XRangeFn(stream, start, stop)
}
