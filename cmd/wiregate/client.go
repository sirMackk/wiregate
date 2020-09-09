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
	log "github.com/sirupsen/logrus"

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
	url := fmt.Sprintf("https://%s/register", apiEndpoint)
	rsp, err := w.client.Post(url, "application/json", &reqBuffer)
	if err != nil {
		log.Errorf("Fatal error while communicating with WireGate Control: %s", err)
		os.Exit(1)
	}
	// TODO handle non-200 rsps?
	reqBuffer.Reset()

	log.Debugf("registerNode response: %#v\n", rsp)
	var registerRsp wg.RegistrationReply
	err = json.NewDecoder(rsp.Body).Decode(&registerRsp)
	if err != nil {
		log.Errorf("Fatal error while decoding json response from WireGate Control: %s", err)
		log.Debugf("registerNode response body %#v\n", rsp.Body)
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
	log.Info("Starting heart beat")
	for {
		select {
		case <-hbTicker.C:
			log.Debug("Heart beat")
			rspBuffer.Reset()
			// TODO: better url formation
			url := fmt.Sprintf("https://%s/beat", wgService.HTTPEndpoint)
			log.Debugf("Sending beat to %s: %#v", url, reqBuffer)
			rsp, err := w.client.Post(url, "application/json", &reqBuffer)
			if err != nil {
				log.Errorf("Error while talking with WireGate Control: %s", err)
				break
			}

			log.Debugf("Received beat response: %#v", rsp)
			var hbRsp wg.HeartBeatResponse
			err = json.NewDecoder(rsp.Body).Decode(&hbRsp)
			if err != nil {
				log.Errorf("Error while decoding heartbeat response: %s", err)
				break
			}
			// TODO: what if wg cmd stalls for too long?
			formattedAllowedIPs := formatAllowedIPsWithCIDR(hbRsp.AllowedIPs)
			log.Debugf("Extracted allowedIPs from beat: %v", formattedAllowedIPs)
			setAllowedIPs := exec.Command("wg", "set", "wg0", "allowed-ips", formattedAllowedIPs)
			if _, err := setAllowedIPs.CombinedOutput(); err != nil {
				log.Errorf("Error while setting up WireGuard interface settings: %s", err)
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
		// We don't care about the identity of the server, we just want an encrypted connection.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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
	fmt.Printf("Enter number of service you wish to connect to (0-%d): ", len(serviceChoices)-1)
	choiceStr, err := consoleReader.ReadString('\n')
	if err != nil {
		log.Errorf("Error while reading from console: %s", err)
		os.Exit(1)
	}
	choice, err := strconv.Atoi(strings.TrimSpace(choiceStr))
	if err != nil {
		log.Errorf("%s is not a number!", choiceStr)
		os.Exit(1)
	}
	if choice < 0 || choice > len(serviceChoices) {
		log.Errorf("%s is outside the range of valid choices", choiceStr)
		os.Exit(1)
	}
	return serviceChoices[choice]
}

func generateWGKeypair() (string, string) {
	if _, err := exec.LookPath("wg"); err != nil {
		log.Errorf("Error: Unable to call 'wg', is WireGuard installed?")
		os.Exit(1)
	}
	privKeyCmd := exec.Command("wg", "genkey")
	privKey, err := privKeyCmd.CombinedOutput()
	if err != nil {
		log.Errorf("Error while calling 'wg genkey': %s", err)
		os.Exit(1)
	}

	pubKeyCmd := exec.Command("wg", "pubkey")
	pubKeyStdin, err := pubKeyCmd.StdinPipe()
	if err != nil {
		log.Errorf("Error while opening stdin to 'wg pubkey' cmd: %s", err)
		os.Exit(1)
	}
	go func() {
		defer pubKeyStdin.Close()
		pubKeyStdin.Write(privKey)
	}()
	pubKey, err := pubKeyCmd.CombinedOutput()
	if err != nil {
		log.Errorf("Error while calling 'wg pubkey' cmd: %s", err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(privKey)), strings.TrimSpace(string(pubKey))
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
		log.Errorf("Error: Unable to call 'wg', is WireGuard installed?")
		os.Exit(1)
	}
	// how to handle cleaning up?
	createIface := exec.Command("ip", "link", "add", "dev", ifaceName, "type", "wireguard")
	if _, err := createIface.CombinedOutput(); err != nil {
		log.Errorf("Error while creating %s interface: %s", ifaceName, err)
		os.Exit(1)
	}
	//address nodeIP must be a cidr
	nodeIPwithCIDR := fmt.Sprintf("%s/%s", nodeIP, nodeCIDR)
	addIfaceAddr := exec.Command("ip", "address", "add", "dev", ifaceName, nodeIPwithCIDR)
	if _, err := addIfaceAddr.CombinedOutput(); err != nil {
		log.Errorf("Error while assigning address '%s' to %s interface: %s", nodeIP, ifaceName, err)
		os.Exit(1)
	}

	formattedAllowedIPs := formatAllowedIPsWithCIDR(allowedIPs)
	wgSetIface := exec.Command("wg", "set", ifaceName, "private-key", wgPrivKey, "peer", "endpoint", endpointAddr, "allowed-ips", formattedAllowedIPs)
	if _, err := wgSetIface.CombinedOutput(); err != nil {
		log.Errorf("Error while setting up WireGuard interface settings: %s", err)
		os.Exit(1)
	}
	enableIface := exec.Command("ip", "link", "set", "up", "dev", ifaceName)
	if _, err := enableIface.CombinedOutput(); err != nil {
		log.Errorf("Error while enabling interface '%s': %s", ifaceName, err)
		os.Exit(1)
	}
}

func client_main() {
	// search mdns for wiregate services
	WGServices := query_mdns_wiregate_svcs("_wiregate._tcp", 3)
	if len(WGServices) == 0 {
		log.Info("No WireGate services found on local network, exiting.")
		os.Exit(0)
	}
	// show prompt asking which one to connect to
	chosenWGService := serviceChoicePrompt(WGServices)

	// ask for password
	fmt.Printf("Enter password: ")
	vpnPassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Error("Error while reading password from stdin")
		os.Exit(1)
	}
	fmt.Printf("\n")

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
		fmt.Errorf("Encountered error while removing WireGuard interface 'wg0': %s", err)
		os.Exit(1)
	}
}
