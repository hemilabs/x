// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:build solaris

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// An fcntl lock belongs to the process.  A second open of the same path
// in this process gets the lock too, and its close drops the lock for
// other processes.  lockedPaths refuses a second open, except a
// read-only open next to other read-only opens.  The value is the
// number of read-only opens, or -1 for a writable open.
var (
	lockedMu    sync.Mutex
	lockedPaths = make(map[string]int)
)

type unixFileLock struct {
	f    *os.File
	path string
}

func (fl *unixFileLock) release() error {
	err := setFileLock(fl.f, false, false)
	// Close before the path leaves the registry.  Any close of the file
	// drops every lock that this process holds on it.
	if cerr := fl.f.Close(); err == nil {
		err = cerr
	}
	lockedMu.Lock()
	if n := lockedPaths[fl.path]; n > 1 {
		lockedPaths[fl.path] = n - 1
	} else {
		delete(lockedPaths, fl.path)
	}
	lockedMu.Unlock()
	return err
}

func newFileLock(path string, readOnly bool) (fl fileLock, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	lockedMu.Lock()
	defer lockedMu.Unlock()
	if n, ok := lockedPaths[abs]; ok && (!readOnly || n < 0) {
		return nil, errors.New("leveldb/storage: file already locked by this process")
	}
	var flag int
	if readOnly {
		flag = os.O_RDONLY
	} else {
		flag = os.O_RDWR
	}
	f, err := os.OpenFile(path, flag, 0)
	if os.IsNotExist(err) {
		f, err = os.OpenFile(path, flag|os.O_CREATE, 0644)
	}
	if err != nil {
		return
	}
	err = setFileLock(f, readOnly, true)
	if err != nil {
		f.Close()
		return
	}
	if readOnly {
		lockedPaths[abs]++
	} else {
		lockedPaths[abs] = -1
	}
	fl = &unixFileLock{f: f, path: abs}
	return
}

func setFileLock(f *os.File, readOnly, lock bool) error {
	flock := syscall.Flock_t{
		Type:   syscall.F_UNLCK,
		Start:  0,
		Len:    0,
		Whence: 1,
	}
	if lock {
		if readOnly {
			flock.Type = syscall.F_RDLCK
		} else {
			flock.Type = syscall.F_WRLCK
		}
	}
	return syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &flock)
}

func rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func syncDir(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}
