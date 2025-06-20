package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/calpa/urusai/config"
	"github.com/calpa/urusai/crawler"
)

var (
	// Overridden at build with: -ldflags "-X main.version=$(git describe --tags --dirty)"
	version = "dev"
)

func main() {
	// ───────────────────── flags ─────────────────────
	cfgPath := flag.String("config", "", "path to JSON/YAML config file (optional)")
	logLevel := flag.String("log", "info", "log level: debug|info|warn|error")
	showVer := flag.Bool("version", false, "print version and exit")
	timeout := flag.Duration("timeout", 0, "overall run timeout (e.g. 30s, 2m). 0 = no timeout")
	flag.Parse()

	if *showVer {
		log.Printf("urusai %s", version)
		return
	}

	setLogLevel(*logLevel)

	// ─────────────────── config load ─────────────────
	var (
		cfg *config.Config
		err error
	)

	switch {
	case *cfgPath == "":
		log.Printf("INFO: %s using default config", time.Now().Format("2006/01/02 15:04:05"))
		cfg, err = config.LoadDefaultConfig()
	default:
		cfg, err = config.LoadFromFile(*cfgPath)
	}
	if err != nil {
		log.Fatalf("ERROR: could not load config: %v", err)
	}

	if *timeout > 0 {
		cfg.Timeout = int(timeout.Seconds()) // keep legacy seconds field for crawler
	}

	// ─────────────────── crawler init ────────────────
	c := crawler.NewCrawler(cfg)

	// ctx cancels on SIGINT/SIGTERM and optional timeout
	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx := baseCtx
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(baseCtx, *timeout)
		defer cancel()
	}

	log.Printf("INFO: %s starting urusai traffic generator ✈️", time.Now().Format("2006/01/02 15:04:05"))

	c.Crawl(ctx)
}

// setLogLevel tweaks the global logger to the requested verbosity.
func setLogLevel(level string) {
	const (
		RESET  = "\033[0m"
		BOLD   = "\033[1m"
		RED    = "\033[31m"
		GREEN  = "\033[32m"
		YELLOW = "\033[33m"
		BLUE   = "\033[34m"
	)
	switch strings.ToLower(level) {
	case "debug":
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.SetPrefix(BLUE + "DEBUG: " + RESET)
	case "info":
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix(GREEN + "INFO: " + RESET)
	case "warn", "warning":
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix(YELLOW + "WARNING: " + RESET)
	case "error":
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.SetPrefix(RED + "ERROR: " + RESET)
	default:
		log.SetFlags(log.Ldate | log.Ltime)
		log.SetPrefix(GREEN + "INFO: " + RESET)
	}
}
