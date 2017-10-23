package main

import (
	"container-tty/tty"
	"container-tty/utils"
	"flag"
	"fmt"

	"os"

	"github.com/spf13/pflag"
)

func main() {
	ttyOptions := tty.NewOptions()
	ttyOptions.AddFlag(pflag.CommandLine)
	utils.InitFlags()
	flag.CommandLine.Parse([]string{})

	server, err := tty.New(ttyOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "New tty server error %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "Start Web TTY Server\n")
	server.Run()
}
