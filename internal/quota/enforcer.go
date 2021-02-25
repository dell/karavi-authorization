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

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-redis/redis"
	"go.opentelemetry.io/otel/trace"
)

// RedisEnforcement is a wrapper around a redis client to approve requests
type RedisEnforcement struct {
	rdb *redis.Client
}

// VolumeData is data about a backend storage volume
type VolumeData struct {
	Name  string
	State string // TODO(ian): Create enum
	Cap   string
}

// NewRedisEnforcement returns a new RedisEnforcement
func NewRedisEnforcement(ctx context.Context, rdb *redis.Client) *RedisEnforcement {
	v := &RedisEnforcement{
		rdb: rdb,
	}
	return v
}

// Request is a request to redis
type Request struct {
	StoragePoolID string `json:"storage_pool_id"`
	Group         string `json:"group"`
	VolumeName    string `json:"volume_name"`
	Capacity      string `json:"capacity"`
}

// Handler is the RedisEnforcement http handler
func (e *RedisEnforcement) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{ "hello": "karavi!!!" }`))
	})
}

// Ping pings the redis instance
func (e *RedisEnforcement) Ping() error {
	res, err := e.rdb.Ping().Result()
	if err != nil {
		return err
	}
	log.Println("Redis response:", res)
	return nil
}

// DataKey returns a redis formatted data key with a Request storage pool ID and group
func (r Request) DataKey() string {
	return fmt.Sprintf("%s:%s:data", r.StoragePoolID, r.Group)
}

// StreamKey returns a redis formatted stream key with a Request storage pool ID and group
func (r Request) StreamKey() string {
	return fmt.Sprintf("%s:%s:stream", r.StoragePoolID, r.Group)
}

// ApprovedField returns a redis formatted approved string with the Request volume
func (r Request) ApprovedField() string {
	return fmt.Sprintf("vol:%s:approved", r.VolumeName)
}

// CapacityField returns a redis formatted capacity string with the Request volume
func (r Request) CapacityField() string {
	return fmt.Sprintf("vol:%s:capacity", r.VolumeName)
}

// CreatedField returns a redis formatted created string with the Request volume
func (r Request) CreatedField() string {
	return fmt.Sprintf("vol:%s:created", r.VolumeName)
}

// DeletingField returns a redis formatted deleting string with the Request volume
func (r Request) DeletingField() string {
	return fmt.Sprintf("vol:%s:deleting", r.VolumeName)
}

// DeletedField returns a redis formatted deleted string with the Request volume
func (r Request) DeletedField() string {
	return fmt.Sprintf("vol:%s:deleted", r.VolumeName)
}

// UnmappingField returns a redis formatted unmapping string with the Request volume
func (r Request) UnmappingField() string {
	return fmt.Sprintf("vol:%s:unmapping", r.VolumeName)
}

// UnmappedField returns a redis formatted unmapped string with the Request volume
func (r Request) UnmappedField() string {
	return fmt.Sprintf("vol:%s:unmapped", r.VolumeName)
}

// ApprovedCapacityField returns the redis formatted approved capacity field
func (r Request) ApprovedCapacityField() string {
	return "approved_capacity"
}

// ApproveRequest approves or disapproves a redist Request
func (e *RedisEnforcement) ApproveRequest(ctx context.Context, r Request, quota int64) (bool, error) {
	ctx, span := trace.SpanFromContext(ctx).Tracer().Start(ctx, "ApproveRequest")
	defer span.End()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		ok, err := e.rdb.HExists(r.DataKey(), r.ApprovedField()).Result()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}

		reqCapInt, err := strconv.ParseInt(r.Capacity, 10, 64)
		if err != nil {
			return false, fmt.Errorf("parse capacity: %w", err)
		}

		_, err = e.rdb.HSetNX(r.DataKey(), r.ApprovedCapacityField(), "0").Result()
		if err != nil {
			continue
		}
		approvedCap, err := e.rdb.HGet(r.DataKey(), r.ApprovedCapacityField()).Result()
		if err != nil {
			return false, err
		}
		approvedCapInt, err := strconv.ParseInt(approvedCap, 10, 64)
		if err != nil {
			return false, fmt.Errorf("parse capacity: %w", err)
		}
		if approvedCapInt+reqCapInt > quota {
			return false, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		// TODO(ian): Pass in the quota and perhaps we can
		// check if quota is exceeded right there, in order
		// to reduce locking churn
		changed, err := e.rdb.Eval(`
local key = KEYS[1]
local approvedCapField = ARGV[1]
local fenceCap = ARGV[2]
local approvedField = ARGV[3]
local capField = ARGV[4]
local delta = ARGV[5]
local streamKey = ARGV[6]

if redis.call('HGET', key, approvedCapField) == fenceCap then
  redis.call('HSET', key, approvedField, 1)
  redis.call('HSET', key, capField, delta)
  redis.call('HINCRBY', key, approvedCapField, delta)
  redis.call('XADD', streamKey, '*',
	ARGV[7], ARGV[8],
	ARGV[9], ARGV[10],
	ARGV[11], ARGV[12])
  return 1
