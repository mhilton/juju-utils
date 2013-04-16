// On-disk mutex protecting a resource
//
// A lock is represented on disk by a directory of a particular name,
// containing an information file.  Taking a lock is done by renaming a
// temporary directory into place.  We use temporary directories because for
// all filesystems we believe that exactly one attempt to claim the lock will
// succeed and the others will fail.
package fslock

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"time"
)

const nameRegexp = "^[a-z]+[a-z0-9.-]*$"

var (
	ErrLockNotHeld = errors.New("lock not held")

	validName = regexp.MustCompile(nameRegexp)

	lockWaitDelay = 1 * time.Second
)

type Lock struct {
	name   string
	parent string
	nonce  []byte
}

func generateNonce() ([]byte, error) {
	nonce := make([]byte, 20)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// NewLock returns a new lock with the given name within the given lock
// directory, without acquiring it. The lock name must match the regular
// expression `^[a-z]+[a-z0-9.-]*`.
func NewLock(lockDir, name string) (*Lock, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("Invalid lock name %q.  Names must match %q", name, nameRegexp)
	}
	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}
	lock := &Lock{
		name:   name,
		parent: lockDir,
		nonce:  nonce,
	}
	// Ensure the parent exists.
	if err := os.MkdirAll(lock.parent, 0755); err != nil {
		return nil, err
	}
	return lock, nil
}

func (lock *Lock) lockDir() string {
	return path.Join(lock.parent, lock.name)
}

func (lock *Lock) heldFile() string {
	return path.Join(lock.lockDir(), "held")
}

func (lock *Lock) acquire() (bool, error) {
	// If the lockDir exists, then the lock is held by someone else.
	_, err := os.Stat(lock.lockDir())
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	// Create a temporary directory (in the temp dir), and then move it to the right name.
	tempLockName := hex.EncodeToString(lock.nonce)
	tempDirName, err := ioutil.TempDir("", tempLockName)
	if err != nil {
		return false, err // this shouldn't really fail...
	}
	err = os.Rename(tempDirName, lock.lockDir())
	if os.IsExist(err) {
		// Beaten to it, clean up temporary directory.
		os.RemoveAll(tempDirName)
		return false, nil
	} else if err != nil {
		return false, err
	}
	// write nonce
	err = ioutil.WriteFile(lock.heldFile(), lock.nonce, 0755)
	if err != nil {
		return false, err
	}
	// We now have the lock.
	return true, nil
}

// Lock blocks until it is able to acquire the lock.
func (lock *Lock) Lock() error {
	for {
		acquired, err := lock.acquire()
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}
		time.Sleep(lockWaitDelay)
	}
	panic("unreachable")
	return nil // unreachable
}

func (lock *Lock) TryLock(duration time.Duration) (isLocked bool, err error) {
	locked := make(chan bool)
	error := make(chan error)
	timeout := make(chan struct{})
	defer func() {
		close(locked)
		close(error)
		close(timeout)
	}()

	go func() {
		for {
			acquired, err := lock.acquire()
			if err != nil {
				locked <- false
				error <- err
				return
			}
			if acquired {
				locked <- true
				error <- nil
				return
			}
			select {
			case <-timeout:
				locked <- false
				error <- nil
				return
			case <-time.After(lockWaitDelay):
				// Keep trying...
			}
		}
	}()

	select {
	case isLocked = <-locked:
		err = <-error
		return
	case <-time.After(duration):
		timeout <- struct{}{}
	}
	// It is possible that the timeout got signalled just before the goroutine
	// tried again, so check the results rather than automatically failing.
	return <-locked, <-error
}

// IsLockHeld returns true if and only if the lockDir exists, and the
// file 'held' in that directory contains the nonce for this lock.
func (lock *Lock) IsLockHeld() bool {
	heldNonce, err := ioutil.ReadFile(lock.heldFile())
	if err != nil {
		return false
	}
	return bytes.Equal(heldNonce, lock.nonce)
}

func (lock *Lock) Unlock() error {
	if !lock.IsLockHeld() {
		return ErrLockNotHeld
	}
	return os.RemoveAll(lock.lockDir())
}