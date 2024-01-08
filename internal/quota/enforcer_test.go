// Copyright © 2021-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package quota_test

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/quota"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis"
)

var ErrFake = errors.New("test error")

func TestRedisEnforcement_Ping(t *testing.T) {
	rdb := testCreateRedisInstance(t)
	t.Run("returns the error", func(t *testing.T) {
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithDB(&quota.FakeRedis{
			PingFn: func() (string, error) { return "", ErrFake },
		}))

		gotErr := sut.Ping()

		wantErr := ErrFake
		if gotErr != wantErr {
			t.Errorf("got err %v, want %v", gotErr, wantErr)
		}
	})
	t.Run("nil error on success", func(t *testing.T) {
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

		err := sut.Ping()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestRedisEnforcement_ValidateOwnership(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("returns true if key exists", func(t *testing.T) {
		mr.HSet(req.DataKey(), req.CreatedField(), "1")
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.ValidateOwnership(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := true
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns false if key does not exist", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.ValidateOwnership(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := false
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		sut := quota.NewRedisEnforcement(context.Background(),
			quota.WithDB(&quota.FakeRedis{HExistsFn: func(key, field string) (bool, error) {
				return false, ErrFake
			}}))

		_, got := sut.ValidateOwnership(context.Background(), req)

		want := ErrFake
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestRedisEnforcement_DeleteRequest(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("puts volume into deleting state", func(t *testing.T) {
		mr.HSet(req.DataKey(), req.ApprovedField(), "1")
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.DeleteRequest(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		isDeleting, err := rc.HExists(req.DataKey(), req.DeletingField()).Result()
		if err != nil {
			t.Fatal(err)
		}

		want := true
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if !isDeleting {
			t.Errorf("expected volume to be marked as deleted but it was not")
		}
	})
	t.Run("returns false if key does not exist", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.DeleteRequest(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := false
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(),
			quota.WithDB(&quota.FakeRedis{EvalIntFn: func(script string, keys []string, args ...interface{}) (int, error) {
				return 0, ErrFake
			}}))

		_, got := sut.DeleteRequest(context.Background(), req)

		want := ErrFake
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestRedisEnforcement_PublishCreated(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("puts volume into deleting state", func(t *testing.T) {
		mr.HSet(req.DataKey(), req.ApprovedField(), "1")
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.PublishCreated(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		isCreated, err := rc.HExists(req.DataKey(), req.CreatedField()).Result()
		if err != nil {
			t.Fatal(err)
		}

		want := true
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if !isCreated {
			t.Errorf("expected volume to be marked as created but it was not")
		}
	})
	t.Run("returns false if key does not exist", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.PublishCreated(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := false
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(),
			quota.WithDB(&quota.FakeRedis{EvalIntFn: func(script string, keys []string, args ...interface{}) (int, error) {
				return 0, ErrFake
			}}))

		_, got := sut.PublishCreated(context.Background(), req)

		want := ErrFake
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestRedisEnforcement_PublishDeleted(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("puts volume into deleting state", func(t *testing.T) {
		mr.HSet(req.DataKey(), req.ApprovedField(), "1")
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.PublishDeleted(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		isDeleted, err := rc.HExists(req.DataKey(), req.DeletedField()).Result()
		if err != nil {
			t.Fatal(err)
		}

		want := true
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if !isDeleted {
			t.Errorf("expected volume to be marked as deleted but it was not")
		}
	})
	t.Run("returns false if key does not exist", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))

		got, err := sut.PublishDeleted(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := false
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		mr.FlushAll()
		sut := quota.NewRedisEnforcement(context.Background(),
			quota.WithDB(&quota.FakeRedis{EvalIntFn: func(script string, keys []string, args ...interface{}) (int, error) {
				return 0, ErrFake
			}}))

		_, got := sut.PublishDeleted(context.Background(), req)

		want := ErrFake
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestRedisEnforcement_ApproveRequest(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("early context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sut := quota.NewRedisEnforcement(ctx, quota.WithRedis(rc))
		req := buildRequest()

		cancel()
		got, gotErr := sut.ApproveRequest(ctx, req, 0)

		want := false
		if got != want {
			t.Errorf("got value %v, want %v", got, want)
		}
		wantErr := context.Canceled
		if gotErr != wantErr {
			t.Errorf("got err = %v, want %v", gotErr, wantErr)
		}
	})
	t.Run("early return on HExists failure", func(t *testing.T) {
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithDB(&quota.FakeRedis{
			HExistsFn: func(key, field string) (bool, error) {
				return false, ErrFake
			},
		}))

		got, gotErr := sut.ApproveRequest(context.Background(), req, 0)

		want := false
		if got != want {
			t.Errorf("got value %v, want %v", got, want)
		}
		wantErr := ErrFake
		if gotErr != wantErr {
			t.Errorf("got err = %v, want %v", gotErr, wantErr)
		}
	})
	t.Run("returns error on invalid req capacity", func(t *testing.T) {
		sut := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rc))
		req.Capacity = "NaN"

		_, got := sut.ApproveRequest(context.Background(), req, 0)

		// Want a strconv.NumError
		want := &strconv.NumError{}
		if !errors.As(got, &want) {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
}

func buildRequest() quota.Request {
	return quota.Request{
		SystemType:    "powerflex",
		SystemID:      "123",
		StoragePoolID: "mypool",
		Group:         "mytenant",
		VolumeName:    "k8s-456",
		Capacity:      "8300000",
	}
}

func TestRequest(t *testing.T) {
	t.Run("keys", func(t *testing.T) {
		type keyFunc func() string
		r := buildRequest()

		tests := []struct {
			name string
			fn   keyFunc
			want string
		}{
			{"DataKey", r.DataKey, "quota:powerflex:123:mypool:mytenant:data"},
			{"StreamKey", r.StreamKey, "quota:powerflex:123:mypool:mytenant:stream"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				got := tt.fn()
				if got != tt.want {
					t.Errorf("%s(): got %q, want %q", tt.name, got, tt.want)
				}
			})
		}
	})
	t.Run("fields", func(t *testing.T) {
		type fieldFunc func() string
		r := buildRequest()

		tests := []struct {
			name string
			fn   fieldFunc
			want string
		}{
			{"ApprovedField", r.ApprovedField, "vol:k8s-456:approved"},
			{"CapacityField", r.CapacityField, "vol:k8s-456:capacity"},
			{"CreatedField", r.CreatedField, "vol:k8s-456:created"},
			{"DeletingField", r.DeletingField, "vol:k8s-456:deleting"},
			{"DeletedField", r.DeletedField, "vol:k8s-456:deleted"},
			{"ApprovedCapacityField", r.ApprovedCapacityField, "approved_capacity"},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				got := tt.fn()
				if got != tt.want {
					t.Errorf("%s(): got %q, want %q", tt.name, got, tt.want)
				}
			})
		}
	})
}

func TestRedisEnforcement(t *testing.T) {
	rdb := testCreateRedisInstance(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sut := quota.NewRedisEnforcement(ctx, quota.WithRedis(rdb))

	const tenantQuota = 100

	t.Run("NewRedisEnforcer", func(t *testing.T) {
		if sut == nil {
			t.Fatal("expected non-nil return value")
		}
	})

	t.Run("approves volume request within quota", func(t *testing.T) {
		r := quota.Request{
			StoragePoolID: "mypool",
			Group:         "mygroup1",
			VolumeName:    "k8s-0",
			Capacity:      "10",
		}

		want := true
		got, err := sut.ApproveRequest(ctx, r, 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ApproveRequest: got %v, want %v", got, want)
		}

		msgs, err := rdb.XRange(r.StreamKey(), "-", "+").Result()
		if err != nil {
			t.Fatal(err)
		}
		var approved []redis.XMessage
		for _, msg := range msgs {
			if msg.Values["status"] == "approved" {
				approved = append(approved, msg)
			}
		}
		if got, want := len(approved), 1; got != want {
			t.Errorf("len(approvals): got %d, want %d", got, want)
		}
	})

	t.Run("approves volume request with infinte quota", func(t *testing.T) {
		r := quota.Request{
			StoragePoolID: "mypool",
			Group:         "mygroup1",
			VolumeName:    "k8s-0",
			Capacity:      "10",
		}

		want := true
		got, err := sut.ApproveRequest(ctx, r, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ApproveRequest: got %v, want %v", got, want)
		}

		msgs, err := rdb.XRange(r.StreamKey(), "-", "+").Result()
		if err != nil {
			t.Fatal(err)
		}
		var approved []redis.XMessage
		for _, msg := range msgs {
			if msg.Values["status"] == "approved" {
				approved = append(approved, msg)
			}
		}
		if got, want := len(approved), 1; got != want {
			t.Errorf("len(approvals): got %d, want %d", got, want)
		}
	})

	t.Run("denies volume request exceeding quota", func(t *testing.T) {
		// Approve requests 0-9 to fill up the quota
		for i := 0; i < 10; i++ {
			r := quota.Request{
				StoragePoolID: "mypool",
				Group:         "mygroup2",
				VolumeName:    fmt.Sprintf("k8s-%d", i),
				Capacity:      "10",
			}
			sut.ApproveRequest(ctx, r, tenantQuota)
		}

		r := quota.Request{
			StoragePoolID: "mypool",
			Group:         "mygroup2",
			VolumeName:    "k8s-10",
			Capacity:      "10",
		}

		want := false
		got, err := sut.ApproveRequest(ctx, r, tenantQuota)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ApproveRequest: got %v, want %v", got, want)
		}
	})

	t.Run("parallel test", func(t *testing.T) {
		var (
			wg              sync.WaitGroup
			allows, denials int64
		)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			i := i

			go func() {
				defer wg.Done()

				r := quota.Request{
					SystemType:    "powerflex",
					SystemID:      "123",
					StoragePoolID: "mypool",
					Group:         "mygroup3",
					VolumeName:    fmt.Sprintf("k8s-%d", i),
					Capacity:      "10",
				}

				var (
					ok  bool
					err error
				)
				for {
					ok, err = sut.ApproveRequest(ctx, r, tenantQuota)
					if err != nil {
						return
					}
					break
				}

				if ok {
					atomic.AddInt64(&allows, 1)
				} else {
					atomic.AddInt64(&denials, 1)
				}
			}()
		}
		wg.Wait()

		gotAllows, gotDenials := atomic.LoadInt64(&allows), atomic.LoadInt64(&denials)
		wantAllows, wantDenials := int64(10), int64(10)
		if gotAllows != wantAllows && gotDenials != wantDenials {
			t.Errorf("got %v/%v, want %v/%v", gotAllows, gotDenials, wantAllows, wantDenials)
		}

		want := "100"
		got, err := rdb.HGet("quota:powerflex:123:mypool:mygroup3:data", "approved_capacity").Result()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Approved capacity: got %v, want %v", got, want)
		}
	})

	t.Run("reconciles approved but not created volume requests", func(t *testing.T) {
		r := quota.Request{
			SystemType:    "powerflex",
			SystemID:      "123",
			StoragePoolID: "mypool",
			Group:         "mygroup4",
			VolumeName:    "k8s-0",
			Capacity:      "10",
		}
		ok, err := sut.ApproveRequest(ctx, r, tenantQuota)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("expected request to be approved, but was not")
		}

		anc := sut.ApprovedNotCreated(ctx, "quota:powerflex:123:mypool:mygroup4:stream")
		if got, want := len(anc), 1; got != want {
			t.Fatalf("ApprovedNotCreated: got len = %v, want %v", got, want)
		}
		if got, want := anc[0].Name, "k8s-0"; got != want {
			t.Errorf("ApprovedNotCreated: got name = %q, want %q", got, want)
		}
	})

	t.Run("duplicate approval requests are ignored", func(t *testing.T) {
		r := quota.Request{
			SystemType:    "powerflex",
			SystemID:      "123",
			StoragePoolID: "mypool",
			Group:         "mygroup5",
			VolumeName:    "k8s-0",
			Capacity:      "10",
		}
		ok, err := sut.ApproveRequest(ctx, r, tenantQuota)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("got %v, want %v", ok, true)
		}
		ok, err = sut.ApproveRequest(ctx, r, tenantQuota)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("got %v, want %v", ok, true)
		}
		anc := sut.ApprovedNotCreated(ctx, "quota:powerflex:123:mypool:mygroup5:stream")
		if got, want := len(anc), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := rdb.HGet("quota:powerflex:123:mypool:mygroup5:data", "approved_capacity").Val(), "10"; got != want {
			t.Errorf("approved_cap: got %v, want %v", got, want)
		}
	})
}

type tb interface {
	testing.TB
}

func testCreateRedisInstance(t tb) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mr.Close()
	})

	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

func BenchmarkApproveRequest(b *testing.B) {
	rdb := testCreateRedisInstance(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sut := quota.NewRedisEnforcement(ctx, quota.WithRedis(rdb))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok, err := sut.ApproveRequest(ctx, quota.Request{
			StoragePoolID: "mypool",
			Group:         "mygroup",
			VolumeName:    fmt.Sprintf("k8s-%d", i),
			Capacity:      "1",
		}, 1_000_000)
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			b.Fatal("failed to approve")
		}
	}
}
