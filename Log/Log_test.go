package Log

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func newTestBufferedLogWriter(t *testing.T, logKey string) *BufferedLogWriter {
	t.Helper()
	return createBufferedLogWriter(logKey, t.TempDir())
}

func testBufferLen(writer *BufferedLogWriter) int {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	return writer.buffer.Len()
}

func testResetBuffer(writer *BufferedLogWriter) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	writer.buffer.Reset()
}

func testFillBuffer(writer *BufferedLogWriter, data []byte) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	writer.buffer.Write(data)
}

func TestLogOut(t *testing.T) {
	setOutput("test_LogOut", t.TempDir())
	for i := 0; i < 10; i++ {
		Criticalf("test")
		time.Sleep(time.Second)
	}
	CloseOutput()
	time.Sleep(time.Second)
}

func TestForceFlush(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_force_flush")
	writer.CheckLogDirExists("test_force_flush")
	writer.CheckLogFileRotation()

	go writer.autoFlush()

	t.Run("ForceFlush_EnablesImmediateFlush", func(t *testing.T) {
		writer.forceFlush(true)
		if !writer.flushForce {
			t.Errorf("Expected flushForce to be true, got false")
		}

		testData := []byte("test log entry with force flush enabled\n")
		n, err := writer.Write(testData)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}

		time.Sleep(100 * time.Millisecond)

		bufferLen := testBufferLen(writer)
		if bufferLen != 0 {
			t.Errorf("Expected buffer to be empty with force flush (immediate write), got %d bytes", bufferLen)
		}
	})

	t.Run("ForceFlush_DisablesImmediateFlush", func(t *testing.T) {
		writer.forceFlush(false)
		if writer.flushForce {
			t.Errorf("Expected flushForce to be false, got true")
		}
	})

	t.Run("ForceFlush_GlobalFunction", func(t *testing.T) {
		setOutput("test_global_force_flush", t.TempDir())
		defer CloseOutput()

		ForceFlush(true)
		writer, _ := getLogWriterAndKey()
		if writer == nil {
			t.Fatal("bufferLogWriter should not be nil after SetOutput")
		}
		flushForce := writer.isForceFlush()
		if !flushForce {
			t.Errorf("Expected global flushForce to be true")
		}

		ForceFlush(false)
		flushForce = writer.isForceFlush()
		if flushForce {
			t.Errorf("Expected global flushForce to be false")
		}
	})

	t.Run("ForceFlush_ImmediateWrite_BypassesBuffer", func(t *testing.T) {
		writer.forceFlush(true)

		initialBufferLen := testBufferLen(writer)
		testData := []byte("immediate write test\n")
		n, err := writer.Write(testData)

		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}

		bufferLen := testBufferLen(writer)
		if bufferLen != initialBufferLen {
			t.Errorf("Expected buffer length to remain %d (bypass buffer), got %d", initialBufferLen, bufferLen)
		}
	})

	writer.writeCloseChan()
	time.Sleep(100 * time.Millisecond)
}

func TestBufferedLogWriter_Write(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_write")
	writer.CheckLogDirExists("test_write")
	writer.CheckLogFileRotation()

	go writer.autoFlush()
	defer func() {
		writer.writeCloseChan()
		time.Sleep(100 * time.Millisecond)
	}()

	t.Run("Write_SmallData", func(t *testing.T) {
		testData := []byte("small log entry\n")
		n, err := writer.Write(testData)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(testData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
		}
	})

	t.Run("Write_LargeData_TriggersFlush", func(t *testing.T) {
		largeData := []byte(strings.Repeat("x", WriteLogFileSize+100) + "\n")
		n, err := writer.Write(largeData)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(largeData) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(largeData), n)
		}

		time.Sleep(100 * time.Millisecond)
	})

	t.Run("Write_BufferOverflow", func(t *testing.T) {
		testWriter := newTestBufferedLogWriter(t, "test_overflow")
		testWriter.CheckLogDirExists("test_overflow")
		testWriter.CheckLogFileRotation()
		defer testWriter.Close()

		testWriter.forceFlush(false)

		fillData := []byte(strings.Repeat("x", MaxBufferedLogFileSize))
		testFillBuffer(testWriter, fillData)

		additionalData := []byte("overflow\n")
		_, err := testWriter.Write(additionalData)
		if err == nil {
			t.Error("Expected buffer overflow error, got nil")
		}
		if err != nil && err.Error() != "buffer overflow" {
			t.Errorf("Expected 'buffer overflow' error, got: %v", err)
		}
	})
}

