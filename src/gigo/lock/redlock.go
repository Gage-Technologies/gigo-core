package lock

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/sourcegraph/conc"
	"sync/atomic"
	"time"
)

type AutoRedLock struct {
	lock   *redsync.Mutex
	locked *atomic.Bool
	ctx    context.Context
	cancel context.CancelFunc
}

type RedLockManager struct {
	sync   *redsync.Redsync
	wg     *conc.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// CreateAutoRedLock
// Creates a new AutoRedLock instance to manage the expiration of a redlock mutex
// Args:
//
//	sync   	- *redsync.Redsync, redsync instance that will be used to coordinate the lock operation
//	name 	- string, name of the lock that will be used
//
// Returns:
//
//	out	    - *AutoRedLock, new AutoRedLock instance
func CreateAutoRedLock(ctx context.Context, sync *redsync.Redsync, waitGroup *conc.WaitGroup, name string) *AutoRedLock {
	// create a new context for managing the auto extend
	ctx, cancel := context.WithCancel(ctx)
	// create a new auto lock instance
	al := &AutoRedLock{
		lock:   sync.NewMutex(name),
		locked: &atomic.Bool{},
		ctx:    ctx,
		cancel: cancel,
	}
	// launch auto extend routine
	waitGroup.Go(al.autoLockExtendRoutine)

	return al
}

// autoLockExtendRoutine
// Automatically manages the timeout expiration for the duration that the lock is enabled and the instance is alive
func (l *AutoRedLock) autoLockExtendRoutine() {
	// loop indefinitely extended lock expiration when locked
	for {
		// check for context closure
		select {
		// handle closed context by exiting
		case <-l.ctx.Done():
			return
		// permit normal execution
		default:
		}

		// extend lock expiration if it is locked
		if l.locked.Load() {
			// attempt to extend lock
			_, _ = l.lock.Extend()
		}

		// sleep for 4 seconds (half the lock expiration time)
		time.Sleep(time.Second * 4)
	}
}

// Kill
// Closes the automatic expiration extension routine to allow the lock to expire
func (l *AutoRedLock) Kill() { l.cancel() }

// Lock
// Wraps the internal Lock function and includes the lock status tracking necessary for expiration extensions
func (l *AutoRedLock) Lock() error { return l.LockContext(nil) }

// Unlock
// Wraps the internal Unlock function and includes the lock status tracking necessary for expiration extensions
// Returns
//
//	ok 		- bool, whether the lock release was successful
func (l *AutoRedLock) Unlock() (bool, error) { return l.UnlockContext(nil) }

// LockContext
// Wraps the internal LockContext function and includes the lock status tracking necessary for expiration extensions
// Args
//
//	ctx		- context.Context, context that will be used for acquiring the redlock
func (l *AutoRedLock) LockContext(ctx context.Context) error {
	// execute lock operation
	err := l.lock.LockContext(ctx)
	// conditionally mark locked as true within the struct
	if err == nil {
		l.locked.Store(true)
	}
	return err
}

// UnlockContext
// Wraps the internal UnlockContext function and includes the lock status tracking necessary for expiration extensions
// Args
//
//	ctx		- context.Context, context that will be used for releasing the redlock
//
// Returns
//
//	ok 		- bool, whether the lock release was successful
func (l *AutoRedLock) UnlockContext(ctx context.Context) (bool, error) {
	// execute unlock operation
	ok, err := l.lock.UnlockContext(ctx)
	// conditionally mark locked as false within the struct
	if ok {
		l.locked.Store(false)
	}
	return ok, err
}

// CreateRedLockManager
// Creates a new CreateRedLockManager that new AutoRedLock instances can be generated from
// Args:
//
//	rdb   	- redis.UniversalClient, redis client used for managing the redlock system
//
// Returns:
//
//	out	    - *RedLockManager, new RedLockManager instance
func CreateRedLockManager(rdb redis.UniversalClient) *RedLockManager {
	// create a new pool from the passed redis client
	pool := goredis.NewPool(rdb)

	// create a new redsync instance from the pool
	rs := redsync.New(pool)

	// create a new context for the manager
	ctx, cancel := context.WithCancel(context.Background())

	return &RedLockManager{
		sync:   rs,
		wg:     conc.NewWaitGroup(),
		ctx:    ctx,
		cancel: cancel,
	}
}

// GetLock
// Gets an AutoRedLock that can manage distributed locking across integrated systems
// Args:
//
//	name 	- string, name of the lock that will be used
//
// Returns:
//
//	out	    - *AutoRedLock, new AutoRedLock instance
func (rl *RedLockManager) GetLock(name string) *AutoRedLock {
	// create a new lock
	return CreateAutoRedLock(rl.ctx, rl.sync, rl.wg, name)
}

// Close
// Closes the RedLockManager and any child locks
func (rl *RedLockManager) Close() {
	rl.cancel()
	rl.wg.Wait()
}
