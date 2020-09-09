package wiregate

import (
	"fmt"
	"os/exec"
	"testing"
)

// TODO: Cover cmd failure scenarios

type MockCommand struct {
	Cmd    *exec.Cmd
	Ret    []byte
	RetErr error
	// TODO: In tests, check command args are correct
	Invocations [][]string
}

func NewMockCommand(cmd, cmdOutput, cmdRetErr string, args ...string) MockCommand {
	mc := MockCommand{
		Cmd:         exec.Command(cmd, args...),
		Ret:         []byte(cmdOutput),
		Invocations: make([][]string, 0),
	}
	mc.SaveInvocation(cmd, args)
	if cmdRetErr != "" {
		mc.RetErr = fmt.Errorf(cmdRetErr)
	} else {
		mc.RetErr = nil
	}

	return mc
}

func (m MockCommand) CombinedOutput() ([]byte, error) {
	return m.Ret, m.RetErr
}

func (m MockCommand) SaveInvocation(cmd string, args []string) {
	invocation := make([]string, len(args)+1)
	invocation[0] = cmd
	for i := 1; i < len(args)+1; i++ {
		invocation[i] = args[i-1]
	}
	m.Invocations = append(m.Invocations, invocation)
}

func createTestServer() *ShellWireguardControl {

	getEndpointIPFn = func(iface string) (string, error) {
		return "199.199.199.199", nil
	}
	srv, _ := NewShellWireguardControl(
		"10.24.99.1/24",
		"10.24.99.0",
		"24",
		"123",
		"wg-interface0",
		"iface0",
		"/tmp/privateKeyFilePath",
	)
	return srv
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
				mockCmd := NewMockCommand(cmd, tt.cmdOutput, tt.cmdRetErr, args...)
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
				mockCmd := NewMockCommand(cmd, tt.cmdOutput, tt.cmdRetErr, args...)
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
