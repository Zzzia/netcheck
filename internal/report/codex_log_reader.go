package report

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"time"
)

const initialCodexSearchBytes int64 = 4 * 1024 * 1024

func openCodexLogReader(logPath string, start time.Time) (io.ReadCloser, bool, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	size := stat.Size()
	if size == 0 {
		return io.NopCloser(bytes.NewReader(nil)), true, nil
	}

	window := initialCodexSearchBytes
	if window > size {
		window = size
	}
	for {
		data, exactLineNumbers, err := readCodexTail(file, size, window)
		if err != nil {
			return nil, false, err
		}
		if exactLineNumbers || codexTailStartsBefore(data, start) {
			return io.NopCloser(bytes.NewReader(data)), exactLineNumbers, nil
		}
		nextWindow := window * 2
		if nextWindow > size {
			nextWindow = size
		}
		if nextWindow == window {
			return io.NopCloser(bytes.NewReader(data)), exactLineNumbers, nil
		}
		window = nextWindow
	}
}

func readCodexTail(file *os.File, size, window int64) ([]byte, bool, error) {
	offset := size - window
	data := make([]byte, window)
	if _, err := file.ReadAt(data, offset); err != nil && err != io.EOF {
		return nil, false, err
	}
	if offset == 0 {
		return data, true, nil
	}
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 || newline+1 >= len(data) {
		return data, false, nil
	}
	return data[newline+1:], false, nil
}

func codexTailStartsBefore(data []byte, start time.Time) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		item, ok := parseCodexLine(scanner.Text(), 0)
		if !ok {
			continue
		}
		return !item.ts.After(start)
	}
	return false
}
