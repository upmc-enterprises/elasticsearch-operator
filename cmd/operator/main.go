package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/upmc-enterprises/elasticsearch-operator/version"
)

var (
	namespace string

	printVersion bool
)

func init() {
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.Parse()

	namespace = os.Getenv("MY_POD_NAMESPACE")
	if len(namespace) == 0 {
		namespace = "default"
	}
}

func main() {
	if printVersion {
		fmt.Println("elasticsearch-operator", version.Version)
		os.Exit(0)
	}
}
