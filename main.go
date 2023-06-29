package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/bombsimon/logrusr/v3"
	"github.com/konveyor/analyzer-lsp/provider"
	"github.com/konveyor/analyzer-dotnet-provider/pkg/dotnet"
	"github.com/sirupsen/logrus"
)

var (
	port = flag.Int("port", 0, "Port must be set")
)

func main() {
	flag.Parse()
	logrusLog := logrus.New()
	logrusLog.SetOutput(os.Stdout)
	logrusLog.SetFormatter(&logrus.TextFormatter{})
	// need to do research on mapping in logrusr to level here TODO
	// logrusLog.SetLevel(logrus.Level(5))
	logrusLog.SetLevel(logrus.Level(10))

	log := logrusr.New(logrusLog)

	client := dotnet.NewDotnetProvider()

	if port == nil || *port == 0 {
		log.Error(fmt.Errorf("port unspecified"), "port number must be specified")
		panic(1)
	}

	s := provider.NewServer(client, *port, log)
	ctx := context.TODO()
	s.Start(ctx)
}
