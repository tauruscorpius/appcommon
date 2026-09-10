package Log

import (
	"os"
	"strings"
	"testing"
	"time"
)

func newTestBufferedLogWriter(t *testing.T, logKey string) *BufferedLogWriter {
	t.Helper()
	return createBufferedLogWriter(logKey, t.TempDir())
}

func testBufferLen(writer *BufferedLogWriter) int {
	writer.bufferMutex.Lock()
	defer writer.bufferMutex.Unlock()

	return writer.buffer.Len()
}

func testResetBuffer(writer *BufferedLogWriter) {
	writer.bufferMutex.Lock()
	defer writer.bufferMutex.Unlock()

	writer.buffer.Reset()
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
		writer.bufferMutex.Lock()
		flushForce := writer.flushForce
		writer.bufferMutex.Unlock()
		if !flushForce {
			t.Errorf("Expected global flushForce to be true")
		}

		ForceFlush(false)
		writer.bufferMutex.Lock()
		flushForce = writer.flushForce
		writer.bufferMutex.Unlock()
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
		testWriter.buffer.Write(fillData)

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

		if writer.fileHandle != nil {
			writer.fileHandle.Sync()
		}
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
		if writer.fileHandle == nil {
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

	if writer.fileHandle == nil {
		t.Fatal("Expected fileHandle to be created")
	}

	writer.Close()

	if writer.fileHandle != nil {
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
		if writer.getFileName() == "" {
			t.Error("Expected fileName to be set")
		}

		Criticalf("test log message")
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("CloseOutput_CleansUp", func(t *testing.T) {
		setOutput("test_close_output", t.TempDir())
		CloseOutput()
		time.Sleep(200 * time.Millisecond)
	})
}
