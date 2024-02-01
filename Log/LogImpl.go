package Log

import (
	"fmt"
	"github.com/tauruscorpius/appcommon/Consts"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	logPathDelimiter = "."
)

var (
	fileLog     *os.File = nil
	keepLogDays          = 30
	logKey      string
	logMutex    sync.RWMutex
)

func LogCreator(logKey string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	logHome := os.Getenv("HOME") + "/log"
	_ = os.MkdirAll(logHome, 0777)
	timeNow := time.Now()
	newFile := func(f string) *os.File {
		fh, err := os.OpenFile(f, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("Create Log File %s Error : %v, Use Stdout innstead of\n", f, err)
			return os.Stdout
		}
		return fh
	}
	fileName := logHome + "/" + logKey + timeNow.Format(Consts.DateDF) + logPathDelimiter + "log"
	var closeLog *os.File = nil
	st, err := os.Stat(fileName)
	if err != nil {
		closeLog = fileLog
		fileLog = nil
	} else if st.IsDir() {
		fmt.Printf("Log File Conflict with Dir Name : %s\n", fileName)
	}
	if fileLog == nil {
		fh := newFile(fileName)
		l.SetOutput(fh)
		fileLog = fh
	}
	if closeLog != nil {
		_ = closeLog.Close()
		closeLog = nil
	}
}

func SetOutput(logBaseName string) {
	logKey := logBaseName
	LogCreator(logKey)
	go func() {
		for {
			time.Sleep(time.Second)
			LogCreator(logKey)
		}
	}()
}

func CloseOutput() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if fileLog != nil {
		fileLog.Close()
		fileLog = nil
	}
}

func getLogFileCreateDate(file string) string {
	ar := strings.Split(file, logPathDelimiter)
	if len(ar) < 2 {
		return ""
	}
	dateExpect := ar[len(ar)-2]
	if len(dateExpect) != 8 {
		return ""
	}
	_, err := strconv.Atoi(dateExpect)
	if err != nil {
		return ""
	}
	return dateExpect
}

func makeLogSubDir(dir string) bool {
	dirExist, err := IsDirExisting(dir)
	if err != nil {
		return false
	}
	if !dirExist {
		err = os.MkdirAll(dir, 0777)
		if err != nil {
			return false
		}
	}
	return true
}

func moveOldLogFiles(logDir string) {
	f, err := os.OpenFile(logDir, os.O_RDONLY, os.ModeDir)
	if err != nil {
		return
	}
	defer f.Close()

	fileList, _ := f.Readdir(-1)
	for _, v := range fileList {
		if !v.IsDir() && strings.HasPrefix(v.Name(), logKey) && strings.HasSuffix(v.Name(), logPathDelimiter+"log") {
			todayDate := time.Now().Format(Consts.DateDF)
			createDate := getLogFileCreateDate(v.Name())
			if len(createDate) == 0 || todayDate == createDate {
				continue
			}
			moveToName := logDir + "/log" + createDate
			if !makeLogSubDir(moveToName) {
				continue
			}
			os.Rename(logDir+"/"+v.Name(), moveToName+"/"+v.Name())
		}
	}
}

func cleanOldLogFiles(logDir string) {
	f, err := os.OpenFile(logDir, os.O_RDONLY, os.ModeDir)
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now()
	// task run enabled
	unlinkStopPoint := now.AddDate(0, 0, (-1)*keepLogDays)
	cleanMaxDate := unlinkStopPoint.Format(Consts.DateDF)
	fileList, _ := f.Readdir(-1)
	for _, v := range fileList {
		objectName := v.Name()
		if v.IsDir() && strings.HasPrefix(objectName, "log") {
			logDate := objectName[3:]
			if len(logDate) != 8 {
				continue
			}
			if logDate >= cleanMaxDate {
				continue
			}
			oldMothDirClean := logDir + "/" + objectName
			if len(objectName) > 0 && strings.HasSuffix(oldMothDirClean, objectName) {
				Criticalf("CLEAN OLD LOG[%s] DIR[%s] OBJECT[%s] KEEPDAYS[%d]\n",
					logDate, oldMothDirClean, objectName, keepLogDays)
				os.RemoveAll(oldMothDirClean)
			}
		}
	}
}

func ArchiveLogFiles() {
	time.Sleep(5 * time.Second)
	for logKey == "" {
		time.Sleep(time.Second)
	}
	logDir := os.Getenv("HOME") + "/log"
	Criticalf("Old Log Dir Keep Month : %d, LogKey: %s\n", keepLogDays, logKey)

	for {
		moveOldLogFiles(logDir)
		cleanOldLogFiles(logDir)
		time.Sleep(10 * time.Second)
	}
}

func IsDirExisting(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
