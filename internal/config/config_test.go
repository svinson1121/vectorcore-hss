package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNormalizesFiveGCConfig(t *testing.T) {
	cfgText := `
hss:
  OriginHost: hss01.example.org
  OriginRealm: example.org
database:
  db_type: sqlite
  database: ":memory:"
5gc:
  sbi:
    bind_address: "::"
    bind_port: 7777
    oauth2_enabled: true
    oauth2_bypass: false
    client:
      mode: scp
      scp_address: "http://127.0.0.200:7777/nscp-proxy/v1"
  udm:
    enabled: true
    mcc: "311"
    mnc: "435"
    nrf_address: "http://127.0.0.10:7777"
    nf_instance_id: "udm-id"
    suci_decryption_keys:
      - key_id: 1
        scheme: 1
        key_file: /tmp/key.pem
  pcf:
    enabled: true
    mcc: "311"
    mnc: "435"
    nrf_address: "http://127.0.0.10:7777"
    nf_instance_id: "pcf-id"
`
	path := writeTestConfig(t, cfgText)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.UDM.Enabled || !cfg.PCF.Enabled {
		t.Fatalf("expected UDM and PCF enabled, got %+v %+v", cfg.UDM, cfg.PCF)
	}
	if cfg.UDM.BindPort != 7777 || cfg.PCF.BindPort != 7777 {
		t.Fatalf("expected shared bind port 7777, got %d %d", cfg.UDM.BindPort, cfg.PCF.BindPort)
	}
	if !cfg.UDM.OAuth2Enabled || cfg.UDM.OAuth2Bypass {
		t.Fatalf("expected shared OAuth settings on UDM, got %+v", cfg.UDM)
	}
	if cfg.PCF.SBIClient.Mode != "scp" {
		t.Fatalf("expected shared SBI client mode on PCF, got %+v", cfg.PCF.SBIClient)
	}
	if len(cfg.UDM.SUCIDecryptionKeys) != 1 {
		t.Fatalf("expected SUCI decryption keys to normalize, got %+v", cfg.UDM.SUCIDecryptionKeys)
	}
}

func TestLoadKeepsLegacyHNetKeys(t *testing.T) {
	cfgText := `
hss:
  OriginHost: hss01.example.org
  OriginRealm: example.org
database:
  db_type: sqlite
  database: ":memory:"
udm:
  enabled: true
  hnet_keys:
    - key_id: 1
      scheme: 1
      key_file: /tmp/key.pem
`
	path := writeTestConfig(t, cfgText)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.UDM.SUCIDecryptionKeys) != 1 {
		t.Fatalf("expected legacy hnet_keys to map, got %+v", cfg.UDM.SUCIDecryptionKeys)
	}
}

func writeTestConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
