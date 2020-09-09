package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

const wgVersion = "0.1"

// TODO: help subcommand
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
	var serverVersion = server.Bool("version", false, "Print version information")
	var serverDebug = server.Bool("debug", false, "Turn on debug-level logging")

	// TODO clean up client flags
	var client = flag.NewFlagSet("client", flag.ExitOnError)
	var clientVersion = client.Bool("version", false, "Print version information")
	var clientDebug = client.Bool("debug", false, "Turn on debug-level logging")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'server' or 'client' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		if err := server.Parse(os.Args[2:]); err == nil {
			if *serverVersion {
				fmt.Printf("WireGate v%s\n", wgVersion)
				os.Exit(0)
			}
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
			if *clientVersion {
				fmt.Printf("WireGate v%s\n", wgVersion)
				os.Exit(0)
			}
			setupLogging(*clientDebug)
			client_main()
		}
	default:
		fmt.Printf("Unknown subcommand '%s'\n", os.Args[1])
		os.Exit(1)
	}
}
