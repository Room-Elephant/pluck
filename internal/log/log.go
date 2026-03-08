package log

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelError
)

var current atomic.Int32

func SupportedLevels() []string {
	return []string{"debug", "info", "error"}
}

func init() {
	current.Store(int32(LevelInfo))
}

func SetLevel(levelString string) {
	switch strings.ToLower(levelString) {
	case "debug":
		current.Store(int32(LevelDebug))
	case "error":
		current.Store(int32(LevelError))
	default:
		current.Store(int32(LevelInfo))
	}
}

func Debugf(format string, args ...any) {
	if Level(current.Load()) <= LevelDebug {
		fmt.Printf("[pluck:debug] "+format+"\n", args...)
	}
}

func Infof(format string, args ...any) {
	if Level(current.Load()) <= LevelInfo {
		fmt.Printf("[pluck] "+format+"\n", args...)
	}
}

func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[pluck:error] "+format+"\n", args...)
}
