// +build !windows

package cio

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/containerd/fifo"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func assertHasPrefix(t *testing.T, s, prefix string) {
	t.Helper()
	if !strings.HasPrefix(s, prefix) {
		t.Fatalf("expected %s to start with %s", s, prefix)
	}
}

func TestNewFIFOSetInDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NewFIFOSetInDir has different behaviour on windows")
	}

	root, err := ioutil.TempDir("", "test-new-fifo-set")
	assert.NilError(t, err)
	defer os.RemoveAll(root)

	fifos, err := NewFIFOSetInDir(root, "theid", true)
	assert.NilError(t, err)

	assertHasPrefix(t, fifos.Stdin, root)
	assertHasPrefix(t, fifos.Stdout, root)
	assertHasPrefix(t, fifos.Stderr, root)
	assert.Check(t, is.Equal("theid-stdin", filepath.Base(fifos.Stdin)))
	assert.Check(t, is.Equal("theid-stdout", filepath.Base(fifos.Stdout)))
	assert.Check(t, is.Equal("theid-stderr", filepath.Base(fifos.Stderr)))
	assert.Check(t, is.Equal(true, fifos.Terminal))

	files, err := ioutil.ReadDir(root)
	assert.NilError(t, err)
	assert.Check(t, is.Len(files, 1))

	assert.NilError(t, fifos.Close())
	files, err = ioutil.ReadDir(root)
	assert.NilError(t, err)
	assert.Check(t, is.Len(files, 0))
}

func TestNewAttach(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("setupFIFOProducers not yet implemented on windows")
	}
	var (
		expectedStdin  = "this is the stdin"
		expectedStdout = "this is the stdout"
		expectedStderr = "this is the stderr"
		stdin          = bytes.NewBufferString(expectedStdin)
		stdout         = new(bytes.Buffer)
		stderr         = new(bytes.Buffer)
	)

	withBytesBuffers := func(streams *Streams) {
		*streams = Streams{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	}
	attacher := NewAttach(withBytesBuffers)

	fifos, err := NewFIFOSetInDir("", "theid", false)
	assert.NilError(t, err)

	io, err := attacher(fifos)
	assert.NilError(t, err)
	defer io.Close()

	producers := setupFIFOProducers(t, io.Config())
	initProducers(t, producers, expectedStdout, expectedStderr)

	actualStdin, err := ioutil.ReadAll(producers.Stdin)
	assert.NilError(t, err)

	io.Wait()
	io.Cancel()
	assert.Check(t, is.NilError(io.Close()))

	assert.Check(t, is.Equal(expectedStdout, stdout.String()))
	assert.Check(t, is.Equal(expectedStderr, stderr.String()))
	assert.Check(t, is.Equal(expectedStdin, string(actualStdin)))
}

type producers struct {
	Stdin  io.ReadCloser
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func setupFIFOProducers(t *testing.T, fifos Config) producers {
	var (
		err   error
		pipes producers
		ctx   = context.Background()
	)

	pipes.Stdin, err = fifo.OpenFifo(ctx, fifos.Stdin, syscall.O_RDONLY, 0)
	assert.NilError(t, err)

	pipes.Stdout, err = fifo.OpenFifo(ctx, fifos.Stdout, syscall.O_WRONLY, 0)
	assert.NilError(t, err)

	pipes.Stderr, err = fifo.OpenFifo(ctx, fifos.Stderr, syscall.O_WRONLY, 0)
	assert.NilError(t, err)

	return pipes
}

func initProducers(t *testing.T, producers producers, stdout, stderr string) {
	_, err := producers.Stdout.Write([]byte(stdout))
	assert.NilError(t, err)
	assert.NilError(t, producers.Stdout.Close())

	_, err = producers.Stderr.Write([]byte(stderr))
	assert.NilError(t, err)
	assert.NilError(t, producers.Stderr.Close())
}
