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
	"sync"
	"syscall"
)

// An fcntl lock belongs to the process.  A second open of the same file
// in this process gets the lock too, and its close drops the lock for
// other processes.  lockedFiles refuses a second open, except a
// read-only open next to other read-only opens.  The value is the
// number of read-only opens, or -1 for a writable open.  The key is the
// file, not the path, so a symlink does not get past it.
var (
	lockedMu    sync.Mutex
	lockedFiles = make(map[fileID]int)
)

type fileID struct {
	dev, ino uint64
}

func fileIDOf(fi os.FileInfo) fileID {
	st := fi.Sys().(*syscall.Stat_t)
	return fileID{dev: st.Dev, ino: st.Ino}
}

type unixFileLock struct {
	f  *os.File
	id fileID
}

func (fl *unixFileLock) release() error {
	err := setFileLock(fl.f, false, false)
	// Close before the file leaves the registry.  Any close of the file
	// drops every lock that this process holds on it.
	if cerr := fl.f.Close(); err == nil {
		err = cerr
	}
	lockedMu.Lock()
	if n := lockedFiles[fl.id]; n > 1 {
		lockedFiles[fl.id] = n - 1
	} else {
		delete(lockedFiles, fl.id)
	}
	lockedMu.Unlock()
	return err
}

func newFileLock(path string, readOnly bool) (fl fileLock, err error) {
	lockedMu.Lock()
	defer lockedMu.Unlock()
	// Check before the open.  The close of a refused descriptor would
	// drop the lock of the first open.
	if fi, err := os.Stat(path); err == nil {
		n, ok := lockedFiles[fileIDOf(fi)]
		if ok && (!readOnly || n < 0) {
			return nil, errors.New("leveldb/storage: file already locked by this process")
		}
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
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return
	}
	err = setFileLock(f, readOnly, true)
	if err != nil {
		f.Close()
		return
	}
	id := fileIDOf(fi)
	if readOnly {
		lockedFiles[id]++
	} else {
		lockedFiles[id] = -1
	}
	fl = &unixFileLock{f: f, id: id}
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
