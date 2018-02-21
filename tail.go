package tail

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type Tail struct {
	fileName string
	file     *os.File
	stat     os.FileInfo
	reader   *bufio.Reader
	config   *Config
}

type Config struct {
	// How frequently polling a file for a changes
	PollInterval time.Duration
	// How long to wait for a deleted file before stop polling.
	// StopPollingTimeout should be greater than PollInterval,
	// for cases when file was deleted and than created again
	StopPollingTimeout time.Duration
}

func (c *Config) Validate() error {
	switch {
	case c.PollInterval <= 0:
		return fmt.Errorf("PollInterval must be > 0")
	case c.StopPollingTimeout <= 0:
		return fmt.Errorf("StopPollingTimeout must be > 0")
	}

	return nil
}

// NewConfig return config with default values
func NewConfig() *Config {
	c := Config{
		PollInterval:       time.Second * 1,
		StopPollingTimeout: time.Second * 5,
	}

	return &c
}

func NewTail(fileName string, offset int64, config *Config) (tail Tail, err error) {
	if config == nil {
		config = NewConfig()
	}

	err = config.Validate()
	if err != nil {
		return tail, err
	}

	tail.config = config
	tail.fileName = fileName
	tail.file, err = os.Open(fileName)
	if err != nil {
		return tail, fmt.Errorf("failed to open file %s: %s", fileName, err)
	}

	tail.stat, err = tail.file.Stat()
	if err != nil {
		return tail, fmt.Errorf("failed to stat file %s: %s", fileName, err)
	}

	if offset != 0 {
		_, err = tail.file.Seek(offset, io.SeekStart)
		if err != nil {
			return tail, fmt.Errorf("failed to seek file %s: %s", fileName, err)
		}
	}

	tail.reader = bufio.NewReader(tail.file)

	return tail, nil
}

func (tail *Tail) ReadLine() (string, error) {
	var linePart string
	for {
		line, err := tail.reader.ReadString('\n')
		if err == nil {
			if linePart != "" {
				line = linePart + line
				linePart = ""
			}

			return strings.TrimRight(line, "\n"), nil
		}

		linePart = line
		changesErr := tail.waitForChanges()
		if changesErr != nil {
			return "", changesErr
		}
	}
}
func (tail *Tail) waitForChanges() error {
	var stat os.FileInfo
	var err error
	lastSuccessfulRead := time.Now()

	for {
		time.Sleep(tail.config.PollInterval)
		stat, err = os.Stat(tail.fileName)
		if err != nil {
			log.Printf("failed to stat file %s: %s", tail.fileName, err)

			if time.Since(lastSuccessfulRead) > tail.config.StopPollingTimeout {
				tail.file.Close()
				return err
			}
			continue
		}

		lastSuccessfulRead = time.Now()
		if !os.SameFile(tail.stat, stat) {
			log.Printf("file was moved %s", tail.fileName)
			tail.file.Close()
			tail.file, err = os.Open(tail.fileName)
			if err != nil {
				log.Printf("failed to open file %s: %s", tail.fileName, err)
				continue
			}
			tail.reader = bufio.NewReader(tail.file)
			break
		}
		if stat.Size() < tail.stat.Size() {
			log.Printf("file was truncated %s", tail.fileName)
			_, err = tail.file.Seek(0, io.SeekStart)
			if err != nil {
				log.Printf("failed to seek file %s: %s", tail.fileName, err)
				continue
			}
			break
		}
		if stat.Size() > tail.stat.Size() {
			break
		}
	}
	tail.stat = stat
	return nil
}

func (tail *Tail) Close() {
	if tail.file != nil {
		tail.file.Close()
	}
}

func (tail *Tail) Offset() (int64, error) {
	offset, err := tail.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return offset - int64(tail.reader.Buffered()), nil
}
