package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/ideasynthesis/mdns"
	wg "github.com/sirmackk/wiregate"
)

// TODO refactor this prototype, goal is to dedupe and share more code

type WireGateService struct {
	Host         string
	Addr         string
	Port         int
	HTTPEndpoint string
	Description  string
}

func WGServiceFromServiceEntry(entry *mdns.ServiceEntry) *WireGateService {
	description := strings.Join(entry.InfoFields, ", ")
	httpEndpoint := fmt.Sprintf("%s:%d", entry.AddrV4[0].String(), entry.Port)
	return &WireGateService{
		Host:         entry.Host,
		Addr:         entry.AddrV4[0].String(),
		Port:         entry.Port,
		HTTPEndpoint: httpEndpoint,
		Description:  description,
	}
}

type WireGateHTTPClient struct {
	client *http.Client
}

func (w *WireGateHTTPClient) registerNode(publicKey, vpnPassword, apiEndpoint string) (string, string, string, []string) {
	var reqBuffer bytes.Buffer
	registerReq := &wg.RegistrationRequest{
		PublicKey: publicKey,
		Password:  vpnPassword,
	}
	json.NewEncoder(&reqBuffer).Encode(registerReq)
	rsp, err := w.client.Post(apiEndpoint, "application/json", &reqBuffer)
	if err != nil {
		fmt.Printf("Fatal error while communicating with WireGate Control: %s\n", err)
		os.Exit(1)
	}
	reqBuffer.Reset()

	var registerRsp wg.RegistrationReply
	err = json.NewDecoder(rsp.Body).Decode(&registerRsp)
	if err != nil {
		fmt.Printf("Fatal error while decoding json response from WireGate Control: %s\n", err)
		os.Exit(1)
	}
	return registerRsp.NodeIp, registerRsp.NodeCIDR, registerRsp.EndpointIPPortPair, registerRsp.AllowedIPs
}

func (w *WireGateHTTPClient) StartHeartBeat(wgService *WireGateService, pubKey string) {
	var reqBuffer bytes.Buffer
	var rspBuffer bytes.Buffer
	hbReq := &wg.HeartBeatRequest{
		PublicKey: pubKey,
	}
	json.NewEncoder(&reqBuffer).Encode(hbReq)
	hbTicker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-hbTicker.C:
			rspBuffer.Reset()
			rsp, err := w.client.Post(wgService.HTTPEndpoint, "application/json", &reqBuffer)
			if err != nil {
				fmt.Printf("Error while talking with WireGate Control: %s\n", err)
				break
			}
			var hbRsp wg.HeartBeatResponse
			err = json.NewDecoder(rsp.Body).Decode(&hbRsp)
			if err != nil {
				fmt.Printf("Error while decoding heartbeat response: %s\n", err)
				break
			}
			// TODO: what if wg cmd stalls for too long?
			formattedAllowedIPs := formatAllowedIPsWithCIDR(hbRsp.AllowedIPs)
			setAllowedIPs := exec.Command("wg", "set", "wg0", "allowed-ips", formattedAllowedIPs)
			if _, err := setAllowedIPs.CombinedOutput(); err != nil {
				fmt.Printf("Error while setting up WireGuard interface settings: %s\n", err)
				os.Exit(1)
			}

		}
	}
}

func get_http_client() *WireGateHTTPClient {
	defaultTransport := http.DefaultTransport.(*http.Transport)

	tr := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	return &WireGateHTTPClient{
		client: &http.Client{Transport: tr},
	}
}

func query_mdns_wiregate_svcs(serviceName string, timeout int) []*WireGateService {
	mdnsTimeout := time.Duration(timeout) * time.Second
	serviceEntryChannel := make(chan *mdns.ServiceEntry, 1)
	entries := make([]*WireGateService, 0)
	go func() {
		mdns.Lookup(serviceName, serviceEntryChannel)
		close(serviceEntryChannel)
	}()
	for {
		select {
		case entry, ok := <-serviceEntryChannel:
			if !ok {
				return entries
			}
			entries = append(entries, WGServiceFromServiceEntry(entry))
		case <-time.After(mdnsTimeout):
			return entries
		}
	}
}

func serviceChoicePrompt(serviceChoices []*WireGateService) *WireGateService {
	consoleReader := bufio.NewReader(os.Stdin)
	fmt.Println("Found following WireGate services running on local network:")
	for i, svc := range serviceChoices {
		fmt.Printf("%d) %s (%s:%d)\n", i, svc.Description, svc.Addr, svc.Port)
	}
	fmt.Printf("Enter number of service you wish to connect to (0-%d): ", len(serviceChoices))
	choiceStr, err := consoleReader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error while reading from console: %s\n", err)
		os.Exit(1)
	}
	choice, err := strconv.Atoi(choiceStr)
	if err != nil {
		fmt.Printf("%s is not a number!\n", choiceStr)
		os.Exit(1)
	}
	if choice < 0 || choice > len(serviceChoices) {
		fmt.Printf("%s is outside the range of valid choices\n", choiceStr)
		os.Exit(1)
	}
	return serviceChoices[choice]
}

