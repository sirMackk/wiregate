package wiregate

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

type FakeIPGen struct {
	count int
}

func (f *FakeIPGen) LeaseIP() (string, string, error) {
	f.count += 1
	return fmt.Sprintf("1.1.1.%d", f.count), "/24", nil
}

func (f *FakeIPGen) ReleaseIP(string) error { return nil }

// TODO: replace with mock to asset calls made to it
type FakeWgControl struct{}

func (f *FakeWgControl) AddHost(key, ip string) error {
	return nil
}

func (f *FakeWgControl) RemoveHost(key string) error {
	return nil
}

func TestGettingNode(t *testing.T) {
	pubkey := "publicKey1"
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})
	n1, err := registry.Put(pubkey)
	if err != nil {
		t.Errorf("Problem with creating registry entry: %v", err)
	}

	_, err = registry.Put(pubkey)
	if err == nil {
		t.Errorf("Created node with duplicated pubkey!")
	}

	n2, err := registry.Get(pubkey)
	if !reflect.DeepEqual(n1, n2) {
		t.Errorf("Get didnt return the same node as was Put: %+v != %+v", n1, n2)
	}

	noNode, err := registry.Get("pubkey_doesnt_exist")
	if noNode != nil || err == nil {
		t.Errorf("Tried to Get inexistent key and didnt fail: %v", noNode)
	}
}

func TestDeletingNode(t *testing.T) {
	pubkey := "publicKey1"
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})
	registry.Put(pubkey)
	err := registry.Delete(pubkey)
	if err != nil {
		t.Errorf("Problem with deleting node: %v", err)
	}

	noNode, err := registry.Get(pubkey)
	if noNode != nil || err == nil {
		t.Errorf("Failed to delete node!")
	}

	err = registry.Delete(pubkey)
	if err == nil {
		t.Errorf("Attempted to delete inexistent node and succeeded!?")
	}
}

func TestGetRegisteredIPs(t *testing.T) {
	pubkey1 := "publicKey1"
	pubkey2 := "publicKey2"
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})

	n1, _ := registry.Put(pubkey1)
	n2, _ := registry.Put(pubkey2)
	expectedIPs := []string{n1.VPNIP, n2.VPNIP}
	resultingIPs := registry.GetRegisteredIPs()
	if len(expectedIPs) != len(resultingIPs) {
		t.Errorf("Was expecting %d IPs, but got %d!", len(expectedIPs), len(resultingIPs))
	}
	if !reflect.DeepEqual(resultingIPs, expectedIPs) {
		t.Errorf("IPs do not match! Expected %v, got %v", expectedIPs, resultingIPs)
	}
}

func TestPurgingGoro(t *testing.T) {
	pubkey1 := "publicKey1"
	pubkey2 := "publicKey2"
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})

	n1, _ := registry.Put(pubkey1)
	n1.lastAliveAt -= 3
	n2, _ := registry.Put(pubkey2)
	n2.lastAliveAt += 10

	registry.StartPurging(1, 1)
	time.Sleep(100 * time.Millisecond)
	registry.StopPurging()

	noNode, err := registry.Get(pubkey1)
	if noNode != nil || err == nil {
		t.Errorf("Expected node '%s' to be purged, but it wasnt", pubkey1)
	}
	n2Node, err := registry.Get(pubkey2)
	if n2Node == nil || !reflect.DeepEqual(n2, n2Node) {
		t.Errorf("Expected node 2 '%s' to exist, but got %v", pubkey2, n2Node)
	}
}
