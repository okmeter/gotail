package tail

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const defaultWaitDuration = time.Second * 5

type Tail struct {
	fileName     string
	pollInterval time.Duration
	file         *os.File
	stat         os.FileInfo
	reader       *bufio.Reader
}

func NewTail(fileName string, offset int64, pollInterval time.Duration) (tail Tail, err error) {
	tail.fileName = fileName
	tail.pollInterval = pollInterval
	tail.file, err = os.Open(fileName)
	if err != nil {
		return tail, fmt.Errorf("failed to open file %s: %s", fileName, err)
	}

	tail.stat, err = tail.file.Stat()
	if err != nil {
		return tail, fmt.Errorf("failed to stat file %s: %s", fileName, err)
	}

	if offset != 0 {
		_, err = tail.file.Seek(offset, os.SEEK_SET)
		if err != nil {
			return tail, fmt.Errorf("failed to seek file %s: %s", fileName, err)
		}
	}

	tail.reader = bufio.NewReader(tail.file)

	return tail, nil
}

func (tail *Tail) ReadLine() string {
	var linePart string
	for {
		line, err := tail.reader.ReadString('\n')
		if err == nil {
			if linePart != "" {
				line = linePart + line
				linePart = ""
			}

			return strings.TrimRight(line, "\n")
		}

		linePart = line
		changesErr := tail.waitForChanges()
		if changesErr != nil {
			return ""
		}
	}
}

func (tail *Tail) waitForChanges() error {
	log.Printf("waiting for changes %s", tail.fileName)

	var done chan struct{}
	var stat os.FileInfo
	var err error

	for {
		select {
		case <-done:
			return fmt.Errorf("failed to stat file")
		default:
			time.Sleep(tail.pollInterval)
			stat, err = os.Stat(tail.fileName)
			if err != nil {
				log.Printf("failed to stat file %s: %s", tail.fileName, err)
				if done == nil {
					done = make(chan struct{})
					timer := time.NewTimer(defaultWaitDuration)

					go func(timer *time.Timer, ch chan struct{}) {
						<-timer.C
						done <- struct{}{}
					}(timer, done)
				}
				continue
			}
			if !os.SameFile(tail.stat, stat) {
				log.Printf("file was moved %s", tail.fileName)
				tail.file.Close()
				tail.file, err = os.Open(tail.fileName)
				if err != nil {
					log.Printf("failed to open file %s: %s", tail.fileName, err)
					continue
				}
				tail.reader = bufio.NewReader(tail.file)
				tail.stat = stat
				return nil
			}
			if stat.Size() < tail.stat.Size() {
				log.Printf("file was truncated %s", tail.fileName)
				_, err = tail.file.Seek(0, os.SEEK_SET)
				if err != nil {
					log.Printf("failed to seek file %s: %s", tail.fileName, err)
					continue
				}
				tail.stat = stat
				return nil
			}
			if stat.Size() > tail.stat.Size() {
				log.Printf("file was appended %s", tail.fileName)
				tail.stat = stat
				return nil
			}
		}

	}
}

func (tail *Tail) Close() {
	if tail.file != nil {
		tail.file.Close()
	}
}

func (tail *Tail) Offset() (int64, error) {
	offset, err := tail.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, err
	}
	return offset - int64(tail.reader.Buffered()), nil
}
