package command

import "io"

// Runner represents an interface to run commands.
type Runner interface {
	// Run starts the specified command and waits for it to complete.
	Run(cmd string) error

	// CombinedOutputTo runs the command and stores both command
	// output and error to out. A typical usage is:
	//
	//          var b bytes.Buffer
	//          CombinedOutput(cmd, &b)
	//          fmt.Println(b.Bytes())
	//
	// Or, you can set out to os.Stdout, the command output and
	// error would show on your terminal immediately before you
	// cmd exit. This is useful for a long run command such as
	// continuously print running logs.
	// also inputs is the used for piping into the command.
	CombinedOutputTo(cmd string, out io.Writer, inputs ...io.Reader) error


	
	// CombinedOutput runs the command and returns its combined standard
	// output and standard error.
	CombinedOutput(cmd string,  inputs ...io.Reader) (string, error)
}
