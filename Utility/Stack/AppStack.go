package Stack

import (
	"errors"
	"fmt"
	"github.com/tauruscorpius/appcommon/Log"
	"os"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	stackDumpBufferSize = 10 * 1024 * 1024
)

var (
	lastStackTime = atomic.Int64{}
)

func DumpAppStack(app string, exit bool) error {
	if !exit {
		currentTime := time.Now().Unix()
		if currentTime-lastStackTime.Load() < 10 {
			return errors.New("stack dump too frequently")
		}
		lastStackTime.Store(currentTime)
	}
	var stackInfo []byte
	stackInfo = make([]byte, stackDumpBufferSize)
	sz := runtime.Stack(stackInfo, true)
	Log.Criticalf("dump stack , stack size : %d\n", sz)
	if sz >= 0 && sz <= len(stackInfo) {
		dumpStackDir := os.Getenv("HOME") + string(os.PathSeparator) + "log"
		os.MkdirAll(dumpStackDir, os.FileMode(0755))
		dumpFile := dumpStackDir + string(os.PathSeparator) + app + ".stack"
		dumpData := fmt.Sprintf("\n==================================\n"+
			"date : %s stack buf size[%d] : detail :\n%s\n%s\n", time.Now().String(), sz, app, stackInfo[:sz])
		writeToFile(dumpFile, dumpData)
	}
	return nil
}

func writeToFile(fileName string, dumpData string) {
	fileHandle, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		Log.Criticalf("Error Create Dump File : %s\n", fileHandle)
		return
	}
	defer fileHandle.Close()
	fileHandle.WriteString(dumpData)
}
