// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build gccgo,linux,amd64

package unix

import "syscall"

//extern gettimeofday
func realGettimeofday(*Timeval, *byte) int32

func gettimeofday(tv *Timeval) (err syscall.Errno) {
	r := realGettimeofday(tv, nil)
	if r < 0 {
		return syscall.GetErrno()
	}
	return 0
}
