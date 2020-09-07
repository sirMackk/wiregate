package main

import (
	"fmt"
	"os/exec"
	"testing"
)

// TODO: Cover cmd failure scenarios
// TODO: In tests, check command args are correct

type MockCommand struct {
	Cmd    *exec.Cmd
	Ret    []byte
	RetErr error
}

func NewMockCommand(cmd string, args ...string) MockCommand {
	return MockCommand{
		Cmd: exec.Command(cmd, args...),
	}
}

func (m MockCommand) CombinedOutput() ([]byte, error) {
	return m.Ret, m.RetErr
}

func createTestServer() *WireguardServer {
	return NewWireguardServer(
		"10.24.99.1/24",
		"123",
		"interface0",
		"/tmp/privateKeyFilePath",
		"postUpCmd",
		"postDownCmd",
	)
}

func TestAddRemoveHost(t *testing.T) {
	s := createTestServer()
	var addHostTests = []struct {
		addOrRemove string
		name        string
		cmdOutput   string
		cmdRetErr   string
		shouldError bool
	}{
		{"add", "Adding host", "Success", "", false},
		{"add", "Adding host", "Failure", "Bad args", true},
		{"remove", "Removing host", "Success", "", false},
		{"remove", "Removing host", "Failure", "Bad args", true},
	}
	for _, tt := range addHostTests {
		t.Run(tt.name, func(t *testing.T) {
			execCommand = func(cmd string, args ...string) Commander {
				mockCmd := NewMockCommand(cmd, args...)
				mockCmd.Ret = []byte(tt.cmdOutput)
				if tt.cmdRetErr != "" {
					mockCmd.RetErr = fmt.Errorf(tt.cmdRetErr)
				} else {
					mockCmd.RetErr = nil
				}
				return mockCmd
			}

			var err error
			if tt.addOrRemove == "add" {
				err = s.AddHost("123123", "10.24.0.10/32")
			} else {
				err = s.RemoveHost("123123")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("Test shouldnt error, but did: %s", err)
			}
		})
	}
}

func TestCreateDestroyInterface(t *testing.T) {
	s := createTestServer()
	var interfaceTests = []struct {
		createOrDestroy string
		name            string
		cmdOutput       string
		cmdRetErr       string
		shouldError     bool
	}{
		{"create", "Create interface", "Success", "", false},
		{"create", "Create interface", "Failure", "Bad args", true},
		{"destroy", "Destroy interface", "Success", "", false},
		{"destroy", "Destroy interface", "Failure", "Bad args", true},
	}
	for _, tt := range interfaceTests {
		t.Run(tt.name, func(t *testing.T) {
			execCommand = func(cmd string, args ...string) Commander {
				mockCmd := NewMockCommand(cmd, args...)
				mockCmd.Ret = []byte(tt.cmdOutput)
				if tt.cmdRetErr != "" {
					mockCmd.RetErr = fmt.Errorf(tt.cmdRetErr)
				} else {
					mockCmd.RetErr = nil
				}
				return mockCmd
			}
			var err error
			if tt.createOrDestroy == "create" {
				err = s.CreateInterface()
			} else {
				err = s.DestroyInterface()
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Test shouldn't error, but did: %s", err)
			}
		})
	}
}
