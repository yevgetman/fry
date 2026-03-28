package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// TailEvents follows events.jsonl and sends parsed events to the returned
// channel. It reads any existing events first, then polls for new ones.
// Blocks until ctx is canceled. Close the returned channel by canceling ctx.
func TailEvents(ctx context.Context, projectDir string) (<-chan BuildEvent, error) {
	eventsPath := filepath.Join(projectDir, config.ObserverEventsFile)
	ch := make(chan BuildEvent, 64)

	go func() {
		defer close(ch)

		var offset int64

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			f, err := os.Open(eventsPath)
			if err != nil {
				// File doesn't exist yet or other error -- wait and retry
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
					continue
				}
			}

			// Seek to where we left off
			if offset > 0 {
				if _, err := f.Seek(offset, 0); err != nil {
					f.Close()
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
						continue
					}
				}
			}

			// Read line-by-line using bufio.Reader to track exact byte offsets.
			// bufio.Scanner reads ahead, which makes f.Seek(0,1) unreliable.
			reader := bufio.NewReader(f)
			for {
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					offset += int64(len(line))
					trimmed := bytes.TrimSpace(line)
					if len(trimmed) == 0 {
						continue
					}
					var evt BuildEvent
					if jsonErr := json.Unmarshal(trimmed, &evt); jsonErr != nil {
						continue // skip malformed lines
					}
					select {
					case ch <- evt:
					case <-ctx.Done():
						f.Close()
						return
					}
				}
				if err != nil {
					break // EOF or read error
				}
			}

			f.Close()

			// Poll interval
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()

	return ch, nil
}

// ReadAllEvents reads all events from the events file and returns them.
// Returns nil if the file does not exist.
func ReadAllEvents(projectDir string) ([]BuildEvent, error) {
	eventsPath := filepath.Join(projectDir, config.ObserverEventsFile)
	f, err := os.Open(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read events: %w", err)
	}
	defer f.Close()

	var events []BuildEvent
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				var evt BuildEvent
				if jsonErr := json.Unmarshal(trimmed, &evt); jsonErr == nil {
					events = append(events, evt)
				}
			}
		}
		if err != nil {
			break // EOF or read error
		}
	}
	return events, nil
}
