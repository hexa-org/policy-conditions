// This code based on contributions from https://github.com/i2-open/i2goSignals with permission
package main

import (
	"fmt"
	"io"

	"log"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/chzyer/readline"
	"github.com/hexa-org/policy-mapper/sdk"
)

type ParserData struct {
	parser *kong.Kong
	cli    *CLI
}

type Globals struct {
	Config       string     `help:"Location of client config files" env:"HEXA_HOME" type:"path"`
	Data         ConfigData `kong:"-"`
	ConfigFile   string     `kong:"-"`
	Output       string     `short:"o" help:"To redirect output to a file" type:"path" `
	AppendOutput bool       `short:"a" default:"false" help:"When true, output to file (--output) will be appended"`
}

type CLI struct {
	Globals

	Add       AddCmd       `cmd:"" help:"Add a new integration"`
	Delete    DeleteCmd    `cmd:"" help:"Delete an integration or policy application point from local configuration"`
	Get       GetCmd       `cmd:"" help:"Retrieve or update information and display"`
	Map       MapCmd       `cmd:"" help:"Convert syntactical policies to and from IDQL"`
	Reconcile ReconcileCmd `cmd:"" help:"Reconcile compares a source set of policies another source (file or alias) of policies to determine differences."`
	Set       SetCmd       `cmd:"" help:"Set or update policies (e.g. set policies -file=idql.json)"`
	Show      ShowCmd      `cmd:"" help:"Show locally stored information about integrations and applications"`
	Exit      ExitCmd      `cmd:"" help:"Exit Hexa console"`
	Help      HelpCmd      `cmd:"" help:"Show help on a command"`
}

/*
Add      AddCmd      `cmd:"" help:"Define a new server to be managed"`
Create   CreateCmd   `cmd:"" help:"Create an issuer KEY, or STREAM"`
Delete   DeleteCmd   `cmd:"" help:"Delete a stream"`
Select   SelectCmd   `cmd:"" help:"Select a defined server or stream/server to perform operations against"`
Get      GetCmd      `cmd:"" help:"Get information from SSF servers"`
Generate GenerateCmd `cmd:"" help:"Generate an event for testing"`
Poll     PollCmd     `cmd:"" help:"Activate a polling client stream with a server identified by <alias>."`
Set      SetCmd      `cmd:"" help:"Set configuration items on server"`
Show     ShowCmd     `cmd:"" help:"Show locally configured information"`

*/

type OutputWriter struct {
	output  *os.File
	isReady bool
	err     error
}

/*
GetOutputWriter returns an output writer if one was requested or nil.  If one was requested and the output
cannot be opened an error is returned.
*/
func (cli *CLI) GetOutputWriter() *OutputWriter {
	if cli.Output == "" {
		return &OutputWriter{
			isReady: false,
		}
	}

	if cli.AppendOutput {
		file, err := os.OpenFile(cli.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println(err.Error())
			return &OutputWriter{
				isReady: false,
				err:     err,
			}
		}
		return &OutputWriter{
			output:  file,
			isReady: true,
		}
	}

	file, err := os.OpenFile(cli.Output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err.Error())
		return &OutputWriter{
			isReady: false,
			err:     err,
		}
	}
	return &OutputWriter{
		output:  file,
		isReady: true,
	}

}

func (o *OutputWriter) GetOutput() io.Writer {
	if o.isReady {
		return o.output
	}
	return nil
}

func (o *OutputWriter) WriteString(msg string, andClose bool) {

	if msg != "" && o.isReady {
		_, _ = o.output.WriteString(msg)
		_ = o.output.Sync()
	}
	if andClose {
		o.Close()
	}
}

func (o *OutputWriter) WriteBytes(msgBytes []byte, andClose bool) {
	if len(msgBytes) != 0 && o.isReady {
		_, _ = o.output.Write(msgBytes)
		_ = o.output.Sync()
	}
	if andClose {
		o.Close()
	}
}

func (o *OutputWriter) Close() {
	if o.isReady {
		_ = o.output.Sync()
		o.isReady = false
		_ = o.output.Close()
	}

}

func initParser(cli *CLI) (*ParserData, error) {
	if cli == nil {
		cli = &CLI{}
	}

	cli.Data = ConfigData{
		Integrations: map[string]*sdk.Integration{},
	}
	parser, err := kong.New(cli,
		kong.Name("hexa"),
		kong.Description("Hexa IDQL Orchestrator administration tool"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:      true,
			Summary:      true,
			Tree:         true,
			NoAppSummary: false,
		}),
		kong.UsageOnError(),
		kong.Writers(os.Stdout, os.Stdout),

		kong.NoDefaultHelp(),
		kong.Bind(&cli.Globals),
		kong.Exit(func(int) {}),
	)
	td := ParserData{
		parser: parser,
		cli:    cli,
	}

	return &td, err
}

func main() {

	console, err := readline.NewEx(&readline.Config{
		Prompt: "hexa> ",
		// HistoryFile:            os.TempDir() + "/goSignals-history",
		DisableAutoSaveHistory: true,
	})
	if err != nil {
		panic(err)
	}
	defer func(console *readline.Instance) {
		_ = console.Close()
	}(console)

	td, err := initParser(&CLI{})

	// ctx.FatalIfErrorf(err)
	if err != nil {
		fmt.Println(err.Error())
	}
	oneCommand := false
	initialArgs := os.Args
	if len(initialArgs) > 1 {
		initialArgs = initialArgs[1:]
		firstArg := initialArgs[0]
		if firstArg[0:1] != "-" {
			oneCommand = true
		} else {
			for i, arg := range initialArgs {
				prefix := arg[0:8]
				if strings.EqualFold("--config", prefix) {
					filepath := arg[strings.Index(arg, "=")+1:]
					td.cli.Globals.Config = filepath
					initialArgs = append(initialArgs[0:i], initialArgs[i+1:]...)
				}
			}
		}

	}

	// fmt.Println("Loading existing configuration...")
	err = td.cli.Data.checkConfigPath(&td.cli.Globals)
	if err != nil {
		fmt.Println("Error reading config directory: " + err.Error())
		panic(-1)
	}
	_ = td.cli.Data.Load(&td.cli.Globals)

	for true {
		var args []string
		if oneCommand {
			args = initialArgs
			fullCommand := initialArgs[0]
			for i, arg := range initialArgs {
				if i > 0 {
					fullCommand = fullCommand + " " + arg
				}
			}
			initialArgs = []string{}
			_ = console.SaveHistory(fullCommand)
		} else {
			line, err := console.Readline()
			if err != nil {
				panic(err)
			}
			//line = line[0 : len(line)-1]
			_ = console.SaveHistory(line)
			args = strings.Split(line, " ")
		}

		var ctx *kong.Context
		ctx, err = td.parser.Parse(args)
		// ctx.Bind(&cli.Globals)
		// ctx.Bind(args)

		if err != nil {
			// Put out the help text response
			td.parser.Errorf("%s", err.Error())
			if err, ok := err.(*kong.ParseError); ok {
				log.Println(err.Error())
				_ = err.Context.PrintUsage(false)
			}
			continue
		}

		err = ctx.Run(&td.cli.Globals)

		if err != nil {
			td.parser.Errorf("%s", err)
			continue
		}
		if oneCommand {
			return
		}
	}

}
