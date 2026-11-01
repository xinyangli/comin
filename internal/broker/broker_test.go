package broker

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoggerHandlesLinesLargerThanScannerTokenLimit(t *testing.T) {
	b := New()
	b.Start()
	defer b.Stop()
	events := b.Subscribe("logger-test")
	stdout, stderr := b.GetLogger("evaluation", "generation-1")
	payload := strings.Repeat("x", 128*1024)

	writeDone := make(chan error, 1)
	go func() {
		_, err := stdout.Write([]byte(payload + "\n"))
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("stdout write blocked on a large log line")
	}
	assert.NoError(t, stdout.Close())
	assert.NoError(t, stderr.Close())

	for {
		select {
		case event := <-events:
			line := event.GetLog().GetLine()
			if line != nil && line.GetSource() == "stdout" {
				assert.Equal(t, payload, line.GetMsg())
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("large log line was not published")
		}
	}
}
