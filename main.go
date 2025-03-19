package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/calpa/urusai/config"
	"github.com/calpa/urusai/crawler"
)

func main() {
	// Parse command line arguments
	var configFile string
	var logLevel string
	var timeout int

	flag.StringVar(&configFile, "config", "", "path to config file")
	flag.StringVar(&logLevel, "log", "info", "logging level (debug, info, warn, error)")
	flag.IntVar(&timeout, "timeout", 0, "for how long the crawler should be running, in seconds (0 means no timeout)")
	flag.Parse()

	// Set up logging
	setLogLevel(logLevel)

	// Load configuration
	var cfg *config.Config
	var err error

	if configFile == "" {
		// Use default configuration if no config file is specified
		log.Println("No config file specified, using default configuration")
		cfg, err = config.LoadDefaultConfig()
		if err != nil {
			log.Fatalf("Failed to load default config: %v", err)
		}
	} else {
		// Load configuration from file
		cfg, err = config.LoadFromFile(configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Set timeout if provided
	if timeout > 0 {
		cfg.Timeout = timeout
	}

	// Create and start crawler
	c := crawler.NewCrawler(cfg)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, exiting gracefully...")
		os.Exit(0)
	}()

	// Start crawling
	log.Println("Starting urusai - HTTP/DNS traffic noise generator")
	c.Crawl()
}

func setLogLevel(level string) {
	switch level {
	case "debug":
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.SetPrefix("DEBUG: ")
	case "info":
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix("INFO: ")
	case "warn":
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix("WARNING: ")
	case "error":
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.SetPrefix("ERROR: ")
	default:
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix("INFO: ")
	}
}
