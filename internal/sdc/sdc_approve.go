// Copyright © 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package sdc

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/go-redis/redis"
	"go.opentelemetry.io/otel/trace"
)

type sdcDB interface {
	Ping() (string, error)
	HGet(key, field string) (string, error)
}

// RedisDB wraps a real redis client and adapts it
// to work with the sdcDB interface.
type RedisDB struct {
	Client *redis.Client
}

var _ sdcDB = (*RedisDB)(nil)

// Ping wraps the original Ping method.
func (r *RedisDB) Ping() (string, error) {
	return r.Client.Ping().Result()
}

// HGet wraps the original HGet method.
func (r *RedisDB) HGet(key, field string) (string, error) {
	return r.Client.HGet(key, field).Result()
}

// RedisSdcApprover is a wrapper around a redis client to approve requests.
type RedisSdcApprover struct {
	rdb sdcDB
}

// Option is to be used for functional options
// with NewRedisSdcApprover
type Option func(v *RedisSdcApprover)

// WithRedis allows for configuring the enforcer with
// a *redis.Client.
func WithRedis(rdb *redis.Client) Option {
	return func(v *RedisSdcApprover) {
		v.rdb = &RedisDB{rdb}
	}
}

// WithDB allows for configuring the enforcer with
// a value that implements the DB interface.
func WithDB(db sdcDB) Option {
	return func(v *RedisSdcApprover) {
		v.rdb = db
	}
}

// NewSdcApprover returns a new RedisSdcApprover.
func NewSdcApprover(_ context.Context, opts ...Option) *RedisSdcApprover {
	v := &RedisSdcApprover{}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Request is a request to redis.
type Request struct {
	Group string `json:"group"`
}

// Ping pings the redis instance.
func (sa *RedisSdcApprover) Ping() error {
	res, err := sa.rdb.Ping()
	if err != nil {
		return err
	}
	log.Println("Redis response:", res)
	return nil
}

// DataKey returns a redis formatted data key based on the Request data.
func (r Request) DataKey() string {
	return fmt.Sprintf("tenant:%s:data", r.Group)
}

// ApproveSdcField returns the redis formatted approved capacity field.
func (r Request) ApproveSdcField() string {
	return "approve_sdc"
}

// CheckSdcApproveFlag checks the approvesdc flag value of the tenant
func (sa *RedisSdcApprover) CheckSdcApproveFlag(ctx context.Context, r Request) (bool, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "checkSdcApproveFlag")
	defer span.End()
	var err error

	flagvalue, err := sa.rdb.HGet(r.DataKey(), r.ApproveSdcField())
	if err != nil {
		return false, err
	}

	flagvaluebool, err := strconv.ParseBool(flagvalue)
	if err != nil {
		return false, err
	}

	if flagvaluebool {
		return true, nil
	}
	return false, nil
}
