package sse

import (
	"bufio"
	"errors"
	"io"

	"neo-code/internal/provider"
)

const (
	// DefaultMaxLineSize 定义单条 SSE 行的默认上限，防止异常长行占用过多内存。
	DefaultMaxLineSize = 256 * 1024
	// DefaultMaxStreamTotalSize 定义单个 SSE 流允许读取的默认总量上限。
	DefaultMaxStreamTotalSize = 10 << 20
)

// BoundedReader 在逐行读取 SSE 数据时执行单行与总量双重限制。
type BoundedReader struct {
	reader             *bufio.Reader
	totalRead          int64
	maxLineSize        int
	maxStreamTotalSize int64
}

// NewBoundedReader 使用默认限制创建 SSE 有界读取器。
func NewBoundedReader(r io.Reader) *BoundedReader {
	return NewBoundedReaderWithLimits(r, DefaultMaxLineSize, DefaultMaxStreamTotalSize)
}

// NewBoundedReaderWithLimits 使用自定义限制创建 SSE 有界读取器。
func NewBoundedReaderWithLimits(r io.Reader, maxLineSize int, maxStreamTotalSize int64) *BoundedReader {
	if maxLineSize <= 0 {
		maxLineSize = DefaultMaxLineSize
	}
	if maxStreamTotalSize <= 0 {
		maxStreamTotalSize = DefaultMaxStreamTotalSize
	}
	return &BoundedReader{
		reader:             bufio.NewReaderSize(r, maxLineSize+1),
		maxLineSize:        maxLineSize,
		maxStreamTotalSize: maxStreamTotalSize,
	}
}

// ReadLine 读取一行 SSE 数据并执行长度限制；返回值会去除行尾换行符。
func (r *BoundedReader) ReadLine() (string, error) {
	line, err := r.reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", provider.ErrLineTooLong
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	rawLen := len(line)
	if rawLen > 0 && line[rawLen-1] == '\n' {
		rawLen--
	}
	if rawLen > r.maxLineSize {
		return "", provider.ErrLineTooLong
	}

	r.totalRead += int64(len(line))
	if r.totalRead > r.maxStreamTotalSize {
		return "", provider.ErrStreamTooLarge
	}

	return TrimLineEnding(string(line)), err
}

// TrimLineEnding 去除字符串末尾连续的 \r 与 \n。
func TrimLineEnding(line string) string {
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	return line
}
