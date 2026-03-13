package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
)

func main() {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("resolve example source path: runtime caller unavailable")
	}
	root := filepath.Dir(currentFile)
	root, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("resolve example root: %v", err)
	}

	report, err := runDemo(context.Background(), demoConfig{
		ProjectRoot: root,
		SessionID:   "govm-example-session",
	})
	if err != nil {
		log.Fatalf("run govm demo: %v", err)
	}
	fmt.Print(report.Render())
}
