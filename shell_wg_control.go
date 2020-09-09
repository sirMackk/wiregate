package wiregate

import (
	"fmt"
	"net"
	"os/exec"
)

var execCommand = NewCommand
var postUpBase = "iptables -A FORWARD -i %s -j ACCEPT; iptables -A FORWARD -o %s -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE"
var postDownBase = "iptables -D FORWARD -i %s -j ACCEPT; iptables -D FORWARD -o %s -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE"

type Commander interface {
	CombinedOutput() ([]byte, error)
}

type Command struct {
	Cmd *exec.Cmd
}

func NewCommand(cmd string, args ...string) Commander {
	return Command{
		Cmd: exec.Command(cmd, args...),
	}
}

func (c Command) CombinedOutput() ([]byte, error) {
	return c.Cmd.CombinedOutput()
}

type ShellWireguardControl struct {
	InterfaceAddress   string
	ListenPort         string
	InterfaceName      string
	PrivateKeyPath     string
	PostUp             string
	PostDown           string
	EndpointIPPortPair string
	EndpointIP         string
}

var getEndpointIPFn = getEndpointIP

func getEndpointIP(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("Unable to resolve local interface %s: %s", ifaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("Unable to get addresses from interface %s: %s", ifaceName, err)
	}
	for _, addr := range addrs {
		if ipnetAddr, ok := addr.(*net.IPNet); ok {
			if ifaceIPv4 := ipnetAddr.IP.To4(); ifaceIPv4 != nil {
				return ifaceIPv4.String(), nil
			}
		}
	}
	return "", nil
}

func NewShellWireguardControl(address, listenPort, wgIface, iface, privateKeypath string) (*ShellWireguardControl, error) {
	postUp := fmt.Sprintf(postUpBase, wgIface, wgIface, iface)
	postDown := fmt.Sprintf(postDownBase, wgIface, wgIface, iface)
	endpointIP, err := getEndpointIPFn(iface)
	if err != nil {
		return nil, err
	}
	s := &ShellWireguardControl{
		PrivateKeyPath:     privateKeypath,
		InterfaceAddress:   address,
		ListenPort:         listenPort,
		InterfaceName:      wgIface,
		PostUp:             postUp,
		PostDown:           postDown,
		EndpointIPPortPair: fmt.Sprintf("%s:%s", endpointIP, listenPort),
		EndpointIP:         endpointIP,
	}
	return s, nil
}

func (s *ShellWireguardControl) CreateInterface() error {
	//TODO figure out getting ipv4/ipv6 proto version
	proto := "-4"

	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("Command 'ip' not found!")
	}
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("Command 'wg' not found!")
	}
	// create interface
	createIface := execCommand("ip", "link", "add", "dev", s.InterfaceName, "type", "wireguard")
	if out, err := createIface.CombinedOutput(); err != nil {
		return fmt.Errorf("Creating interface failed when calling ip: %s\n%s", err, out)
	}

	// add ip addr to iface
	ipSetAddr := execCommand("ip", proto, "address", "add", s.InterfaceAddress, "dev", s.InterfaceName)
	if out, err := ipSetAddr.CombinedOutput(); err != nil {
		return fmt.Errorf("Adding ip address to interface %s failed: %s\n%s", s.InterfaceName, err, out)
	}

	// wg setconf flags and args
	wgSetConfig := execCommand("wg", "set", s.InterfaceName, "listen-port", s.ListenPort, "private-key", s.PrivateKeyPath)
	if out, err := wgSetConfig.CombinedOutput(); err != nil {
		return fmt.Errorf("Creating interface failed when calling wg: %s\n%s", err, out)
	}

	// activate interface
	activateIface := execCommand("ip", "link", "set", "up", "dev", s.InterfaceName)
	if out, err := activateIface.CombinedOutput(); err != nil {
		return fmt.Errorf("Activating interface failed when calling ip: %s\n%s", err, out)
	}

	// execute postup
	postUpCmd := execCommand("sh", "-c", s.PostUp)
	if out, err := postUpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Activating interface failed when executing post-up cmd: %s\n%s", err, out)
	}

	return nil
}

func (s *ShellWireguardControl) DestroyInterface() error {
	//TODO figure out getting ipv4/ipv6 proto version
	//proto := "-4"
	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("Command 'ip' not found!")
	}
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("Command 'wg' not found!")
	}
	// ip -4 rule show - wg allowed-ips to see if these change ip -4 rule show results
	ipLinkDelete := execCommand("ip", "link", "delete", "dev", s.InterfaceName, "type", "wireguard")
	if out, err := ipLinkDelete.CombinedOutput(); err != nil {
		return fmt.Errorf("Deleting interface %s failed: %s\n%s", s.InterfaceName, err, out)
	}
	postDownCmd := execCommand("sh", "-c", s.PostDown)
	if out, err := postDownCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Executing post-down cmd failed: %s\n%s", err, out)
	}

	return nil
}

func (s *ShellWireguardControl) AddHost(pubkey, peerIP string) error {
	wgSetPeer := execCommand("wg", "set", s.InterfaceName, "peer", pubkey, "allowed-ips", peerIP)
	if out, err := wgSetPeer.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to add peer (%s - %s): %s\n%s", pubkey, peerIP, err, out)
	}
	return nil
}

func (s *ShellWireguardControl) RemoveHost(pubkey string) error {
	wgSetPeerRemove := execCommand("wg", "set", s.InterfaceName, "peer", pubkey, "remove")
	if out, err := wgSetPeerRemove.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to removed peer (%s): %s\n%s", pubkey, err, out)
	}
	return nil
}
