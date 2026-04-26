package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HSS      HSSConfig      `yaml:"hss"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
	EIR      EIRConfig      `yaml:"eir"`
	Roaming  RoamingConfig  `yaml:"roaming"`
	Geored   GeoredConfig   `yaml:"geored"`
	API      APIConfig      `yaml:"api"`
	GSUP     GSUPConfig     `yaml:"gsup"`
	FiveGC   FiveGCConfig   `yaml:"5gc"`
	UDM      UDMConfig      `yaml:"udm"`
	PCF      PCFConfig      `yaml:"pcf"`
}

type SBIClientConfig struct {
	Mode                 string        `yaml:"mode"`
	SCPAddress           string        `yaml:"scp_address"`
	ReconnectHoldoffTime time.Duration `yaml:"reconnect_holdoff_time"`
}

type FiveGCSBIConfig struct {
	BindAddress   string          `yaml:"bind_address"`
	BindPort      int             `yaml:"bind_port"`
	TLSCertFile   string          `yaml:"tls_cert_file"`
	TLSKeyFile    string          `yaml:"tls_key_file"`
	OAuth2Enabled bool            `yaml:"oauth2_enabled"`
	OAuth2Bypass  bool            `yaml:"oauth2_bypass"`
	Client        SBIClientConfig `yaml:"client"`
}

type FiveGCNFConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MCC          string `yaml:"mcc"`
	MNC          string `yaml:"mnc"`
	NRFAddress   string `yaml:"nrf_address"`
	NFInstanceID string `yaml:"nf_instance_id"`
}

type FiveGCUDMConfig struct {
	FiveGCNFConfig     `yaml:",inline"`
	SUCIDecryptionKeys []HNetKeyConfig `yaml:"suci_decryption_keys"`
}

type FiveGCPCFConfig struct {
	FiveGCNFConfig `yaml:",inline"`
}

type FiveGCConfig struct {
	SBI FiveGCSBIConfig `yaml:"sbi"`
	UDM FiveGCUDMConfig `yaml:"udm"`
	PCF FiveGCPCFConfig `yaml:"pcf"`
}

// UDMConfig controls the 5G UDR/UDM listener (Nudm SBI interfaces).
// VectorCore acts as both UDM and UDR — it implements the Nudm REST APIs
// that Open5GS AUSF/AMF/SMF call, backed directly by the same PostgreSQL DB.
type UDMConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_address"`
	BindPort    int    `yaml:"bind_port"`
	// MCC and MNC identify the home PLMN for NRF registration.
	// If left empty they are inherited from hss.MCC / hss.MNC at startup.
	MCC string `yaml:"mcc"`
	MNC string `yaml:"mnc"`
	// NRFAddress is the base URL of the Open5GS NRF, e.g. "http://nrf:7777".
	// Leave empty to skip NRF registration (standalone / dev mode).
	NRFAddress string `yaml:"nrf_address"`
	// NFInstanceID is a stable UUID for this UDM instance.
	// Auto-generated on first start if left blank.
	NFInstanceID string `yaml:"nf_instance_id"`
	// TLS — leave blank for cleartext HTTP/2 (h2c), used in typical Open5GS lab setups.
	TLSCertFile string `yaml:"tls_cert_file"`
	TLSKeyFile  string `yaml:"tls_key_file"`
	// OAuth2Enabled validates Bearer JWT tokens on inbound requests.
	OAuth2Enabled bool `yaml:"oauth2_enabled"`
	// OAuth2Bypass skips token validation (dev/lab mode only).
	OAuth2Bypass bool            `yaml:"oauth2_bypass"`
	SBIClient    SBIClientConfig `yaml:"sbi_client"`
	// SUCIDecryptionKeys holds the Home Network private keys used to decrypt
	// encrypted SUCI (SUCI Protection Scheme Profile A / Profile B per
	// TS 33.501 Annex C). Each entry maps a
	// HomeNetworkPublicKeyIdentifier (1-255) to a PEM key file.
	SUCIDecryptionKeys []HNetKeyConfig `yaml:"suci_decryption_keys"`
	// LegacyHNetKeys keeps backward compatibility with the older config key.
	LegacyHNetKeys []HNetKeyConfig `yaml:"hnet_keys"`
}

