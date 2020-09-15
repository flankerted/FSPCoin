// Copyright 2018 The go-contatract Authors
// This file is part of go-contatract.
//
// go-contatract is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-contatract is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-contatract. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"runtime"
	"strings"

	"github.com/contatract/go-contatract/cmd/utils"
	"github.com/contatract/go-contatract/params"

	cli "gopkg.in/urfave/cli.v1"
)

var elephantCommand = cli.Command{
	Action:    utils.MigrateFlags(foo1),
	Name:      "elephant",
	Usage:     "elephant want to do something interesting...",
	ArgsUsage: " ",
	Category:  "MISCELLANEOUS COMMANDS",
}

// reportBug reports a bug by opening a new URL to the go-contatract GH issue
// tracker and setting default values as the issue body.
func foo1(ctx *cli.Context) error {
	// execute template and write contents to buff
	var buff bytes.Buffer

	fmt.Fprintln(&buff, header)
	fmt.Fprintln(&buff, "Version:", params.Version)
	fmt.Fprintln(&buff, "Go Version:", runtime.Version())
	fmt.Fprintln(&buff, "OS:", runtime.GOOS)
	printOSDetails(&buff)
	return nil
}

// copied from the Go source. Copyright 2018 The Go Authors
func elephantOSDetails(w io.Writer) {
	switch runtime.GOOS {
	case "darwin":
		printCmdOut(w, "uname -v: ", "uname", "-v")
		printCmdOut(w, "", "sw_vers")
	case "linux":
		printCmdOut(w, "uname -sr: ", "uname", "-sr")
		printCmdOut(w, "", "lsb_release", "-a")
	case "openbsd", "netbsd", "freebsd", "dragonfly":
		printCmdOut(w, "uname -v: ", "uname", "-v")
	case "solaris":
		out, err := ioutil.ReadFile("/etc/release")
		if err == nil {
			fmt.Fprintf(w, "/etc/release: %s\n", out)
		} else {
			fmt.Printf("failed to read /etc/release: %v\n", err)
		}
	}
}

// printCmdOut prints the output of running the given command.
// It ignores failures; 'go bug' is best effort.
//
// copied from the Go source. Copyright 2018 The Go Authors
func elephantPrintCmdOut(w io.Writer, prefix, path string, args ...string) {
	cmd := exec.Command(path, args...)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s %s: %v\n", path, strings.Join(args, " "), err)
		return
	}
	fmt.Fprintf(w, "%s%s\n", prefix, bytes.TrimSpace(out))
}

const elephantHeader = `Please answer these questions before submitting your issue. Thanks!

#### What did you do?
 
#### What did you expect to see?
 
#### What did you see instead?
 
#### System details
`
