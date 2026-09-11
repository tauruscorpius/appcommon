package Log

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tauruscorpius/appcommon/Consts"
)

const (
	defaultMaxLogFileSize  int64 = 100 * 1024 * 1024
	defaultMaxLogBackups         = 30
	defaultFlushInterval         = time.Second
	defaultLogBufferSize         = 1024 * 1024
	MaxBufferedLogFileSize       = 50 * 1024 * 1024
	WriteLogFileSize             = defaultLogBufferSize
	maxRollingLogSequence        = 99999
)

type RollingLogConfig struct {
	Dir           string
	LogKey        string
	MaxFileSize   int64
	MaxBackups    int
	FlushInterval time.Duration
	BufferSize    int
}

var (
	logKey            string
	bufferLogWriter   *BufferedLogWriter
	bufferLogWriterMu sync.RWMutex
)

type BufferedLogWriter struct {
	mu          sync.Mutex
	cfg         RollingLogConfig
	buffer      strings.Builder
	fileHandle  *os.File
	fileName    string
	activeDate  string
	sequence    uint64
	currentSize int64
	flushForce  bool
	closed      bool

	chanFlush chan struct{}
	chanClose chan struct{}
	chanDone  chan struct{}
	doneOnce  sync.Once
}

func CreateBufferedLogWriter(logKey string) *BufferedLogWriter {
	return newBufferedLogWriter(defaultRollingLogConfig(logKey, ""))
}

func createBufferedLogWriter(logKey, logDir string) *BufferedLogWriter {
	return newBufferedLogWriter(defaultRollingLogConfig(logKey, logDir))
}

func newBufferedLogWriter(cfg RollingLogConfig) *BufferedLogWriter {
	cfg = withRollingLogConfigDefaults(cfg)
	return &BufferedLogWriter{
		cfg:       cfg,
		chanFlush: make(chan struct{}, 512),
		chanClose: make(chan struct{}, 1),
		chanDone:  make(chan struct{}),
	}
}

func SetOutput(logBaseName string) {
	SetOutputWithConfig(RollingLogConfig{LogKey: logBaseName})
}

func SetOutputWithConfig(cfg RollingLogConfig) {
	CloseOutput()

	writer := newBufferedLogWriter(cfg)

	bufferLogWriterMu.Lock()
	logKey = writer.cfg.LogKey
	bufferLogWriter = writer
	bufferLogWriterMu.Unlock()

	l.SetOutput(writer)
	writer.forceFlush(true)
	go writer.autoFlush()
}

func setOutput(logBaseName, logDir string) {
	SetOutputWithConfig(RollingLogConfig{LogKey: logBaseName, Dir: logDir})
}

func defaultRollingLogConfig(logKey, logDir string) RollingLogConfig {
	if logDir == "" {
		logDir = filepath.Join(os.Getenv("HOME"), "log")
	}
	return RollingLogConfig{
		Dir:           logDir,
		LogKey:        logKey,
		MaxFileSize:   defaultMaxLogFileSize,
		MaxBackups:    defaultMaxLogBackups,
		FlushInterval: defaultFlushInterval,
		BufferSize:    defaultLogBufferSize,
	}
}

func withRollingLogConfigDefaults(cfg RollingLogConfig) RollingLogConfig {
	if cfg.Dir == "" {
		cfg.Dir = filepath.Join(os.Getenv("HOME"), "log")
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = defaultMaxLogFileSize
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 0
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = defaultLogBufferSize
	}
	return cfg
}

func (b *BufferedLogWriter) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0, errors.New("log writer closed")
	}
	if b.buffer.Len()+len(p) > MaxBufferedLogFileSize {
		return 0, errors.New("buffer overflow")
	}
	n, err = b.buffer.Write(p)
	if err != nil {
		return n, err
	}
	if b.flushForce || b.buffer.Len() >= b.cfg.BufferSize {
		if err := b.flushLocked(); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (b *BufferedLogWriter) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	if err := b.flushLocked(); err != nil {
		fmt.Printf("flush log on close error : %v\n", err)
	}
	b.closeFileLocked()
	b.closed = true
	b.closeDone()
}