// HNetKeyConfig is one entry in the Home Network key list.
type HNetKeyConfig struct {
	// KeyID is the HomeNetworkPublicKeyIdentifier (1–255) set on the SIM.
	KeyID int `yaml:"key_id"`
	// Scheme: 1 = Profile A (X25519/ECIES), 2 = Profile B (P-256/ECIES).
	Scheme int `yaml:"scheme"`
	// KeyFile is the path to the PEM-encoded EC private key file.
	KeyFile string `yaml:"key_file"`
}

// PCFConfig controls the 5G PCF listener (Npcf SBI interfaces).
type PCFConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_address"`
	BindPort    int    `yaml:"bind_port"`
	// MCC and MNC identify the home PLMN for NRF registration.
	// If left empty they are inherited from hss.MCC / hss.MNC at startup.
	MCC           string          `yaml:"mcc"`
	MNC           string          `yaml:"mnc"`
	NRFAddress    string          `yaml:"nrf_address"`
	NFInstanceID  string          `yaml:"nf_instance_id"`
	TLSCertFile   string          `yaml:"tls_cert_file"`
	TLSKeyFile    string          `yaml:"tls_key_file"`
	OAuth2Enabled bool            `yaml:"oauth2_enabled"`
	OAuth2Bypass  bool            `yaml:"oauth2_bypass"`
	SBIClient     SBIClientConfig `yaml:"sbi_client"`
}

// GSUPConfig controls the Osmocom GSUP/HLR listener used by OsmoMSC/OsmoSGSN
// for 2G/3G circuit-switched authentication (SGs SMS and CSFB voice).
type GSUPConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_address"`
	BindPort    int    `yaml:"bind_port"`
}

type RoamingConfig struct {
	// AllowUndefinedNetworks controls the default behaviour when a subscriber
	// roams to a network that has no entry in the roaming_network table.
	// true  → allow roaming (only deny rules need to be configured)
	// false → deny roaming (allow rules must be configured per network)
	AllowUndefinedNetworks bool `yaml:"allow_undefined_networks"`
}

type HSSConfig struct {
	OriginHost                   string   `yaml:"OriginHost"`
	OriginRealm                  string   `yaml:"OriginRealm"`
	BindAddress                  string   `yaml:"BindAddress"`
	BindPort                     int      `yaml:"BindPort"`
	EnableSCTP                   bool     `yaml:"EnableSCTP"`   // listen on SCTP in addition to TCP
	DiameterDSCP                 int      `yaml:"DiameterDSCP"` // DSCP value for Diameter sockets, 0-63; 0 leaves best-effort/default marking
	DWRInterval                  int      `yaml:"DWRInterval"`
	CancelLocationRequestEnabled bool     `yaml:"CancelLocationRequest_Enabled"`
	AllowedPeers                 []string `yaml:"AllowedPeers"`
	MCC                          string   `yaml:"MCC"` // e.g. "311"
	MNC                          string   `yaml:"MNC"` // e.g. "435"
	// SCSCFPool is the list of S-CSCF URIs offered to the I-CSCF on first
	// registration (when the subscriber has no S-CSCF assigned yet).
	// Mirrors PyHSS scscf_pool. At least one entry is recommended.
	SCSCFPool []string `yaml:"scscf_pool"`
	// TLS — leave empty to disable. Future: also supported for API.
	TLSCertFile string `yaml:"TLSCertFile"`
	TLSKeyFile  string `yaml:"TLSKeyFile"`
}

type APIConfig struct {
	Enabled     bool   `yaml:"enabled"`
	BindAddress string `yaml:"bind_address"`
	BindPort    int    `yaml:"bind_port"`
	// TLS — leave empty to disable.
	TLSCertFile string `yaml:"tls_cert_file"`
	TLSKeyFile  string `yaml:"tls_key_file"`
	// API key authentication
	AuthEnabled bool     `yaml:"auth_enabled"`
	APIKeys     []string `yaml:"api_keys"`
}

