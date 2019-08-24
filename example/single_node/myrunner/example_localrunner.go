package myrunner

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/medyagh/kic/pkg/command"
	"github.com/pkg/errors"
	"k8s.io/klog"
)

func NewLocalRunner(client string) command.Runner {
	return &LocalRunner{
		client: client,
	}
}

// LocalRunner runs commands using the os/exec package.
type LocalRunner struct {
	client string
}

// Run starts the specified command in a bash shell and waits for it to complete.
func (l *LocalRunner) Run(cmd string) error {
	klog.Infoln("Run:", cmd)
	c := exec.Command("/usr/local/bin/docker" + " " + cmd)
	if err := c.Run(); err != nil {
		return errors.Wrapf(err, "Run: %s", cmd)
	}
	return nil
}

// CombinedOutputTo runs the command and stores both command
// output and error to out.
func (l *LocalRunner) CombinedOutputTo(cmd string, out io.Writer, inputs ...io.Reader) error {

	klog.Infoln("Run with output:", cmd)
	c := exec.Command("/usr/local/bin/docker" + " " + cmd)
	if inputs != nil {
		c.Stdin = inputs[0]
	}
	c.Stdout = out
	c.Stderr = out
	err := c.Run()
	if err != nil {
		return errors.Wrapf(err, "CombinedOutputTo: %s", cmd)
	}

	return nil
}

// CombinedOutput runs the command  in a bash shell and returns its
// combined standard output and standard error.
func (e *LocalRunner) CombinedOutput(cmd string, inputs ...io.Reader) (string, error) {
	var b bytes.Buffer
	err := e.CombinedOutputTo(cmd, &b, inputs...)
	if err != nil {
		return "", errors.Wrapf(err, "CombinedOutput: %s\n output: %s", cmd, b.Bytes())
	}
	return b.String(), nil

}
