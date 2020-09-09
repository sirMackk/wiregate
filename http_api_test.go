package wiregate

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisteringNewNodes(t *testing.T) {
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})
	api := HttpApi{Registry: registry, EndpointIPPortPair: "127.0.0.1:8083", WGServerPublicKey: "123", WGServerPeerIP: "1.1.1.1"}

	var registrationTests = []struct {
		name           string
		method         string
		jsonPayload    string
		expectedStatus int
		expectedRsp    string
	}{
		{"goodRequest", "POST", `{"publicKey": "somePublicKey1"}`, http.StatusOK,
			`{"NodeIp":"1.1.1.1","NodeCIDR":"/24","EndpointIPPortPair":"127.0.0.1:8083","AllowedIPs":["1.1.1.1"],"WGServerPublicKey":"123","WGServerPeerIP":"1.1.1.1"}`},
		{"wrongMethod", "PUT", "", http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)},
		{"badJson", "POST", `{"publicKe": `, http.StatusInternalServerError, "Error: unexpected EOF"},
		{"dupeKey", "POST", `{"publicKey": "somePublicKey1"}`, http.StatusInternalServerError,
			`Node with pubkey somePublicKey1 already exists`},
	}

	for _, tt := range registrationTests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.WriteString(tt.jsonPayload)
			req, err := http.NewRequest(tt.method, "/register", &buf)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(api.registerNode)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Unexpected status code %d, want %d", http.StatusOK, status)
			}
			if strings.TrimSuffix(rr.Body.String(), "\n") != tt.expectedRsp {
				t.Errorf("Unexpected response, got %#v, want %#v", rr.Body.String(), tt.expectedRsp)
			}
		})
	}
}

func TestRemovingNode(t *testing.T) {
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})
	registry.Put("pubKey1")
	api := HttpApi{Registry: registry, EndpointIPPortPair: "127.0.0.1:8083"}

	var deletionTests = []struct {
		name           string
		method         string
		jsonPayload    string
		expectedStatus int
		expectedRsp    string
	}{
		{"removeNode", "DELETE", `{"publicKey":"pubKey1"}`, http.StatusNoContent, ""},
		{"badMethod", "POST", `{"publicKey":"pubKey1"}`, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)},
		{"badJson", "DELETE", `{"someKindaofJson`, http.StatusInternalServerError, "Error while decoding json"},
		{"alreadyRemoved", "DELETE", `{"publicKey":"pubKey1"}`, http.StatusNotFound, "Node not found"},
	}
	for _, tt := range deletionTests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.WriteString(tt.jsonPayload)
			req, err := http.NewRequest(tt.method, "/delete", &buf)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(api.unregisterNode)
			handler.ServeHTTP(rr, req)
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Unexpected status code %d, want %d", rr.Code, status)
			}
			if strings.TrimRight(rr.Body.String(), "\n") != tt.expectedRsp {
				t.Errorf("Unexpected response, got %#v, want %#v", rr.Body.String(), tt.expectedRsp)
			}
		})
	}
}

func TestHeartBeat(t *testing.T) {
	registry := NewRegistry(&FakeIPGen{}, &FakeWgControl{})
	registry.Put("pubKey1")
	api := HttpApi{Registry: registry, EndpointIPPortPair: "127.0.0.1:8083"}

	var heartBeatTests = []struct {
		name           string
		method         string
		jsonPayload    string
		expectedStatus int
		expectedRsp    string
	}{
		{"beatHeart", "POST", `{"publicKey":"pubKey1"}`, http.StatusOK, `{"AllowedIPs":["1.1.1.1"]}`},
		{"badMethod", "GET", `{"publicKey":"pubKey1"}`, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)},
		{"badJson", "POST", `{"publicKey":`, http.StatusInternalServerError, "Error while decoding json"},
		{"badKey", "POST", `{"publicKey":"pubKey999"}`, http.StatusNotFound, "Node not found"},
	}
	for _, tt := range heartBeatTests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.WriteString(tt.jsonPayload)
			req, err := http.NewRequest(tt.method, "/heartbeat", &buf)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(api.heartBeat)
			handler.ServeHTTP(rr, req)
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Unexpected status code %d, want %d", rr.Code, tt.expectedStatus)
			}

			if strings.TrimSuffix(rr.Body.String(), "\n") != tt.expectedRsp {
				t.Errorf("Unexpected response, got %#v, want %#v", rr.Body.String(), tt.expectedRsp)
			}
		})
	}
}
