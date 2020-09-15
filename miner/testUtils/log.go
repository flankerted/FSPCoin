// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

package testUtils

import (
	"os"
	"path/filepath"

	"github.com/contatract/go-contatract/log"
)

var LoggerTest log.Logger

func CreateLogger(fileName string) (logger log.Logger) {
	logger = log.New()
	path := filepath.Join(defaultDataDir(""), fileName)
	dir := filepath.Join(defaultDataDir(""))
	if err := os.MkdirAll(dir, 0700); err != nil {
		LogError("CreateLogger failed", "err", err)
		return nil
	}
	if ok, _ := pathExists(path); ok {
		bak := path + ".bak"
		if okBak, _ := pathExists(bak); okBak {
			os.Remove(bak)
		}
		os.Rename(path, bak)
	}
	fileHandle := log.LvlFilterHandler(log.LvlInfo, log.GetDefaultFileHandle(path))
	logger.SetHandler(fileHandle)
	//logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(false))))

	return
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func LogInfo(msg string, ctx ...interface{}) {
	LoggerTest.Info(msg, ctx...)
}

func LogWarn(msg string, ctx ...interface{}) {
	LoggerTest.Warn(msg, ctx...)
}

func LogError(msg string, ctx ...interface{}) {
	LoggerTest.Error(msg, ctx...)
}

func LogCrit(msg string, ctx ...interface{}) {
	LoggerTest.Crit(msg, ctx...)
}
