package main

import (
	"reflect"
	"testing"
)

func TestAvailableIPGeneration(t *testing.T) {
	ipgen, err := NewSimpleIPGen("192.168.1.2/29")
	if err != nil {
		t.Errorf("Error while initializing SimpleIPGen: %s", err)
	}
	expectedNumberOfIPs := 6
	if ips := len(ipgen.AvailableIPs); ips != expectedNumberOfIPs {
		t.Errorf("Expecting to have %d ips, got %d", expectedNumberOfIPs, ips)
	}
	expectedAvailableIPs := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3",
		"192.168.1.4", "192.168.1.5", "192.168.1.6",
	}
	if reflect.DeepEqual(expectedAvailableIPs, ipgen.AvailableIPs) {
		t.Errorf("Expected available IPs: %v\nGot: %v", expectedAvailableIPs, ipgen.AvailableIPs)
	}
}

func TestAvailableIPGenerationOverTop(t *testing.T) {
	ipgen, err := NewSimpleIPGen("192.168.1.1/23")
	if err != nil {
		t.Errorf("Error while initializing SimpleIPGen: %s", err)
	}
	expectedNumberOfIPs := 510
	if ips := len(ipgen.AvailableIPs); ips != expectedNumberOfIPs {
		t.Errorf("Expecting to have %d ips, got %d", expectedNumberOfIPs, ips)
	}
}

func TestLeasingIPs(t *testing.T) {
	ipgen, err := NewSimpleIPGen("192.168.1.2/29")
	if err != nil {
		t.Errorf("Error while initializing SimpleIPGen: %s", err)
	}
	_, err = ipgen.LeaseIP()
	if err != nil {
		t.Errorf("Error while leasing IP : %s", err)
	}
	for i := 0; i < 5; i++ {
		_, err = ipgen.LeaseIP()
		if err != nil {
			t.Errorf("Expected to lease 5 ips, got error while trying to lease %d ip: %s", i, err)
		}
	}
	_, err = ipgen.LeaseIP()
	if err == nil {
		t.Errorf("Expected error when leasing exhausted ipgen, but there was no error")
	}
}

func TestReleasingIPs(t *testing.T) {
	ipgen, err := NewSimpleIPGen("192.168.1.2/29")
	if err != nil {
		t.Errorf("Error while initializing SimpleIPGen: %s", err)
	}
	ip, err := ipgen.LeaseIP()
	if err != nil {
		t.Errorf("Error while leasing IP : %s", err)
	}
	err = ipgen.ReleaseIP(ip)
	if err != nil {
		t.Errorf("Error while releasing IP: %s", err)
	}
	err = ipgen.ReleaseIP("10.24.1.1")
	if err == nil {
		t.Errorf("Expected error when trying to release IP that doesnt exist, but there was no error")
	}
}
