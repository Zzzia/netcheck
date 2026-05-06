package report

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"time"
)

const initialCodexTailBytes int64 = 8 * 1024 * 1024

func openCodexLogWindow(logPath string, start time.Time) (io.ReadCloser, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	window := initialCodexTailBytes
	if window > size {
		window = size
	}
	for {
		data, err := readCodexTail(file, size, window)
		if err != nil {
			return nil, err
		}
		if window == size || codexTailStartsBefore(data, start) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
		next := window * 2
		if next > size {
			next = size
		}
		if next == window {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
		window = next
	}
}

func readCodexTail(file *os.File, size, window int64) ([]byte, error) {
	offset := size - window
	data := make([]byte, window)
	if _, err := file.ReadAt(data, offset); err != nil && err != io.EOF {
		return nil, err
	}
	if offset == 0 {
		return data, nil
	}
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 || newline+1 >= len(data) {
		return data, nil
	}
	return data[newline+1:], nil
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
