package utils

import (
	goflag "flag"

	"github.com/spf13/pflag"
)

func InitFlags() {

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	pflag.Parse()
}