func (b *BufferedLogWriter) closeAndWait() {
	b.writeCloseChan()
	<-b.chanDone
}

func (b *BufferedLogWriter) CheckLogDirExists(logKey string) bool {
	err := os.MkdirAll(b.cfg.Dir, 0777)
	if err != nil {
		fmt.Printf("create log dir %s error : %v\n", b.cfg.Dir, err)
		return false
	}
	return true
}

func (b *BufferedLogWriter) CheckLogFileRotation() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format(Consts.DateDF)
	if b.fileHandle != nil && b.activeDate == today {
		return ""
	}
	prev := b.fileName
	if err := b.rotateLocked(time.Now()); err != nil {
		fmt.Printf("rotate log file error : %v\n", err)
		return prev
	}
	return prev
}

func (b *BufferedLogWriter) CreateLogFile(fileName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.createLogFileLocked(fileName); err != nil {
		fmt.Printf("create Log File %s Error : %v, use default stdout\n", fileName, err)
	}
}

func (b *BufferedLogWriter) writeCloseChan() {
	select {
	case b.chanClose <- struct{}{}:
	case <-b.chanDone:
	}
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

func (b *BufferedLogWriter) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.flushLocked(); err != nil {
		fmt.Printf("flush log error : %v\n", err)
	}
}

func (b *BufferedLogWriter) flushLocked() error {
	if b.buffer.Len() == 0 {
		return nil
	}
	data := b.buffer.String()
	b.buffer.Reset()
	return b.writeFileLocked([]byte(data))
}

func (b *BufferedLogWriter) writeFile(s []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.writeFileLocked(s); err != nil {
		fmt.Printf("write log file error : %v\n", err)
	}
}

func (b *BufferedLogWriter) writeFileLocked(s []byte) error {
	if len(s) == 0 {
		return nil
	}
	if err := b.ensureLogFileLocked(len(s)); err != nil {
		_, _ = os.Stdout.Write(s)
		return err
	}
	n, err := b.fileHandle.Write(s)
	b.currentSize += int64(n)
	return err
}

func (b *BufferedLogWriter) ensureLogFileLocked(nextWriteBytes int) error {
	now := time.Now()
	today := now.Format(Consts.DateDF)
	if b.fileHandle == nil || b.activeDate != today || b.currentLogFileRemovedLocked() || b.currentSize+int64(nextWriteBytes) > b.cfg.MaxFileSize {
		return b.rotateLocked(now)
	}
	return nil
}

func (b *BufferedLogWriter) currentLogFileRemovedLocked() bool {
	if b.fileName == "" {
		return false
	}
	st, err := os.Stat(b.fileName)
	if err == nil {
		b.currentSize = st.Size()
		return false
	}
	if os.IsNotExist(err) {
		b.closeFileLocked()
		b.currentSize = 0
		return true
	}
	return false
}

func (b *BufferedLogWriter) rotateLocked(now time.Time) error {
	if err := os.MkdirAll(b.cfg.Dir, 0777); err != nil {
		return err
	}
	today := now.Format(Consts.DateDF)
	if b.activeDate != today {
		b.activeDate = today
		b.sequence = b.maxSequenceForDateLocked(today)
	}
	if b.sequence >= maxRollingLogSequence {
		b.sequence = 0
	}
	b.sequence++

	fileName := b.buildLogFileName(now, b.sequence)
	if err := b.createLogFileLocked(filepath.Join(b.cfg.Dir, fileName)); err != nil {
		return err
	}
	b.fileName = filepath.Join(b.cfg.Dir, fileName)
	b.currentSize = 0
	b.cleanBackupsLocked()
	return nil
}

func (b *BufferedLogWriter) buildLogFileName(now time.Time, sequence uint64) string {
	return fmt.Sprintf("%s_%s_%s_%05d.log", b.cfg.LogKey, now.Format(Consts.DateDF), now.Format("150405"), sequence)
}

func (b *BufferedLogWriter) createLogFileLocked(fileName string) error {
	fh, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	b.closeFileLocked()
	b.fileHandle = fh
	return nil
}

