package log

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/arschles/assert"
)

var (
	world = "world"
)

func getWriters() (io.Writer, *bytes.Buffer, io.Writer, *bytes.Buffer) {
	var out, err bytes.Buffer
	stdout, stderr := io.MultiWriter(os.Stdout, &out), io.MultiWriter(os.Stderr, &err)
	return stdout, &out, stderr, &err
}

func TestMsg(t *testing.T) {
	stdout, _, stderr, _ := getWriters()
	lg := NewLogger(stdout, stderr, true)
	lg.Msg("hello %s", world)
}

func TestErr(t *testing.T) {
	stdout, _, stderr, _ := getWriters()
	lg := NewLogger(stdout, stderr, false)
	lg.Err("hello %s", world)
}

func TestInfo(t *testing.T) {
	stdout, out, stderr, err := getWriters()
	lg := NewLogger(stdout, stderr, false)
	lg.Info("hello %s", world)
	assert.Equal(t, string(err.Bytes()), addColor(InfoPrefix+" ", Green), "stderr output")
	assert.Equal(t, string(out.Bytes()), "hello world\n", "stdout output")
}

func TestDebug(t *testing.T) {
	stdout, out, stderr, err := getWriters()
	lgOn := NewLogger(stdout, stderr, true)
	lgOff := NewLogger(stdout, stderr, false)
	lgOff.Debug("hello %s", world)
	assert.Equal(t, out.Len(), 0, "stdout buffer length")
	assert.Equal(t, err.Len(), 0, "stderr buffer length")
	lgOn.Debug("hello %s", world)
	assert.Equal(t, string(err.Bytes()), addColor(DebugPrefix+" ", Cyan), "stderr output")
	assert.Equal(t, string(out.Bytes()), "hello world\n", "stdout output")
}

func TestWarn(t *testing.T) {
	stdout, out, stderr, err := getWriters()
	lg := NewLogger(stdout, stderr, false)
	lg.Warn("hello %s", world)
	assert.Equal(t, string(err.Bytes()), addColor(WarnPrefix+" ", Yellow), "stderr output")
	assert.Equal(t, string(out.Bytes()), "hello world\n", "stdout output")
}

func TestAppendNewLine(t *testing.T) {
	str := "abc"
	newStr := appendNewLine(str)
	assert.Equal(t, newStr, str+"\n", "new string")
}
