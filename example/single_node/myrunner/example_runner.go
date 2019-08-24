package myrunner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/medyagh/kic/pkg/command"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/klog"
)

func NewNodeRunner(id string) command.Runner {
	return &NodeRunner{
		Id:     id,
		client: "docker",
	}
}

// NodeRunner runs commands inside the container
// wrapper arround docker exec
type NodeRunner struct {
	Id     string // could be either container name or id.
	client string // could be docker/podman ...
}

// Run starts the specified command in a bash shell and waits for it to complete.
func (o *NodeRunner) Run(cmd string) error {
	fmt.Println("Run:", cmd)
	args := []string{
		o.client,
		"exec",
		// run with privileges so we can remount etc..
		"--privileged",
	}
	// since we have a io.Reader we need -i interactive so we can pipe the input
	args = append(args,
		"-i", //
	)

	args = append(
		args,
		o.Id, // up to here we got so far: "docker exec --privileged -i container_name"
		cmd,  // now the actual command
	)

	c := exec.Command(cmd)
	if err := c.Run(); err != nil {
		return errors.Wrapf(err, "running command: %s", cmd)
	}
	return nil
}

// CombinedOutputTo runs the command and stores both command
// output and error to out.
func (o *NodeRunner) CombinedOutputTo(cmd string, out io.Writer, inputs ...io.Reader) error {
	args := []string{
		o.client,
		"exec",
		// run with privileges so we can remount etc..
		"--privileged",
	}
	// since we have a io.Reader we need -i interactive so we can pipe the input
	args = append(args,
		"-i", //
	)

	args = append(
		args,
		o.Id, // up to here we got so far: "docker exec --privileged -i container_name"
		cmd,  // now the actual command
	)
	// if the command is hooked up to the processes's output we want a tty
	if isTerminal(out) {
		args = append(args, "-t")
	}

	klog.Infoln("Run with output:", args)

	c := exec.Command(strings.Join(args, " "))
	if inputs != nil {
		c.Stdin = inputs[0]
	}
	c.Stdout = out
	c.Stderr = out

	err := c.Run()
	if err != nil {
		return errors.Wrapf(err, "running command: %s\n.", cmd)
	}

	return nil
}

// CombinedOutput runs the command  in a bash shell and returns its
// combined standard output and standard error.
func (o *NodeRunner) CombinedOutput(cmd string, inputs ...io.Reader) (string, error) {
	var b bytes.Buffer
	err := o.CombinedOutputTo(cmd, &b, inputs...)
	if err != nil {
		inputLogMsg := ""
		if inputs != nil {
			inputLogMsg = fmt.Sprintf("inputs: %v", inputs[0])
		}
		return "", errors.Wrapf(err, "running command: %s\n output: %s %v", cmd, b.Bytes(), inputLogMsg)
	}
	return b.String(), nil

}

// isTerminal returns true if the writer w is a terminal
func isTerminal(w io.Writer) bool {
	if v, ok := (w).(*os.File); ok {
		return terminal.IsTerminal(int(v.Fd()))
	}
	return false
}