func generateWGKeypair() (string, string) {
	if _, err := exec.LookPath("wg"); err != nil {
		fmt.Printf("Error: Unable to call 'wg', is WireGuard installed?\n")
		os.Exit(1)
	}
	privKeyCmd := exec.Command("wg", "genkey")
	privKey, err := privKeyCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error while calling 'wg genkey': %s\n", err)
		os.Exit(1)
	}

	pubKeyCmd := exec.Command("wg", "pubkey")
	pubKeyStdin, err := pubKeyCmd.StdinPipe()
	if err != nil {
		fmt.Printf("Error while opening stdin to 'wg pubkey' cmd: %s\n", err)
		os.Exit(1)
	}
	go func() {
		defer pubKeyStdin.Close()
		pubKeyStdin.Write(privKey)
	}()
	pubKey, err := pubKeyCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error while calling 'wg pubkey' cmd: %s\n", err)
		os.Exit(1)
	}
	return string(privKey), string(pubKey)
}

func formatAllowedIPsWithCIDR(allowedIPs []string) string {
	for i, ip := range allowedIPs {
		allowedIPs[i] = fmt.Sprintf("%s/32", ip)
	}
	return strings.Join(allowedIPs, ",")
}

func createWGInterface(wgPrivKey, nodeIP, nodeCIDR, endpointAddr string, allowedIPs []string) {
	// TODO: make interface name be customizable
	ifaceName := "wg0"
	if _, err := exec.LookPath("wg"); err != nil {
		fmt.Printf("Error: Unable to call 'wg', is WireGuard installed?\n")
		os.Exit(1)
	}
	// how to handle cleaning up?
	createIface := exec.Command("ip", "link", "add", "dev", ifaceName, "type", "wireguard")
	if _, err := createIface.CombinedOutput(); err != nil {
		fmt.Printf("Error while creating %s interface: %s\n", ifaceName, err)
		os.Exit(1)
	}
	//address nodeIP must be a cidr
	nodeIPwithCIDR := fmt.Sprintf("%s/%s", nodeIP, nodeCIDR)
	addIfaceAddr := exec.Command("ip", "address", "add", "dev", ifaceName, nodeIPwithCIDR)
	if _, err := addIfaceAddr.CombinedOutput(); err != nil {
		fmt.Printf("Error while assigning address '%s' to %s interface: %s\n", nodeIP, ifaceName, err)
		os.Exit(1)
	}

	formattedAllowedIPs := formatAllowedIPsWithCIDR(allowedIPs)
	wgSetIface := exec.Command("wg", "set", ifaceName, "private-key", wgPrivKey, "peer", "endpoint", endpointAddr, "allowed-ips", formattedAllowedIPs)
	if _, err := wgSetIface.CombinedOutput(); err != nil {
		fmt.Printf("Error while setting up WireGuard interface settings: %s\n", err)
		os.Exit(1)
	}
	enableIface := exec.Command("ip", "link", "set", "up", "dev", ifaceName)
	if _, err := enableIface.CombinedOutput(); err != nil {
		fmt.Printf("Error while enabling interface '%s': %s", ifaceName, err)
		os.Exit(1)
	}
}

func client_main() {
	// search mdns for wiregate services
	WGServices := query_mdns_wiregate_svcs("_wiregate._tcp", 3)
	if len(WGServices) == 0 {
		fmt.Println("No WireGate services found on local network, exiting.")
		os.Exit(0)
	}
	// show prompt asking which one to connect to
	chosenWGService := serviceChoicePrompt(WGServices)

	// ask for password
	vpnPassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Printf("Error while reading password from stdin")
		os.Exit(1)
	}

	// get wireguard private/public key
	wgPubkey, wgPrivKey := generateWGKeypair()

	// register node
	httpClient := get_http_client()
	wgNodeIP, wgNodeCIDR, wgEndpointIPPortPair, wgAllowedIPs := httpClient.registerNode(wgPubkey, string(vpnPassword), chosenWGService.HTTPEndpoint)

	// create wireguard device
	createWGInterface(wgPrivKey, wgNodeIP, wgNodeCIDR, wgEndpointIPPortPair, wgAllowedIPs)

	// keep sending heartbeats + keep updating allowed IPs
	httpClient.StartHeartBeat(chosenWGService, wgPubkey)

	terminator := make(chan os.Signal, 1)
	signal.Notify(terminator, os.Interrupt)
	<-terminator

	// cleanup wg0 iface
	ipLinkDelete := exec.Command("ip", "link", "delete", "dev", "wg0")
	if _, err := ipLinkDelete.CombinedOutput(); err != nil {
		fmt.Printf("Encountered error while removing WireGuard interface 'wg0': %s\n", err)
		os.Exit(1)
	}
}