type DatabaseConfig struct {
	Type            string `yaml:"db_type"`
	Host            string `yaml:"server"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	MaxOpenConns    int    `yaml:"pool_size"`
	MaxIdleConns    int    `yaml:"pool_idle"`
	ConnMaxLifetime int    `yaml:"pool_recycle"`
}

func (d DatabaseConfig) DSN() (string, error) {
	switch d.Type {
	case "postgresql", "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable client_encoding=UTF8",
			d.Host, d.Port, d.Username, d.Password, d.Database), nil
	case "sqlite":
		return d.Database, nil
	default:
		return "", fmt.Errorf("unsupported db_type %q", d.Type)
	}
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"` // path to log file; empty = no file output
}

type EIRConfig struct {
	NoMatchResponse int  `yaml:"no_match_response"`
	IMSIIMEILogging bool `yaml:"imsi_imei_logging"`
	// TACDBEnabled loads the GSMA Type Allocation Code device database into
	// memory at startup and enriches EIR history with device make/model.
	// NOTE: "TAC" here means IMEI Type Allocation Code, not the RAN
	// Tracking Area Code — they share an abbreviation but are unrelated.
	TACDBEnabled bool `yaml:"tac_db_enabled"`
}

type GeoredPeer struct {
	NodeID      string `yaml:"node_id"`
	Address     string `yaml:"address"`
	BearerToken string `yaml:"bearer_token"`
}

