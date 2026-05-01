package ptyproxy

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestUTF8RingBufferSnapshotKeepsUTF8Boundary(t *testing.T) {
	buffer := NewUTF8RingBuffer(8)

	_, _ = buffer.Write([]byte("abc你"))
	_, _ = buffer.Write([]byte("好"))

	snapshot := buffer.SnapshotString()
	if !utf8.ValidString(snapshot) {
		t.Fatalf("snapshot is not utf8 valid: %q", snapshot)
	}
	if strings.ContainsRune(snapshot, '\uFFFD') {
		t.Fatalf("snapshot contains replacement rune: %q", snapshot)
	}
}

func TestUTF8RingBufferConcurrentWriteAndCapacity(t *testing.T) {
	buffer := NewUTF8RingBuffer(64 * 1024)

	const goroutines = 8
	const rounds = 500

	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	for index := 0; index < goroutines; index++ {
		go func() {
			defer waitGroup.Done()
			for round := 0; round < rounds; round++ {
				_, _ = buffer.Write([]byte("日志-中文-ansi-\u001b[31mred\u001b[0m\n"))
			}
		}()
	}
	waitGroup.Wait()

	snapshot := buffer.SnapshotBytes()
	if len(snapshot) > 64*1024 {
		t.Fatalf("snapshot size = %d, want <= %d", len(snapshot), 64*1024)
	}
	if !utf8.Valid(snapshot) {
		t.Fatalf("snapshot bytes are not utf8 valid")
	}
	if strings.ContainsRune(string(snapshot), '\uFFFD') {
		t.Fatalf("snapshot contains replacement rune")
	}
}

func TestNormalizeUTF8WindowStripsBrokenPrefixAndSuffix(t *testing.T) {
	// "你" = e4 bd a0，"好" = e5 a5 bd
	raw := []byte{0xBD, 0xA0, 0xE5, 0xA5}

	safe := normalizeUTF8Window(raw)
	if len(safe) != 0 {
		t.Fatalf("safe len = %d, want 0 for fully broken window", len(safe))
	}
}
