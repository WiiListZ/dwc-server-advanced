package logging

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// The console keeps the last lines the server printed, so the moderation
// panel can show what the terminal shows. In the frontend process it also
// receives the backend's output, so it holds the whole console.

const (
	consoleCapacity   = 5000
	consoleMaxLineLen = 4096
)

// ConsoleLine is one line of console output. Text keeps the ANSI colours.
type ConsoleLine struct {
	Seq  uint64    `json:"seq"`
	Time time.Time `json:"time"`
	Text string    `json:"text"`
}

// ConsoleSnapshot is the reply to a console read.
type ConsoleSnapshot struct {
	Lines []ConsoleLine `json:"lines"`
	// Next is the sequence number to ask for next time.
	Next uint64 `json:"next"`
	// Dropped is true when some of the requested lines are no longer kept.
	Dropped bool `json:"dropped"`
}

type consoleBuffer struct {
	mutex   sync.Mutex
	lines   []ConsoleLine
	start   int
	nextSeq uint64
}

var buffer = &consoleBuffer{nextSeq: 1}

// consoleSource is one writer into the console. Each source keeps its own
// unfinished line, so output written at the same time is not mixed up.
type consoleSource struct {
	partial []byte
}

// console is the source the logger writes to.
var console = ConsoleWriter()

// ConsoleWriter returns a new writer that feeds the console buffer.
func ConsoleWriter() io.Writer {
	return &consoleSource{}
}

func (s *consoleSource) Write(p []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	data := append(s.partial, p...)
	for {
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			break
		}
		buffer.appendLocked(data[:index])
		data = data[index+1:]
	}
	if len(data) > consoleMaxLineLen {
		buffer.appendLocked(data)
		data = nil
	}
	s.partial = append([]byte(nil), data...)
	return len(p), nil
}

func (c *consoleBuffer) appendLocked(raw []byte) {
	raw = bytes.TrimRight(raw, "\r")
	if len(raw) > consoleMaxLineLen {
		raw = raw[:consoleMaxLineLen]
	}
	line := ConsoleLine{Seq: c.nextSeq, Time: time.Now(), Text: string(raw)}
	c.nextSeq++
	if len(c.lines) < consoleCapacity {
		c.lines = append(c.lines, line)
		return
	}
	c.lines[c.start] = line
	c.start = (c.start + 1) % consoleCapacity
}

// ConsoleSince returns the lines with a sequence number of at least since,
// at most limit of them (the newest ones when more are available).
func ConsoleSince(since uint64, limit int) ConsoleSnapshot {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	snapshot := ConsoleSnapshot{Lines: []ConsoleLine{}, Next: buffer.nextSeq}
	count := len(buffer.lines)
	if count == 0 {
		return snapshot
	}
	oldest := buffer.lines[buffer.start].Seq
	if since < oldest {
		snapshot.Dropped = since > 0
		since = oldest
	}
	if since >= buffer.nextSeq {
		return snapshot
	}
	first := int(since - oldest)
	if limit > 0 && count-first > limit {
		first = count - limit
		snapshot.Dropped = true
	}
	for i := first; i < count; i++ {
		snapshot.Lines = append(snapshot.Lines, buffer.lines[(buffer.start+i)%count])
	}
	return snapshot
}