func (b *BufferedLogWriter) closeFileLocked() {
	if b.fileHandle != nil {
		b.fileHandle.Sync()
		b.fileHandle.Close()
		b.fileHandle = nil
	}
}

func (b *BufferedLogWriter) forceFlush(flush bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if flush != b.flushForce {
		fmt.Printf("force flush flag change to : %v\n", flush)
		b.flushForce = flush
	}
}

func (b *BufferedLogWriter) autoFlush() {
	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()
	defer b.closeDone()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.chanFlush:
			b.flush()
		case <-b.chanClose:
			b.mu.Lock()
			_, _ = b.buffer.WriteString("close log writer\n")
			if err := b.flushLocked(); err != nil {
				fmt.Printf("flush log on close error : %v\n", err)
			}
			b.closeFileLocked()
			b.closed = true
			b.mu.Unlock()
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
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.fileName
}

func (b *BufferedLogWriter) bufferLen() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Len()
}

func (b *BufferedLogWriter) resetBuffer() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer.Reset()
}

func (b *BufferedLogWriter) isForceFlush() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.flushForce
}

func (b *BufferedLogWriter) isFileOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.fileHandle != nil
}

func ForceFlush(forceFlush bool) {
	if writer := getBufferedLogWriter(); writer != nil {
		writer.forceFlush(forceFlush)
	}
}

func CloseOutput() {
	if writer := getBufferedLogWriter(); writer != nil {
		writer.closeAndWait()
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

func (b *BufferedLogWriter) logFilesLocked() []os.FileInfo {
	dirEntries, err := os.ReadDir(b.cfg.Dir)
	if err != nil {
		return nil
	}
	var files []os.FileInfo
	for _, entry := range dirEntries {
		if entry.IsDir() || !b.isRollingLogFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			files = append(files, info)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	return files
}

func (b *BufferedLogWriter) cleanBackupsLocked() {
	if b.cfg.MaxBackups <= 0 {
		return
	}
	files := b.logFilesLocked()
	for len(files) > b.cfg.MaxBackups {
		_ = os.Remove(filepath.Join(b.cfg.Dir, files[0].Name()))
		files = files[1:]
	}
}

func (b *BufferedLogWriter) maxSequenceForDateLocked(date string) uint64 {
	files := b.logFilesLocked()
	var maxSeq uint64
	for _, file := range files {
		parsedDate, seq, ok := b.parseRollingLogFile(file.Name())
		if ok && parsedDate == date && seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq
}

func (b *BufferedLogWriter) isRollingLogFile(fileName string) bool {
	_, _, ok := b.parseRollingLogFile(fileName)
	return ok
}

func (b *BufferedLogWriter) parseRollingLogFile(fileName string) (string, uint64, bool) {
	prefix := b.cfg.LogKey + "_"
	if !strings.HasPrefix(fileName, prefix) || !strings.HasSuffix(fileName, ".log") {
		return "", 0, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(fileName, prefix), ".log")
	parts := strings.Split(body, "_")
	if len(parts) != 3 || len(parts[0]) != 8 || len(parts[1]) != 6 || len(parts[2]) != 5 {
		return "", 0, false
	}
	if _, err := time.Parse("20060102_150405", parts[0]+"_"+parts[1]); err != nil {
		return "", 0, false
	}
	seq, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return parts[0], seq, true
}

func getLogFileCreateDate(file string) string {
	if strings.Contains(file, ".") {
		parts := strings.Split(file, ".")
		for _, part := range parts {
			if len(part) != 8 {
				continue
			}
			if _, err := strconv.Atoi(part); err == nil {
				return part
			}
		}
	}

	parts := strings.Split(strings.TrimSuffix(file, ".log"), "_")
	if len(parts) < 3 {
		return ""
	}
	dateExpect := parts[len(parts)-3]
	if len(dateExpect) != 8 {
		return ""
	}
	_, err := strconv.Atoi(dateExpect)
	if err != nil {
		return ""
	}
	return dateExpect
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
