package quota_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"karavi-authorization/internal/quota"

	"os/exec"

	"github.com/go-redis/redis"
)

func TestRedisEnforcer(t *testing.T) {
	rdb := testCreateRedisInstance(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sut := quota.NewRedisEnforcement(ctx, rdb)

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
		got, err := rdb.HGet("mypool:mygroup3:data", "approved_capacity").Result()
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Approved capacity: got %v, want %v", got, want)
		}
	})

	t.Run("reconciles approved but not created volume requests", func(t *testing.T) {
		r := quota.Request{
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

		anc := sut.ApprovedNotCreated(ctx, "mypool:mygroup4:stream")
		if got, want := len(anc), 1; got != want {
			t.Fatalf("ApprovedNotCreated: got len = %v, want %v", got, want)
		}
		if got, want := anc[0].Name, "k8s-0"; got != want {
			t.Errorf("ApprovedNotCreated: got name = %q, want %q", got, want)
		}
	})

	t.Run("duplicate approval requests are ignored", func(t *testing.T) {
		r := quota.Request{
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
		anc := sut.ApprovedNotCreated(ctx, "mypool:mygroup5:stream")
		if got, want := len(anc), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := rdb.HGet("mypool:mygroup5:data", "approved_capacity").Val(), "10"; got != want {
			t.Errorf("approved_cap: got %v, want %v", got, want)
		}
	})
}

type tb interface {
	testing.TB
}

func testCreateRedisInstance(t tb) *redis.Client {
	var retries int
	for {
		cmd := exec.Command("docker", "run",
			"--rm",
			"--name", "test-redis",
			"--net", "host",
			"--detach",
			"redis")
		b, err := cmd.CombinedOutput()
		if err != nil {
			retries++
			if retries >= 3 {
				t.Fatalf("starting redis in docker: %s, %v", string(b), err)
			}
			time.Sleep(time.Second)
			continue
		}
		break
	}

	t.Cleanup(func() {
		err := exec.Command("docker", "stop", "test-redis").Start()
		if err != nil {
			t.Fatal(err)
		}
	})

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Wait for a PING before returning, or fail with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, err := rdb.Ping().Result()
		if err != nil {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			default:
				time.Sleep(time.Nanosecond)
				continue
			}
		}

		break
	}

	return rdb
}

func BenchmarkApproveRequest(b *testing.B) {
	rdb := testCreateRedisInstance(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sut := quota.NewRedisEnforcement(ctx, rdb)

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