func TestBufferedLogWriter_Flush(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_flush")
	writer.CheckLogDirExists("test_flush")
	writer.CheckLogFileRotation()

	go writer.autoFlush()
	defer func() {
		writer.writeCloseChan()
		time.Sleep(100 * time.Millisecond)
	}()

	t.Run("Flush_WritesBufferToFile", func(t *testing.T) {
		writer.forceFlush(false)
		testData := []byte("test flush data\n")
		writer.Write(testData)

		writer.writeFlushChan()
		time.Sleep(100 * time.Millisecond)

		bufferLen := testBufferLen(writer)
		if bufferLen > 0 {
			t.Logf("Buffer has %d bytes after flush", bufferLen)
		}
	})

	t.Run("Flush_EmptyBuffer", func(t *testing.T) {
		testResetBuffer(writer)
		writer.flush()
	})
}

func TestBufferedLogWriter_WriteFile(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_write_file")
	writer.CheckLogDirExists("test_write_file")
	writer.CheckLogFileRotation()
	defer writer.Close()

	t.Run("WriteFile_WithContent", func(t *testing.T) {
		testContent := []byte("direct write file test\n")
		writer.writeFile(testContent)

		writer.flush()
	})

	t.Run("WriteFile_EmptyBytes", func(t *testing.T) {
		writer.writeFile([]byte{})
	})

	t.Run("WriteFile_NoFileHandle_UsesStdout", func(t *testing.T) {
		testWriter := newTestBufferedLogWriter(t, "test_no_handle")
		testWriter.writeFile([]byte("stdout test\n"))
	})

	t.Run("WriteFile_NilBytes", func(t *testing.T) {
		writer.writeFile(nil)
	})
}

func TestBufferedLogWriter_FileRotation(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_rotation")
	writer.CheckLogDirExists("test_rotation")

	t.Run("CheckLogFileRotation_CreatesNewFile", func(t *testing.T) {
		prevFile := writer.CheckLogFileRotation()
		if writer.fileName == "" {
			t.Error("Expected fileName to be set after rotation check")
		}
		if !writer.isFileOpen() {
			t.Error("Expected fileHandle to be created")
		}
		if prevFile != "" {
			t.Logf("Previous file was: %s", prevFile)
		}
	})

	t.Run("CheckLogFileRotation_SameDay", func(t *testing.T) {
		currentFile := writer.fileName
		prevFile := writer.CheckLogFileRotation()
		if prevFile != "" {
			t.Errorf("Expected no rotation on same day, got previous file: %s", prevFile)
		}
		if writer.fileName != currentFile {
			t.Errorf("Expected fileName to remain %s, got %s", currentFile, writer.fileName)
		}
	})

	writer.Close()
}

func TestBufferedLogWriter_Close(t *testing.T) {
	writer := newTestBufferedLogWriter(t, "test_close")
	writer.CheckLogDirExists("test_close")
	writer.CheckLogFileRotation()

	if !writer.isFileOpen() {
		t.Fatal("Expected fileHandle to be created")
	}

	writer.Close()

	if writer.isFileOpen() {
		t.Error("Expected fileHandle to be nil after Close")
	}

	writer.Close()
}

