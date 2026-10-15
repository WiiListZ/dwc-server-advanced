package logging

import (
	"fmt"
	"testing"
)

func resetConsole() {
	buffer.mutex.Lock()
	buffer.lines, buffer.start, buffer.nextSeq = nil, 0, 1
	buffer.mutex.Unlock()
}

func TestConsoleKeepsSourcesApart(t *testing.T) {
	resetConsole()
	a, b := ConsoleWriter(), ConsoleWriter()
	_, _ = a.Write([]byte("hello "))
	_, _ = b.Write([]byte("other line\n"))
	_, _ = a.Write([]byte("world\r\nsecond\n"))

	snapshot := ConsoleSince(0, 0)
	got := []string{}
	for _, line := range snapshot.Lines {
		got = append(got, line.Text)
	}
	if fmt.Sprint(got) != "[other line hello world second]" || snapshot.Next != 4 {
		t.Fatalf("lines = %q, next = %d", got, snapshot.Next)
	}
	if more := ConsoleSince(snapshot.Next, 0); len(more.Lines) != 0 {
		t.Fatalf("unexpected new lines: %+v", more.Lines)
	}
}

func TestConsoleRingAndLimit(t *testing.T) {
	resetConsole()
	w := ConsoleWriter()
	for i := 1; i <= consoleCapacity+10; i++ {
		_, _ = fmt.Fprintf(w, "line %d\n", i)
	}

	all := ConsoleSince(1, 0)
	if len(all.Lines) != consoleCapacity || !all.Dropped || all.Lines[0].Text != "line 11" {
		t.Fatalf("got %d lines, dropped %v, first %q", len(all.Lines), all.Dropped, all.Lines[0].Text)
	}
	last := ConsoleSince(0, 3)
	if len(last.Lines) != 3 || last.Lines[2].Text != fmt.Sprintf("line %d", consoleCapacity+10) {
		t.Fatalf("limit: %+v", last.Lines)
	}
	tail := ConsoleSince(uint64(consoleCapacity+9), 0)
	if len(tail.Lines) != 2 || tail.Dropped {
		t.Fatalf("tail: %+v", tail)
	}
}
