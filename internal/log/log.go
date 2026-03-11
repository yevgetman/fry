package log

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var (
	Verbose bool

	mu      sync.Mutex
	logFile io.Writer
)

func SetLogFile(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	logFile = w
}

func Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), message)

	mu.Lock()
	defer mu.Unlock()

	_, _ = io.WriteString(os.Stdout, line)
	if logFile != nil {
		_, _ = io.WriteString(logFile, line)
	}
}

func AgentBanner(sprintNum, totalSprints int, sprintName string, iter, maxIter int, engine, model string) {
	if model == "" {
		model = "default"
	}
	banner := fmt.Sprintf(
		"▶ AGENT  sprint %d/%d \"%s\"  iter %d/%d  engine=%s  model=%s",
		sprintNum,
		totalSprints,
		sprintName,
		iter,
		maxIter,
		engine,
		model,
	)
	Log("%s", banner)
}
