// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul-esm/version"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	ExitCodeOK int = 0

	ExitCodeError = 10 + iota
)

func main() {
	// Handle parsing the CLI flags.
	var configFiles AppendSliceValue
	var isVersion bool

	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.Var(&configFiles, "config-file", "A config file to use. Can be either .hcl or .json "+
		"format. Can be specified multiple times.")
	f.Var(&configFiles, "config-dir", "A directory to look for .hcl or .json config files in. "+
		"Can be specified multiple times.")
	f.BoolVar(&isVersion, "v", false, "")
	f.BoolVar(&isVersion, "version", false, "Print the version of this daemon.")

	f.Usage = func() {
		f.SetOutput(os.Stdout)
		fmt.Print(usage, "\n")
		f.PrintDefaults()
	}

	err := f.Parse(os.Args[1:])
	if err != nil {
		if err != flag.ErrHelp {
			fmt.Printf("error parsing flags: %v", err)
		}
		os.Exit(ExitCodeError)
	}

	if isVersion {
		fmt.Printf("%s\n", version.GetHumanVersion())
		os.Exit(ExitCodeOK)
	}

	// Build the config.
	config, err := BuildConfig([]string(configFiles))
	if err != nil {
		fmt.Println(err)
		os.Exit(ExitCodeError)
	}

	// Set up logging.
	logConfig := logging.Config{
		Name:              "consul-esm",
		LogLevel:          config.LogLevel,
		EnableSyslog:      config.EnableSyslog,
		SyslogFacility:    config.SyslogFacility,
		LogJSON:           config.LogJSON,
		LogFilePath:       config.LogFile,
		LogRotateBytes:    config.LogRotateBytes,
		LogRotateMaxFiles: config.LogRotateMaxFiles,
		LogRotateDuration: config.LogRotateDuration,
	}
	logger, err := logging.Setup(logConfig, os.Stdout)
	if err != nil {
		os.Exit(ExitCodeError)
	}
	agent, err := NewAgent(config, logger)
	if err != nil {
		panic(err)
	}

	// Consul compatibility is only verified at startup. If new Consul servers
	// join later with incompatible versions, inconsistent results may occur with
	// updating health checks for external services.
	err = agent.VerifyConsulCompatibility()
	if err != nil {
		fmt.Println(err)
		os.Exit(ExitCodeError)
	}

	// Set up shutdown and signal handling.
	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh)
	go handleSignals(agent.logger, signalCh, agent)

	ui := cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}
	if config.LogJSON {
		ui.Writer = io.Discard
	}
	ui.Output("Consul ESM running!")
	if config.Datacenter == "" {
		ui.Info(fmt.Sprintf("            Datacenter: (default)"))
	} else {
		ui.Info(fmt.Sprintf("            Datacenter: %q", config.Datacenter))
	}
	ui.Info(fmt.Sprintf("             Partition: %q", config.Partition))
	ui.Info(fmt.Sprintf("               Service: %q", config.Service))
	ui.Info(fmt.Sprintf("           Service Tag: %q", config.Tag))
	ui.Info(fmt.Sprintf("            Service ID: %q", agent.serviceID()))
	ui.Info(fmt.Sprintf("Node Reconnect Timeout: %q", config.NodeReconnectTimeout.String()))
	ui.Info(fmt.Sprintf("   Disable coordinates: %t", config.DisableCoordinateUpdates))
	ui.Info(fmt.Sprintf("        Statsd address: %q", config.Telemetry.StatsdAddr))
	ui.Info(fmt.Sprintf("         Metrix prefix: %q", config.Telemetry.MetricsPrefix))
	ui.Info("")
	ui.Output("Log data will now stream in as it occurs:\n")

	// Run the agent!
	if err := agent.Run(); err != nil {
		ui.Error(err.Error())
		os.Exit(ExitCodeError)
	}

	os.Exit(ExitCodeOK)
}

func handleSignals(logger hclog.Logger, signalCh chan os.Signal, agent *Agent) {
	for sig := range signalCh {
		logger.Debug("Caught signal:" + sig.String())
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("Shutting down...")
			agent.Shutdown()
		default:
		}
	}
}

const usage = `
Usage: consul-esm [options]

  A config file is optional, and can be either HCL or JSON format.
`

// AppendSliceValue implements the flag.Value interface and allows multiple
// calls to the same variable to append a list.
type AppendSliceValue []string

func (s *AppendSliceValue) String() string {
	return strings.Join(*s, ",")
}

func (s *AppendSliceValue) Set(value string) error {
	if *s == nil {
		*s = make([]string, 0, 1)
	}

	*s = append(*s, value)
	return nil
}
