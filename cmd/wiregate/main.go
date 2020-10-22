package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

const wgVersion = "0.9.2"

// TODO: keep all IPs as net.IP structs instead of strings

func setupLogging(debug bool) {
	if debug {
		fmt.Printf("Enabling debug-level logging\n")
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}

func printHelp() {
	fmt.Println("WireGate sets up WireGuard VPNs on LANs easily")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("wiregate [command]")
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("server\tStart as WireGate server")
	fmt.Println("client\tStart as client")
	fmt.Println("version\tPrint version")
	fmt.Println("help\tPrint this text")
	fmt.Println("")
	fmt.Println("Use 'wiregate [command] --help' for more information about a command")
}


func main() {
	var server = flag.NewFlagSet("server", flag.ExitOnError)
	var iface = server.String("interface", "", "REQUIRED: Network interface to use")
	var wgIface = server.String("wg-interface", "wg0", "Name for WireGuard interface")
	var wgPort = server.Int("wg-port", 51820, "WireGuard port")
	var wgCIDR = server.String("wg-cidr", "10.24.1.1/24", "IPv4 CIDR subnet for WireGuard VPN. The WireGuard interface will use the first subnet address")
	var mdnsServiceDesc = server.String("http-service-description", "Wiregate", "MDNS WireGate HTTP Control description")
	var httpPort = server.Int("http-port", 38490, "WireGate HTTP Control port")
	var vpnPassword = server.String("vpn-password", "", "REQUIRED: Password to register with the WireGate VPN")
	var purgeInterval = server.Int("purge-interval", 10, "Interval to purge unresponsive clients")
	var serverDebug = server.Bool("debug", false, "Turn on debug-level logging")

	var client = flag.NewFlagSet("client", flag.ExitOnError)
	var clientDebug = client.Bool("debug", false, "Turn on debug-level logging")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		if err := server.Parse(os.Args[2:]); err == nil {
			setupLogging(*serverDebug)

			if *iface == "" {
				fmt.Printf("Missing '-interface' argument!")
				os.Exit(1)
			}
			if *vpnPassword == "" {
				// TODO: check for weak password; generate share-able password if empty.
				fmt.Printf("Missing '-vpn-password' argment!")
				os.Exit(1)
			}
			conf := &ServerConfig{
				iface:           *iface,
				wgIface:         *wgIface,
				wgPort:          *wgPort,
				wgCIDR:          *wgCIDR,
				mdnsServiceDesc: *mdnsServiceDesc,
				httpPort:        *httpPort,
				vpnPassword:     *vpnPassword,
				purgeInterval:   *purgeInterval,
			}
			server_main(conf)
		}
	case "client":
		if err := client.Parse(os.Args[2:]); err == nil {
			setupLogging(*clientDebug)
			client_main()
		}
	case "version":
		fmt.Printf("WireGate %s\n", wgVersion)
	case "help":
		printHelp()
	default:
		fmt.Printf("Unknown subcommand: %s", os.Args[1])
		os.Exit(1)
	}
}
