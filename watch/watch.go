package watch

import (
	"strings"
	"time"

	"github.com/edma2/pantsindex/analysis"
	"github.com/go-fsnotify/fsevents"
)

type SourceChange struct {
	Class string
	Path  string
}

func PathChanges(path string, stop chan bool) chan string {
	es := &fsevents.EventStream{
		Paths:   []string{path},
		Latency: 500 * time.Millisecond,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot}
	es.Start()
	changes := make(chan string)
	go func() {
		for {
			select {
			case <-stop:
				es.Stop()
				close(changes)
			case events := <-es.Events:
				for _, e := range events {
					changes <- e.Path
				}
			}
		}
	}()
	return changes
}

func AnalysisFileChanges(pathChanges chan string) chan string {
	changes := make(chan string)
	go func() {
		for path := range pathChanges {
			if strings.HasSuffix(path, ".analysis") {
				changes <- path
			}
		}
	}()
	return changes
}

func AnalysisChanges(analysisFileChanges chan string) chan *SourceChange {
	changes := make(chan *SourceChange)
	go func() {
		for path := range analysisFileChanges {
			analysis.ReadAnalysisFile(path, func(class, path string) {
				changes <- &SourceChange{Class: class, Path: path}
			})
		}
	}()
	return changes
}