end
return 0
`, []string{r.DataKey()},
			r.ApprovedCapacityField(),
			approvedCap, // this is the fencing token
			r.ApprovedField(),
			r.CapacityField(),
			r.Capacity,
			r.StreamKey(),
			"name", r.VolumeName,
			"cap", r.Capacity,
			"status", "approved").Int()
		if err != nil {
			return false, err
		}
		if changed == 0 {
			continue
		}
		break
	}
	return true, nil
}

// DeleteRequest marks the volume as being in the process of deletion only.
// It's OK for this to be called multiple times, as the only negative impact
// would be multiple stream entries.
func (e *RedisEnforcement) DeleteRequest(ctx context.Context, r Request) (bool, error) {
	changed, err := e.rdb.Eval(`
local key = KEYS[1]
local approvedField = ARGV[1]
local deletingField = ARGV[2]
local streamKey = ARGV[3]

if redis.call('HEXISTS', key, approvedField) == 1 then
  redis.call('HSET', key, deletingField, 1)
  redis.call('XADD', streamKey, '*',
	ARGV[4], ARGV[5],
    ARGV[6], ARGV[7])
  return 1
end
return 0
`, []string{r.DataKey()},
		r.ApprovedField(),
		r.DeletingField(),
		r.StreamKey(),
		"name", r.VolumeName,
		"status", "deleting").Int()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// UnmapRequest approves or disapproves an unmap redist Request
func (e *RedisEnforcement) UnmapRequest(ctx context.Context, r Request) (bool, error) {
	changed, err := e.rdb.Eval(`
local key = KEYS[1]
local approvedField = ARGV[1]
local unmappingField = ARGV[2]
local streamKey = ARGV[3]

if redis.call('HEXISTS', key, approvedField) == 1 then
  redis.call('HSET', key, unmappingField, 1)
  redis.call('XADD', streamKey, '*',
	ARGV[4], ARGV[5],
    ARGV[6], ARGV[7])
  return 1
end
return 0
`, []string{r.DataKey()},
		r.ApprovedField(),
		r.UnmappingField(),
		r.StreamKey(),
		"name", r.VolumeName,
		"status", "unmapping").Int()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// PublishCreated ...
func (e *RedisEnforcement) PublishCreated(ctx context.Context, r Request) (bool, error) {
	changed, err := e.rdb.Eval(`
local key = KEYS[1]
local approvedField = ARGV[1]
local createdField = ARGV[2]
local streamKey = ARGV[3]

if redis.call('HEXISTS', key, approvedField) == 1 then
  redis.call('HSET', key, createdField, 1)
  redis.call('XADD', streamKey, '*',
	ARGV[4], ARGV[5],
	ARGV[6], ARGV[7],
	ARGV[8], ARGV[9])
  return 1
end
return 0
`, []string{r.DataKey()},
		r.ApprovedField(),
		r.CreatedField(),
		r.StreamKey(),
		"name", r.VolumeName,
		"cap", r.Capacity,
		"status", "created").Int()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// PublishDeleted ...
func (e *RedisEnforcement) PublishDeleted(ctx context.Context, r Request) (bool, error) {
	changed, err := e.rdb.Eval(`
local key = KEYS[1]
local approvedField = ARGV[1]
local deletedField = ARGV[2]
local approvedCapField = ARGV[3]
local capField = ARGV[4]
local streamKey = ARGV[5]

if redis.call('HEXISTS', key, approvedField) == 1 then
  redis.call('HSET', key, deletedField, 1)
  redis.call('HSETNX', key, capField, 0)
  local cap = redis.call('HGET', key, capField)
  if tonumber(cap) > 0 then
    redis.call('HINCRBY', key, approvedCapField, tonumber(cap)*-1)
  end
  redis.call('XADD', streamKey, '*',
	ARGV[6], ARGV[7],
	ARGV[8], ARGV[9],
	ARGV[10], ARGV[11])
  return 1
end
return 0
`, []string{r.DataKey()},
		r.ApprovedField(),
		r.DeletedField(),
		r.ApprovedCapacityField(),
		r.CapacityField(),
		r.StreamKey(),
		"name", r.VolumeName,
		"cap", r.Capacity,
		"status", "deleted").Int()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// ApprovedNotCreated ...
// TODO(ian): this should be a continous stream to build an eventually
// consistent view.
func (e *RedisEnforcement) ApprovedNotCreated(ctx context.Context, streamKey string) []VolumeData {
	msgs, err := e.rdb.XRange(streamKey, "-", "+").Result()
	if err != nil {
		panic(err)
	}
	approved := make(map[interface{}]struct{})
	created := make(map[interface{}]struct{})
	for _, msg := range msgs {
		switch msg.Values["status"] {
		case "approved":
			approved[msg.Values["name"]] = struct{}{}
		case "created":
			created[msg.Values["name"]] = struct{}{}
		}
	}
	diff := make([]VolumeData, 0)
	for k := range approved {
		if _, ok := created[k]; !ok {
			diff = append(diff, VolumeData{
				Name: k.(string),
			})
		}
	}

	return diff
}

func asString(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		panic("not a string")
	}
	return s
}
