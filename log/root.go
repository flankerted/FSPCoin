package log

import (
	"os"
)

var (
	root          = &logger{[]interface{}{}, new(swapHandler)}
	root2         = &logger{[]interface{}{}, new(swapHandler)}
	StdoutHandler = StreamHandler(os.Stdout, LogfmtFormat())
	StderrHandler = StreamHandler(os.Stderr, LogfmtFormat())
	closeTerm     = uint32(0) // 1-close blizzard, 2-close blizcs
	showLine      = false
)

func init() {
	root.SetHandler(DiscardHandler())
	//path := "term.log"
	//fileHandle := LvlFilterHandler(LvlInfo, GetDefaultFileHandle(path))
	//root2.SetHandler(fileHandle)
}

// New returns a new logger with the given context.
// New is a convenient alias for Root().New
func New(ctx ...interface{}) Logger {
	return root.New(ctx...)
}

// Root returns the root logger
func Root() Logger {
	return root
}

// The following functions bypass the exported logger methods (logger.Debug,
// etc.) to keep the call depth the same for all paths to logger.write so
// runtime.Caller(2) always refers to the call site in client code.

// Trace is a convenient alias for Root().Trace
func Trace(msg string, ctx ...interface{}) {
	root.write(msg, LvlTrace, ctx)
	//root2.write(msg, LvlTrace, ctx)
}

// Debug is a convenient alias for Root().Debug
func Debug(msg string, ctx ...interface{}) {
	root.write(msg, LvlDebug, ctx)
	//root2.write(msg, LvlDebug, ctx)
}

// Info is a convenient alias for Root().Info
func Info(msg string, ctx ...interface{}) {
	root.write(msg, LvlInfo, ctx)
	//root2.write(msg, LvlInfo, ctx)
}

// Warn is a convenient alias for Root().Warn
func Warn(msg string, ctx ...interface{}) {
	root.write(msg, LvlWarn, ctx)
	//root2.write(msg, LvlWarn, ctx)
}

// Error is a convenient alias for Root().Error
func Error(msg string, ctx ...interface{}) {
	root.write(msg, LvlError, ctx)
	//root2.write(msg, LvlError, ctx)
}

// Crit is a convenient alias for Root().Crit
func Crit(msg string, ctx ...interface{}) {
	root.write(msg, LvlCrit, ctx)
	//root2.write(msg, LvlCrit, ctx)
	os.Exit(1)
}

func SetTermClose(flag uint32) {
	closeTerm = flag
}

func SetShowLine() {
	showLine = true
}

func Msg(msg string, ctx ...interface{}) {
	// root.write(msg, LvlInfo, ctx)
	// root2.write(msg, LvlInfo, ctx)
}
