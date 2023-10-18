package lock

import (
	"github.com/go-redis/redis/v8"
	"testing"
)

func TestCreateRedLock(t *testing.T) {
	// create redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	rl := CreateRedLockManager(rdb)
	if rl == nil {
		t.Error("\nCreate redlock failed\n    Error: redlock returned as nil")
		return
	}

	t.Log("\nCreate redlock succeeded")
}

func TestRedLock_GetLock(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	rl := CreateRedLockManager(rdb)
	if rl == nil {
		t.Error("\nCreate redlock failed\n    Error: redlock returned as nil")
		return
	}

	lock := rl.GetLock("test-get-lock")
	if lock == nil {
		t.Error("\nGet lock failed\n    Error: lock returned as nil")
		return
	}

	t.Log("\nGet lock succeeded")
}

func TestAutoLock_Lock_Unlock(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	rl := CreateRedLockManager(rdb)
	if rl == nil {
		t.Error("\nCreate redlock failed\n    Error: redlock returned as nil")
		return
	}

	lock := rl.GetLock("test-auto-lock-lock-unlock")
	if lock == nil {
		t.Error("\nGet lock failed\n    Error: lock returned as nil")
		return
	}

	err := lock.Lock()
	if err != nil {
		t.Error("\nAuto lock lock failed\n    Error: ", err)
		return
	}

	if !lock.locked.Load() {
		t.Error("\nAuto lock lock failed\n    Error: internal locked var not set")
		return
	}

	ok, err := lock.Unlock()
	if err != nil {
		t.Error("\nAuto lock unlock failed\n    Error: ", err)
		return
	}

	if !ok {
		t.Error("\nAuto lock unlock failed\n    Error: lock not released")
		return
	}

	if lock.locked.Load() {
		t.Error("\nAuto lock unlock failed\n    Error: internal locked var not unset")
		return
	}

	t.Log("\nAuto lock lock/unlock succeeded")
}
