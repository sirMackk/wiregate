package wiregate

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

type IPGenerator interface {
	LeaseIP() (string, string, error)
	ReleaseIP(string) error
}

type SimpleIPGen struct {
	BaseIPCIDR   string
	BaseIP       string
	CIDR         string
	LastIP       string
	AvailableIPs map[string]bool
}

// baseIPCIDR is an ipv4 w/ cidr address that is used to generate
// the whole range of IPs in the subnet.
func NewSimpleIPGen(baseIPCIDR string) (*SimpleIPGen, error) {
	ipgen := &SimpleIPGen{
		BaseIPCIDR:   baseIPCIDR,
		CIDR:         strings.Split(baseIPCIDR, "/")[1],
		AvailableIPs: make(map[string]bool),
	}
	err := ipgen.populateIPs()
	if err != nil {
		return nil, err
	}
	return ipgen, nil
}

func (i *SimpleIPGen) populateIPs() error {
	baseIP, ipNet, err := net.ParseCIDR(i.BaseIPCIDR)
	i.BaseIP = baseIP.String()
	if err != nil {
		return err
	}
	mask := []byte(ipNet.Mask)
	notMask := make([]byte, len(mask))
	for i, octet := range mask {
		notMask[i] = ^octet
	}
	lastIP := make([]byte, len(mask))
	for i, octet := range baseIP.To4() {
		lastIP[i] = octet | notMask[i]
	}
	i.LastIP = fmt.Sprintf("%d.%d.%d.%d", lastIP[0], lastIP[1], lastIP[2], lastIP[3])
	// Use network base IP to get full range
	i.generateIPsInRange(ipNet.IP.To4(), lastIP)
	return nil
}

func (i *SimpleIPGen) generateIPsInRange(baseIP, lastIP []byte) {
	lastOctet := len(baseIP) - 1
	baseIP[lastOctet]++
	for ip := append([]byte(nil), baseIP...); !bytes.Equal(ip, lastIP); {
		if ip[lastOctet] == 255 {
			// Roll over the previous octets
			for i := lastOctet; i > 0; i-- {
				if ip[i] == 255 {
					ip[i] = 0
					if i > 0 {
						ip[i-1]++
					}
				}
			}
		} else {
			ip[lastOctet]++
		}
		i.AvailableIPs[fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])] = false
	}
}

func (i *SimpleIPGen) LeaseIP() (string, string, error) {
	for ip, leased := range i.AvailableIPs {
		if !leased {
			i.AvailableIPs[ip] = true
			return ip, i.CIDR, nil
		}
	}
	return "", "", fmt.Errorf("Error! No IPs left to lease!")
}

func (i *SimpleIPGen) ReleaseIP(ip string) error {
	if _, ok := i.AvailableIPs[ip]; ok {
		i.AvailableIPs[ip] = false
	} else {
		return fmt.Errorf("IP %s not in available IPs!", ip)
	}
	return nil
}
