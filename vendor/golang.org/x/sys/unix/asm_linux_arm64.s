// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux
// +build arm64
// +build !gccgo

#include "textflag.h"

// Just jump to package syscall's implementation for all these functions.
// The runtime may know about them.

TEXT	·Syscall(SB),NOSPLIT,$0-56
	B	syscall·Syscall(SB)

TEXT ·Syscall6(SB),NOSPLIT,$0-80
	B	syscall·Syscall6(SB)

TEXT ·RawSyscall(SB),NOSPLIT,$0-56
	B	syscall·RawSyscall(SB)

TEXT ·RawSyscall6(SB),NOSPLIT,$0-80
	B	syscall·RawSyscall6(SB)
