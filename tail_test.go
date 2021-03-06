package tail

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"io/ioutil"

	"os"
	"testing"
	"time"
)

func TestTail(t *testing.T) {
	f, err := ioutil.TempFile("/tmp", "log")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	cfg := NewConfig()
	cfg.PollInterval = time.Millisecond * 500

	tail, err := NewTail(f.Name(), 0, cfg)
	assert.NoError(t, err)
	defer tail.Close()

	lines := make(chan string, 10)
	errors := make(chan error)
	go func() {
		for {
			line, err := tail.ReadLine()
			if err != nil {
				errors <- err
			}
			lines <- line
		}
	}()

	f.WriteString("foo\n")
	assert.Equal(t, "foo", <-lines)

	// append
	f.WriteString("bar\nbuz\n")
	assert.Equal(t, "bar", <-lines)
	assert.Equal(t, "buz", <-lines)

	// move rotation
	err = os.Rename(f.Name(), f.Name()+".1")
	assert.NoError(t, err)
	defer os.Remove(f.Name() + ".1")
	f, err = os.Create(f.Name())
	assert.NoError(t, err)
	f.WriteString("foo1\nbar1\n")
	assert.Equal(t, "foo1", <-lines)
	assert.Equal(t, "bar1", <-lines)

	// truncate rotation
	f, err = os.OpenFile(f.Name(), os.O_WRONLY|os.O_TRUNC, 0)
	assert.NoError(t, err)
	f.WriteString("buz1\n")
	assert.Equal(t, "buz1", <-lines)

	// delete
	err = os.Remove(f.Name())
	assert.NoError(t, err)
	time.Sleep(3 * tail.config.PollInterval)
	f, err = os.Create(f.Name())
	assert.NoError(t, err)
	f.WriteString("foo2\n")
	assert.Equal(t, "foo2", <-lines)

	// no eol
	f.WriteString("bar2\nbu")
	time.Sleep(3 * tail.config.PollInterval)
	f.WriteString("z2\n")
	assert.Equal(t, "bar2", <-lines)
	assert.Equal(t, "buz2", <-lines)

	//delete permanently
	os.Remove(f.Name())

	err = <-errors
	assert.Error(t, err)
}

func TestConfigValidates(t *testing.T) {
	config := NewConfig()
	err := config.Validate()
	require.NoError(t, err)

	emptyConfig := Config{}
	err = emptyConfig.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PollInterval must be")
}
