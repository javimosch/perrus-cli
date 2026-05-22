package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const Version = "1.0.0"
const defaultConfigPath = "config.yaml"
const defaultPort = 8080

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	case "check":
		cmdCheck()
	case "results":
		cmdResults()
	case "config-init":
		cmdConfigInit()
	case "config-validate":
		cmdConfigValidate()
	case "version":
		fmt.Printf("perrus-cli v%s\n", Version)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func cmdStart() {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	port := fs.Int("port", defaultPort, "HTTP server port")
	configPath := fs.String("config", defaultConfigPath, "Config file path")
	daemon := fs.Bool("daemon", false, "Run as background daemon")
	fs.Parse(os.Args[2:])

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if *daemon {
		startDaemon(*port, *configPath)
		return
	}
	runServer(*port, cfg)
}

func cmdStop() {
	stopDaemon()
}

func cmdStatus() {
	checkDaemonStatus()
}

func cmdCheck() {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "Config file path")
	asJSON := fs.Bool("json", false, "Output as JSON")
	fs.Parse(os.Args[2:])

	name := fs.Arg(0)
	if name == "" {
		fmt.Fprintln(os.Stderr, "usage: perrus-cli check <endpoint-name> [-config path] [-json]")
		os.Exit(1)
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	var target *Endpoint
	for i := range cfg.Endpoints {
		if cfg.Endpoints[i].Name == name {
			target = &cfg.Endpoints[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintf(os.Stderr, "endpoint not found: %s\n", name)
		os.Exit(1)
	}

	res := checkEndpoint(target)
	if *asJSON {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
	} else {
		printResult(res)
	}
	if !res.Success {
		os.Exit(2)
	}
}

func cmdResults() {
	fs := flag.NewFlagSet("results", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "Config file path")
	format := fs.String("format", "table", "Output format: table|json")
	fs.Parse(os.Args[2:])

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	results := checkAll(cfg)
	if *format == "json" {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return
	}
	for _, res := range results {
		printResult(res)
	}
}

func cmdConfigInit() {
	fs := flag.NewFlagSet("config-init", flag.ExitOnError)
	out := fs.String("out", defaultConfigPath, "Output path")
	fs.Parse(os.Args[2:])

	if err := os.WriteFile(*out, []byte(exampleConfig()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("config written to %s\n", *out)
}

func cmdConfigValidate() {
	fs := flag.NewFlagSet("config-validate", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "Config file path")
	fs.Parse(os.Args[2:])

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("config OK — %d endpoints\n", len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		fmt.Printf("  • %s (%s) every %s\n", ep.Name, ep.URL, ep.Interval)
	}
}

func printResult(res *Result) {
	status := "PASS"
	if !res.Success {
		status = "FAIL"
	}
	fmt.Printf("[%s] %s — %dms\n", status, res.EndpointName, res.Duration)
	for _, cc := range res.Conditions {
		mark := "✓"
		if !cc.Passed {
			mark = "✗"
		}
		fmt.Printf("  %s %s\n", mark, cc.Condition)
	}
	for _, e := range res.Errors {
		fmt.Printf("  ! %s\n", e)
	}
}

func printHelp() {
	fmt.Print(`perrus-cli — developer-oriented health dashboard and uptime monitor

Usage:
  perrus-cli <command> [options]

Commands:
  start              Start the monitoring daemon and web dashboard
  stop               Stop the background daemon
  status             Show daemon status
  check <name>       Run a one-shot check on a named endpoint
  results            Run all checks and print results immediately
  config-init        Write an example config.yaml
  config-validate    Validate a config file
  version            Show version
  help               Show this help

Start Options:
  -port int          HTTP server port (default 8080)
  -config string     Config file path (default config.yaml)
  -daemon            Run as background daemon

Check Options:
  -config string     Config file path (default config.yaml)
  -json              Output result as JSON (exits 2 if check fails)

Results Options:
  -config string     Config file path (default config.yaml)
  -format string     Output format: table|json (default table)

Config-Init Options:
  -out string        Output path (default config.yaml)

Config-Validate Options:
  -config string     Config file path (default config.yaml)

Examples:
  perrus-cli config-init
  perrus-cli config-validate
  perrus-cli start
  perrus-cli start -port 9090 -config /etc/perrus/config.yaml
  perrus-cli start -daemon
  perrus-cli check my-api -json
  perrus-cli results -format json
  perrus-cli stop
  perrus-cli status
`)
}
