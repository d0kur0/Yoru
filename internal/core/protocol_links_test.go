package core

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestExtendedProtocolImports(t *testing.T) {
	key := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	fixtures := map[string]string{
		"Hysteria2-obfs": "hy2://user:password@example.com:443?obfs=salamander&obfs-password=secret%2Bvalue&alpn=h3&sni=example.com&insecure=0",
		"VMess":          "vmess://" + base64.StdEncoding.EncodeToString([]byte(`{"v":"2","ps":"VMess","add":"example.com","port":"443","id":"01234567-89ab-cdef-0123-456789abcdef","aid":"0","net":"ws","host":"example.com","path":"/vpn","tls":"tls","sni":"example.com"}`)),
		"TUIC":           "tuic://01234567-89ab-cdef-0123-456789abcdef:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3",
		"AnyTLS":         "anytls://test-password@example.com:443?sni=example.com",
		"XHTTP":          "vless://01234567-89ab-cdef-0123-456789abcdef@example.com:443?security=tls&type=xhttp&path=%2Fvpn&mode=packet-up&sni=example.com",
		"WireGuard":      "[Interface]\nPrivateKey = " + key + "\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = " + key + "\nEndpoint = example.com:51820\nAllowedIPs = 0.0.0.0/0\n",
		"AmneziaWG":      "proxies:\n - name: AWG\n   type: wireguard\n   server: example.com\n   port: 51820\n   ip: 10.0.0.2\n   private-key: " + key + "\n   public-key: " + key + "\n   amnezia-wg-option:\n     version: 2\n     jc: 4\n     jmin: 40\n     jmax: 70\n",
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			s, e := ParseLink(source)
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.Proxy()
			if e != nil {
				t.Fatal(e)
			}
			if name == "Hysteria2-obfs" && (p["obfs"] != "salamander" || p["obfs-password"] != "secret+value" || p["password"] != "user:password" || p["skip-cert-verify"] != false) {
				t.Fatal("lost Hysteria2 parameters")
			}
			if name == "XHTTP" && p["network"] != "xhttp" {
				t.Fatal("lost transport")
			}
			if name == "AmneziaWG" && p["amnezia-wg-option"] == nil {
				t.Fatal("lost AWG options")
			}
			c := DefaultConfig()
			c.Settings.TUN = false
			c.Servers = []Server{s}
			c.Selected = s.ID
			data, e := c.YAML("127.0.0.1:19099", "test", 19098)
			if e != nil {
				t.Fatal(e)
			}
			if binary := os.Getenv("MIHOMO_TEST_BINARY"); binary != "" {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.yaml")
				if e = os.WriteFile(path, data, 0600); e != nil {
					t.Fatal(e)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if output, e := exec.CommandContext(ctx, binary, "-t", "-d", dir, "-f", path).CombinedOutput(); e != nil {
					t.Fatalf("core validation: %v %s", e, output)
				}
			}
		})
	}
}

func TestXHTTPExtraNotSilentlyLost(t *testing.T) {
	_, e := ParseLink("vless://01234567-89ab-cdef-0123-456789abcdef@example.com:443?security=tls&type=xhttp&extra=%7B%7D")
	if e == nil {
		t.Fatal("extra silently ignored")
	}
}
