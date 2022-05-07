package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Leantar/fimagent/agent"
	"github.com/Leantar/fimagent/modules/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath = flag.String("config", "config.yaml", "Specify a path to load the config from")
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Parse command line arguments
	flag.Parse()

	var conf agent.Config
	err := config.FromYamlFile(*configPath, &conf)
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("failed to read config")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	a := agent.New(conf)

	err = a.Connect()
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("failed to connect to server ")
	}

	go func() {
		err := a.Run()
		if err != nil {
			log.Fatal().Caller().Err(err).Msg("failed to run agent")
		}
	}()

	<-quit

	err = a.Stop()
	if err != nil {
		log.Fatal().Caller().Err(err).Msg("failed to stop agent")
	}
}
