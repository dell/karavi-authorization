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

package sdc_test

import (
	"context"
	"errors"
	"karavi-authorization/internal/sdc"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis"
)

var ErrFake = errors.New("test error")

func TestSdcApprover_Ping(t *testing.T) {
	rdb := testCreateRedisInstance(t)
	t.Run("returns the error", func(t *testing.T) {
		sut := sdc.NewSdcApprover(context.Background(), sdc.WithDB(&sdc.FakeRedis{
			PingFn: func() (string, error) { return "", ErrFake },
		}))

		gotErr := sut.Ping()

		wantErr := ErrFake
		if gotErr != wantErr {
			t.Errorf("got err %v, want %v", gotErr, wantErr)
		}
	})
	t.Run("nil error on success", func(t *testing.T) {
		sut := sdc.NewSdcApprover(context.Background(), sdc.WithRedis(rdb))

		err := sut.Ping()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSdcApprover_checkSdcApproveFlag(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	req := buildRequest()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Run("returns true if key value is true", func(t *testing.T) {
		mr.HSet(req.DataKey(), req.ApproveSdcField(), "true")
		sut := sdc.NewSdcApprover(context.Background(), sdc.WithRedis(rc))

		got, err := sut.CheckSdcApproveFlag(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := true
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns false if key vale is false", func(t *testing.T) {
		mr.FlushAll()
		mr.HSet(req.DataKey(), req.ApproveSdcField(), "false")
		sut := sdc.NewSdcApprover(context.Background(), sdc.WithRedis(rc))

		got, err := sut.CheckSdcApproveFlag(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		want := false
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		sut := sdc.NewSdcApprover(context.Background(),
			sdc.WithDB(&sdc.FakeRedis{HGetFn: func(key, field string) (string, error) {
				return "false", ErrFake
			},
			}))

		_, got := sut.CheckSdcApproveFlag(context.Background(), req)

		want := ErrFake
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func buildRequest() sdc.Request {
	return sdc.Request{
		Group: "mytenant",
	}
}

func TestRequest(t *testing.T) {
	t.Run("keys", func(t *testing.T) {
		type keyFunc func() string
		r := buildRequest()

		var tests = []struct {
			name string
			fn   keyFunc
			want string
		}{
			{"DataKey", r.DataKey, "tenant:mytenant:data"},
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

		var tests = []struct {
			name string
			fn   fieldFunc
			want string
		}{
			{"ApproveSdcField", r.ApproveSdcField, "approve_sdc"},
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
