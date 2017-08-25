package main

import (
	"log"
	"os"

	"github.com/datianshi/concourse-service-adapter/adapter"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

func main() {
	stderrLogger := log.New(os.Stderr, "[concourse-service-adapter] ", log.LstdFlags)
	manifestGenerator := adapter.ManifestGenerator{
		StderrLogger: stderrLogger,
		ConfigPath:   "/var/vcap/jobs/service-adapter/config/service-adapter.conf",
	}
	binder := adapter.Binder{StderrLogger: stderrLogger}
	serviceadapter.HandleCommandLineInvocation(os.Args, manifestGenerator, binder, nil)
}