type GeoredConfig struct {
	Enabled               bool         `yaml:"enabled"`
	NodeID                string       `yaml:"node_id"`
	ListenPort            int          `yaml:"listen_port"`
	BearerToken           string       `yaml:"bearer_token"`
	TLSCertFile           string       `yaml:"tls_cert_file"`
	TLSKeyFile            string       `yaml:"tls_key_file"`
	SyncOAM               bool         `yaml:"sync_oam"`
	SyncState             bool         `yaml:"sync_state"`
	BatchMaxEvents        int          `yaml:"batch_max_events"`
	BatchMaxAgeMs         int          `yaml:"batch_max_age_ms"`
	QueueSize             int          `yaml:"queue_size"`
	PeriodicSyncIntervalS int          `yaml:"periodic_sync_interval_s"`
	Peers                 []GeoredPeer `yaml:"peers"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg := &Config{
		HSS:      HSSConfig{BindAddress: "0.0.0.0", BindPort: 3868, DWRInterval: 30},
		Database: DatabaseConfig{Port: 5432, MaxOpenConns: 30, MaxIdleConns: 10, ConnMaxLifetime: 300},
		Logging:  LoggingConfig{Level: "info"},
		EIR:      EIRConfig{NoMatchResponse: 2, IMSIIMEILogging: true, TACDBEnabled: true},
		Roaming:  RoamingConfig{AllowUndefinedNetworks: true},
		API:      APIConfig{Enabled: true, BindAddress: "0.0.0.0", BindPort: 8080},
		GSUP:     GSUPConfig{Enabled: false, BindAddress: "::", BindPort: 4222},
		UDM:      UDMConfig{Enabled: false, BindAddress: "::", BindPort: 7777, SBIClient: SBIClientConfig{Mode: "direct", ReconnectHoldoffTime: 2 * time.Second}},
		PCF:      PCFConfig{Enabled: false, BindAddress: "::", BindPort: 7778, SBIClient: SBIClientConfig{Mode: "direct", ReconnectHoldoffTime: 2 * time.Second}},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}
	normalizeFiveGC(cfg)
	if cfg.HSS.OriginHost == "" {
		return nil, fmt.Errorf("config: hss.OriginHost is required")
	}
	if cfg.HSS.OriginRealm == "" {
		return nil, fmt.Errorf("config: hss.OriginRealm is required")
	}
	if cfg.HSS.DiameterDSCP < 0 || cfg.HSS.DiameterDSCP > 63 {
		return nil, fmt.Errorf("config: hss.DiameterDSCP must be between 0 and 63")
	}
	if cfg.Database.Type == "" {
		return nil, fmt.Errorf("config: database.db_type is required")
	}
	return cfg, nil
}

func normalizeFiveGC(cfg *Config) {
	if len(cfg.UDM.SUCIDecryptionKeys) == 0 && len(cfg.UDM.LegacyHNetKeys) > 0 {
		cfg.UDM.SUCIDecryptionKeys = cfg.UDM.LegacyHNetKeys
	}

	if !fiveGCConfigured(cfg.FiveGC) {
		return
	}

	if cfg.FiveGC.SBI.BindAddress == "" {
		cfg.FiveGC.SBI.BindAddress = "::"
	}
	if cfg.FiveGC.SBI.BindPort == 0 {
		cfg.FiveGC.SBI.BindPort = 7777
	}
	if cfg.FiveGC.SBI.Client.Mode == "" {
		cfg.FiveGC.SBI.Client.Mode = "direct"
	}
	if cfg.FiveGC.SBI.Client.ReconnectHoldoffTime <= 0 {
		cfg.FiveGC.SBI.Client.ReconnectHoldoffTime = 2 * time.Second
	}
	if cfg.UDM.SBIClient.ReconnectHoldoffTime <= 0 {
		cfg.UDM.SBIClient.ReconnectHoldoffTime = 2 * time.Second
	}
	if cfg.PCF.SBIClient.ReconnectHoldoffTime <= 0 {
		cfg.PCF.SBIClient.ReconnectHoldoffTime = 2 * time.Second
	}

	cfg.UDM.Enabled = cfg.FiveGC.UDM.Enabled
	cfg.UDM.MCC = cfg.FiveGC.UDM.MCC
	cfg.UDM.MNC = cfg.FiveGC.UDM.MNC
	cfg.UDM.NRFAddress = cfg.FiveGC.UDM.NRFAddress
	cfg.UDM.NFInstanceID = cfg.FiveGC.UDM.NFInstanceID
	cfg.UDM.SUCIDecryptionKeys = cfg.FiveGC.UDM.SUCIDecryptionKeys

	cfg.PCF.Enabled = cfg.FiveGC.PCF.Enabled
	cfg.PCF.MCC = cfg.FiveGC.PCF.MCC
	cfg.PCF.MNC = cfg.FiveGC.PCF.MNC
	cfg.PCF.NRFAddress = cfg.FiveGC.PCF.NRFAddress
	cfg.PCF.NFInstanceID = cfg.FiveGC.PCF.NFInstanceID

	if cfg.FiveGC.SBI.BindAddress != "" {
		cfg.UDM.BindAddress = cfg.FiveGC.SBI.BindAddress
		cfg.PCF.BindAddress = cfg.FiveGC.SBI.BindAddress
	}
	if cfg.FiveGC.SBI.BindPort != 0 {
		cfg.UDM.BindPort = cfg.FiveGC.SBI.BindPort
		cfg.PCF.BindPort = cfg.FiveGC.SBI.BindPort
	}
	cfg.UDM.TLSCertFile = cfg.FiveGC.SBI.TLSCertFile
	cfg.UDM.TLSKeyFile = cfg.FiveGC.SBI.TLSKeyFile
	cfg.UDM.OAuth2Enabled = cfg.FiveGC.SBI.OAuth2Enabled
	cfg.UDM.OAuth2Bypass = cfg.FiveGC.SBI.OAuth2Bypass
	cfg.UDM.SBIClient = cfg.FiveGC.SBI.Client

	cfg.PCF.TLSCertFile = cfg.FiveGC.SBI.TLSCertFile
	cfg.PCF.TLSKeyFile = cfg.FiveGC.SBI.TLSKeyFile
	cfg.PCF.OAuth2Enabled = cfg.FiveGC.SBI.OAuth2Enabled
	cfg.PCF.OAuth2Bypass = cfg.FiveGC.SBI.OAuth2Bypass
	cfg.PCF.SBIClient = cfg.FiveGC.SBI.Client
}

func fiveGCConfigured(cfg FiveGCConfig) bool {
	return cfg.UDM.Enabled || cfg.PCF.Enabled ||
		cfg.SBI.BindAddress != "" || cfg.SBI.BindPort != 0 ||
		cfg.UDM.NRFAddress != "" || cfg.PCF.NRFAddress != "" ||
		cfg.UDM.NFInstanceID != "" || cfg.PCF.NFInstanceID != "" ||
		len(cfg.UDM.SUCIDecryptionKeys) > 0
}
