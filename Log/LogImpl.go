package Log

import (
	"errors"
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
	keepLogDays = 30
	logKey      string
)

var bufferLogWriter *BufferedLogWriter = nil

type BufferedLogWriter struct {
	logMutex    sync.Mutex
	logDir      string
	logPrefix   string
	fileName    string
	bufferMutex sync.RWMutex
	buffer      strings.Builder
	chanFlush   chan struct{}
	chanClose   chan struct{}
	fileHandle  *os.File
}

func CreateBufferedLogWriter(logKey string) *BufferedLogWriter {
	logDir := os.Getenv("HOME") + string(os.PathSeparator) + "log"
	logPrefix := logDir + string(os.PathSeparator) + logKey
	return &BufferedLogWriter{
		logDir:     logDir,
		logPrefix:  logPrefix,
		chanFlush:  make(chan struct{}, 512),
		chanClose:  make(chan struct{}, 512),
		fileHandle: nil,
	}
}

func (b *BufferedLogWriter) CheckLogDirExists(logKey string) bool {
	err := os.MkdirAll(b.logDir, 0777)
	if err != nil {
		fmt.Printf("create log dir %s error : %v\n", b.logDir, err)
		return false
	}
	return true
}

func (b *BufferedLogWriter) CheckLogFileRotation() string {
	fileName := b.logPrefix + "_" + time.Now().Format(Consts.DateDF) + ".log"
	st, err := os.Stat(fileName)
	if b.fileHandle == nil || err != nil {
		b.CreateLogFile(fileName)
		prevFileName := b.fileName
		b.fileName = fileName
		return prevFileName
	} else if st.IsDir() {
		fmt.Printf("log file name conflict with dir name : " + fileName)
	}
	return ""
}

func (b *BufferedLogWriter) CreateLogFile(fileName string) {
	b.Close()
	fh, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("create Log File %s Error : %v, use default stdout\n", fileName, err)
		return
	}
	b.fileHandle = fh
}

const WriteLogFileSize = 1024 * 1024
const MaxBufferedLogFileSize = 50 * 1024 * 1024

func (b *BufferedLogWriter) writeCloseChan() {
	b.writeLogChan(b.chanClose)
}

func (b *BufferedLogWriter) writeFlushChan() {
	b.writeLogChan(b.chanFlush)
}

func (b *BufferedLogWriter) writeLogChan(ch chan struct{}) {
	if len(ch) >= cap(ch) {
		fmt.Printf("log writer chan full\n")
		return
	}
	ch <- struct{}{}
}

func (b *BufferedLogWriter) Write(p []byte) (n int, err error) {
	b.bufferMutex.Lock()
	defer b.bufferMutex.Unlock()

	if b.buffer.Len() >= MaxBufferedLogFileSize {
		return 0, errors.New("buffer overflow")
	}
	a, e := b.buffer.Write(p)
	if b.buffer.Len() >= WriteLogFileSize {
		b.writeFlushChan()
	}
	return a, e
}

func (b *BufferedLogWriter) Close() {
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	if b.fileHandle != nil {
		b.fileHandle.Close()
		b.fileHandle = nil
	}
}

func (b *BufferedLogWriter) getBuffer() string {
	b.bufferMutex.Lock()
	defer b.bufferMutex.Unlock()
	if b.buffer.Len() == 0 {
		return ""
	}
	var s string
	s += b.buffer.String()
	b.buffer.Reset()
	return s
}

func (b *BufferedLogWriter) flush() {
	s := b.getBuffer()
	if len(s) > 0 {
		fileHandle := b.fileHandle
		if fileHandle == nil {
			fileHandle = os.Stdout
		}
		fileHandle.Write([]byte(s))
		fileHandle.Sync()
	}
}

func (b *BufferedLogWriter) autoFlush() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.chanFlush:
			b.flush()
		case <-b.chanClose:
			b.flush()
			b.Close()
			return
		}
	}
}

func SetOutput(logBaseName string) {
	logKey = logBaseName
	bufferLogWriter = CreateBufferedLogWriter(logKey)
	bufferLogWriter.CheckLogFileRotation()
	l.SetOutput(bufferLogWriter)
	go bufferLogWriter.autoFlush()
	go func() {
		for {
			bufferLogWriter.CheckLogDirExists(logKey)
			rotationFile := bufferLogWriter.CheckLogFileRotation()
			if len(rotationFile) > 0 {
				fmt.Printf("log file rotated : file : %s, new file : %s\n", rotationFile, bufferLogWriter.fileName)
			}
			time.Sleep(time.Second)
		}
	}()
}

func CloseOutput() {
	if bufferLogWriter != nil {
		bufferLogWriter.writeCloseChan()
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