func TestGetLogFileCreateDate(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		expected string
	}{
		{
			name:     "ValidLogFile",
			fileName: "app.20240105.log",
			expected: "20240105",
		},
		{
			name:     "InvalidFormat_NoDelimiter",
			fileName: "app_20240105_log",
			expected: "",
		},
		{
			name:     "InvalidFormat_ShortDate",
			fileName: "app.2024.log",
			expected: "",
		},
		{
			name:     "InvalidFormat_NonNumericDate",
			fileName: "app.abcd1234.log",
			expected: "",
		},
		{
			name:     "ValidLogFile_MultipleDelimiters",
			fileName: "app.service.20240105.log",
			expected: "20240105",
		},
		{
			name:     "ValidRollingLogFile",
			fileName: "app_20240105_120305_001.log",
			expected: "20240105",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLogFileCreateDate(tt.fileName)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsDirExisting(t *testing.T) {
	tempDir := os.TempDir() + "/test_log_dir_exists"
	os.RemoveAll(tempDir)

	t.Run("DirNotExists", func(t *testing.T) {
		exists, err := IsDirExisting(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if exists {
			t.Error("Expected directory to not exist")
		}
	})

	t.Run("DirExists", func(t *testing.T) {
		os.MkdirAll(tempDir, 0777)
		defer os.RemoveAll(tempDir)

		exists, err := IsDirExisting(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !exists {
			t.Error("Expected directory to exist")
		}
	})
}

func TestSetOutputAndCloseOutput(t *testing.T) {
	t.Run("SetOutput_InitializesWriter", func(t *testing.T) {
		setOutput("test_set_output", t.TempDir())
		defer CloseOutput()

		writer, _ := getLogWriterAndKey()
		if writer == nil {
			t.Fatal("Expected bufferLogWriter to be initialized")
		}
		Criticalf("test log message")
		if writer.getFileName() == "" {
			t.Error("Expected fileName to be set")
		}
	})

	t.Run("CloseOutput_CleansUp", func(t *testing.T) {
		setOutput("test_close_output", t.TempDir())
		CloseOutput()
		time.Sleep(200 * time.Millisecond)
	})
}

func TestBufferedLogWriter_RollingBySizeAndMaxBackups(t *testing.T) {
	logDir := t.TempDir()
	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_rolling",
		MaxFileSize:   16,
		MaxBackups:    2,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})

	for i := 0; i < 4; i++ {
		if _, err := writer.Write([]byte("0123456789abcdef\n")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	writer.Close()

	matches, err := filepath.Glob(filepath.Join(logDir, "test_rolling_*.log"))
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	var archiveMatches []string
	for _, name := range matches {
		if _, _, ok := writer.parseRollingLogFile(filepath.Base(name)); ok {
			archiveMatches = append(archiveMatches, name)
		}
	}
	if len(archiveMatches) != 2 {
		t.Fatalf("Expected max 2 log backups, got %d: %+v", len(archiveMatches), archiveMatches)
	}
	for _, name := range archiveMatches {
		if getLogFileCreateDate(filepath.Base(name)) == "" {
			t.Fatalf("Expected rolling log filename to include date: %s", name)
		}
	}
}

func TestBufferedLogWriter_CurrentLogFileUsesDatedTimeName(t *testing.T) {
	logDir := t.TempDir()
	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_current",
		MaxFileSize:   16,
		MaxBackups:    10,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})
	defer writer.Close()

	if _, err := writer.Write([]byte("current log line\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	pattern := regexp.MustCompile(`^test_current_\d{8}_\d{6}\.log$`)
	if !pattern.MatchString(filepath.Base(writer.getFileName())) {
		t.Fatalf("expected current log file to use dated time name, got %s", writer.getFileName())
	}
}

func TestBufferedLogWriter_DoesNotArchiveOrphanCurrentLogFileOnStart(t *testing.T) {
	logDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(logDir, "test_orphan_20260911_151929_00019.log"), []byte("archive"), 0666); err != nil {
		t.Fatalf("write seed archive failed: %v", err)
	}
	orphanName := filepath.Join(logDir, "test_orphan_20260911_151952.log")
	if err := os.WriteFile(orphanName, []byte("orphan current"), 0666); err != nil {
		t.Fatalf("write orphan current failed: %v", err)
	}

	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_orphan",
		MaxFileSize:   1024,
		MaxBackups:    10,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})
	defer writer.Close()

	now := time.Date(2026, 9, 11, 15, 23, 21, 0, time.Local)
	if err := writer.rotateLocked(now); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if _, err := os.Stat(orphanName); err != nil {
		t.Fatalf("expected startup to leave orphan current log untouched: %v", err)
	}
	archivedName := filepath.Join(logDir, "test_orphan_20260911_151952_00020.log")
	if _, err := os.Stat(archivedName); !os.IsNotExist(err) {
		t.Fatalf("expected startup not to archive orphan current log, stat err: %v", err)
	}
	currentName := filepath.Join(logDir, "test_orphan_20260911_152321.log")
	if writer.getFileName() != currentName {
		t.Fatalf("expected new current log file %s, got %s", currentName, writer.getFileName())
	}
}

func TestBufferedLogWriter_SequenceResetsPerDayAndUsesFiveDigits(t *testing.T) {
	logDir := t.TempDir()
	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_sequence",
		MaxFileSize:   16,
		MaxBackups:    10,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})
	defer writer.Close()

	firstDay := time.Date(2026, 9, 11, 10, 0, 0, 0, time.Local)
	if err := writer.rotateLocked(firstDay); err != nil {
		t.Fatalf("first day first rotate failed: %v", err)
	}
	if err := writer.rotateLocked(firstDay.Add(time.Second)); err != nil {
		t.Fatalf("first day second rotate failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, "test_sequence_20260911_100000_00001.log")); err != nil {
		t.Fatalf("expected first archived sequence to use five digits: %v", err)
	}

	secondDay := firstDay.AddDate(0, 0, 1)
	if err := writer.rotateLocked(secondDay); err != nil {
		t.Fatalf("second day rotate failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, "test_sequence_20260911_100001_00002.log")); err != nil {
		t.Fatalf("expected same-day sequence to increment: %v", err)
	}
	if err := writer.rotateLocked(secondDay.Add(time.Second)); err != nil {
		t.Fatalf("second day archive rotate failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, "test_sequence_20260912_100000_00001.log")); err != nil {
		t.Fatalf("expected next-day sequence to reset: %v", err)
	}

	if _, _, ok := writer.parseRollingLogFile("test_sequence_20260911_100000_001.log"); ok {
		t.Fatal("expected three-digit sequence log filename to be invalid")
	}
	if _, _, ok := writer.parseRollingLogFile("test_sequence_20260911_100000_00001.log"); !ok {
		t.Fatal("expected five-digit sequence log filename to be valid")
	}
}

func TestBufferedLogWriter_SequenceWrapsAfterFiveDigitLimit(t *testing.T) {
	logDir := t.TempDir()
	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_sequence_wrap",
		MaxFileSize:   16,
		MaxBackups:    10,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})
	defer writer.Close()

	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.Local)
	if err := os.WriteFile(filepath.Join(logDir, "test_sequence_wrap_20260911_095959_99998.log"), []byte("old"), 0666); err != nil {
		t.Fatalf("write seed archive failed: %v", err)
	}
	if err := writer.rotateLocked(now); err != nil {
		t.Fatalf("initial rotate failed: %v", err)
	}
	if err := writer.rotateLocked(now.Add(time.Second)); err != nil {
		t.Fatalf("rotate to max sequence failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, "test_sequence_wrap_20260911_100000_99999.log")); err != nil {
		t.Fatalf("expected sequence to reach five-digit limit: %v", err)
	}

	if err := writer.rotateLocked(now.Add(2 * time.Second)); err != nil {
		t.Fatalf("rotate after max sequence failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(logDir, "test_sequence_wrap_20260911_100001_00001.log")); err != nil {
		t.Fatalf("expected sequence to wrap to 00001: %v", err)
	}
}

func TestBufferedLogWriter_RecreatesRemovedCurrentLogFile(t *testing.T) {
	logDir := t.TempDir()
	writer := newBufferedLogWriter(RollingLogConfig{
		Dir:           logDir,
		LogKey:        "test_removed",
		MaxFileSize:   1024,
		MaxBackups:    10,
		FlushInterval: time.Hour,
		BufferSize:    1,
	})
	defer writer.Close()

	first := []byte("first log line\n")
	if _, err := writer.Write(first); err != nil {
		t.Fatalf("first Write failed: %v", err)
	}
	removedFile := writer.getFileName()
	if removedFile == "" {
		t.Fatal("expected first write to create log file")
	}
	if err := os.Remove(removedFile); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	second := []byte("second log line\n")
	if _, err := writer.Write(second); err != nil {
		t.Fatalf("second Write failed: %v", err)
	}
	recreatedFile := writer.getFileName()
	if recreatedFile == "" {
		t.Fatal("expected second write to recreate log file")
	}
	if _, err := os.Stat(recreatedFile); err != nil {
		t.Fatalf("expected recreated log file to exist: %v", err)
	}

	writer.mu.Lock()
	currentSize := writer.currentSize
	writer.mu.Unlock()
	if currentSize != int64(len(second)) {
		t.Fatalf("expected currentSize to reset to %d, got %d", len(second), currentSize)
	}
}

func TestCloseOutput_FlushesBufferedLogs(t *testing.T) {
	logDir := t.TempDir()
	setOutput("test_close_flush", logDir)
	ForceFlush(false)
	Infof("buffered close flush")
	CloseOutput()

	matches, err := filepath.Glob(filepath.Join(logDir, "test_close_flush_*.log"))
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Expected one flushed archive log file, got %d: %+v", len(matches), matches)
	}
	if _, _, ok := getBufferedLogWriter().parseRollingLogFile(filepath.Base(matches[0])); !ok {
		t.Fatalf("Expected CloseOutput to archive current log file, got %s", matches[0])
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "buffered close flush") {
		t.Fatalf("Expected CloseOutput to flush buffered log, got: %s", string(data))
	}
}
