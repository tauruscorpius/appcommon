package Log

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tauruscorpius/appcommon/Consts"
)

const (
	logPathDelimiter = "."
)

var (
	keepLogDays       = 30
	logKey            string
	bufferLogWriter   *BufferedLogWriter
	bufferLogWriterMu sync.RWMutex
)

type BufferedLogWriter struct {
	logMutex    sync.Mutex
	logDir      string
	logPrefix   string
	fileName    string
	bufferMutex sync.RWMutex
	buffer      strings.Builder
	chanFlush   chan struct{}
	chanClose   chan struct{}
	chanDone    chan struct{}
	doneOnce    sync.Once
	fileHandle  *os.File
	flushForce  bool
}

func CreateBufferedLogWriter(logKey string) *BufferedLogWriter {
	return createBufferedLogWriter(logKey, "")
}

func createBufferedLogWriter(logKey, logDir string) *BufferedLogWriter {
	if logDir == "" {
		logDir = os.Getenv("HOME") + string(os.PathSeparator) + "log"
	}
	logPrefix := logDir + string(os.PathSeparator) + logKey
	return &BufferedLogWriter{
		logDir:     logDir,
		logPrefix:  logPrefix,
		chanFlush:  make(chan struct{}, 512),
		chanClose:  make(chan struct{}, 512),
		chanDone:   make(chan struct{}),
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
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	if b.fileHandle == nil || err != nil {
		prevFileName := b.fileName
		b.createLogFileLocked(fileName)
		b.fileName = fileName
		return prevFileName
	} else if st.IsDir() {
		fmt.Printf("log file name conflict with dir name : " + fileName)
	}
	return ""
}

func (b *BufferedLogWriter) CreateLogFile(fileName string) {
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	b.createLogFileLocked(fileName)
}

func (b *BufferedLogWriter) createLogFileLocked(fileName string) {
	fh, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("create Log File %s Error : %v, use default stdout\n", fileName, err)
		return
	}

	b.closeLocked()
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
	select {
	case ch <- struct{}{}:
	default:
		fmt.Printf("log writer chan full\n")
	}
}

func (b *BufferedLogWriter) Write(p []byte) (n int, err error) {
	b.bufferMutex.Lock()
	defer b.bufferMutex.Unlock()
	if b.flushForce {
		b.writeFile(p)
		return len(p), nil
	} else {
		if b.buffer.Len() >= MaxBufferedLogFileSize {
			return 0, errors.New("buffer overflow")
		}
		a, e := b.buffer.Write(p)
		if b.buffer.Len() >= WriteLogFileSize || b.flushForce {
			b.writeFlushChan()
		}
		return a, e
	}
}

func (b *BufferedLogWriter) Close() {
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	b.closeLocked()
}

func (b *BufferedLogWriter) closeLocked() {
	if b.fileHandle != nil {
		b.fileHandle.Sync()
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
	b.writeFile([]byte(s))
}

func (b *BufferedLogWriter) writeFile(s []byte) {
	if len(s) == 0 {
		return
	}

	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	fileHandle := b.fileHandle
	if fileHandle == nil {
		fileHandle = os.Stdout
	}
	_, _ = fileHandle.Write(s)
}

func (b *BufferedLogWriter) forceFlush(flush bool) {
	b.bufferMutex.Lock()
	defer b.bufferMutex.Unlock()

	if flush != b.flushForce {
		fmt.Printf("force flush flag change to : %v\n", flush)
		b.flushForce = flush
	}
}

func (b *BufferedLogWriter) autoFlush() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer b.closeDone()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.chanFlush:
			b.flush()
		case <-b.chanClose:
			b.Write([]byte("close log writer\n"))
			b.flush()
			b.Close()
			return
		}
	}
}

func (b *BufferedLogWriter) closeDone() {
	b.doneOnce.Do(func() {
		close(b.chanDone)
	})
}

func (b *BufferedLogWriter) getFileName() string {
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	return b.fileName
}

func SetOutput(logBaseName string) {
	setOutput(logBaseName, "")
}

func setOutput(logBaseName, logDir string) {
	CloseOutput()

	writer := createBufferedLogWriter(logBaseName, logDir)

	bufferLogWriterMu.Lock()
	logKey = logBaseName
	bufferLogWriter = writer
	bufferLogWriterMu.Unlock()

	writer.CheckLogFileRotation()
	l.SetOutput(writer)
	writer.forceFlush(true)

	go writer.autoFlush()
	go func(writer *BufferedLogWriter, logKey string) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-writer.chanDone:
				return
			case <-ticker.C:
				writer.CheckLogDirExists(logKey)
				rotationFile := writer.CheckLogFileRotation()
				if len(rotationFile) > 0 {
					fmt.Printf("log file rotated : file : %s, new file : %s\n", rotationFile, writer.getFileName())
				}
			}
		}
	}(writer, logKey)
	go func(writer *BufferedLogWriter) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-writer.chanDone:
				return
			case <-ticker.C:
				writer.forceFlush(false)
			}
		}
	}(writer)
}

func ForceFlush(forceFlush bool) {
	if writer := getBufferedLogWriter(); writer != nil {
		writer.forceFlush(forceFlush)
	}
}

func CloseOutput() {
	if writer := getBufferedLogWriter(); writer != nil {
		writer.writeCloseChan()
	}
}

func getLogWriterAndKey() (*BufferedLogWriter, string) {
	bufferLogWriterMu.RLock()
	defer bufferLogWriterMu.RUnlock()

	return bufferLogWriter, logKey
}

func getBufferedLogWriter() *BufferedLogWriter {
	bufferLogWriterMu.RLock()
	defer bufferLogWriterMu.RUnlock()

	return bufferLogWriter
}

func getLogKey() string {
	bufferLogWriterMu.RLock()
	defer bufferLogWriterMu.RUnlock()

	return logKey
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
	currentLogKey := getLogKey()
	if currentLogKey == "" {
		return
	}

	f, err := os.OpenFile(logDir, os.O_RDONLY, os.ModeDir)
	if err != nil {
		return
	}
	defer f.Close()

	fileList, _ := f.Readdir(-1)
	for _, v := range fileList {
		if !v.IsDir() && strings.HasPrefix(v.Name(), currentLogKey) && strings.HasSuffix(v.Name(), logPathDelimiter+"log") {
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
	for getLogKey() == "" {
		time.Sleep(time.Second)
	}
	logDir := os.Getenv("HOME") + "/log"
	Criticalf("Old Log Dir Keep Month : %d, LogKey: %s\n", keepLogDays, getLogKey())

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
