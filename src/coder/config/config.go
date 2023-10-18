package config

type DERPServer struct {
	Enable        bool     `yaml:"enable"`
	RegionID      int      `yaml:"region_id"`
	RegionCode    string   `yaml:"region_code"`
	RegionName    string   `yaml:"region_name"`
	STUNAddresses []string `yaml:"stun_addresses"`
}

type DERPConfig struct {
	URL    string     `yaml:"url"`
	Path   string     `yaml:"path"`
	Server DERPServer `yaml:"server"`
}

var DefaultDERPConfig = DERPConfig{
	URL:  "",
	Path: "",
	Server: DERPServer{
		Enable:        true,
		RegionID:      999,
		RegionCode:    "gigo",
		RegionName:    "Gigo Embedded Relay",
		STUNAddresses: []string{"stun.l.google.com:19302"},
	},
}

func MergeDERPConfig(cfg DERPConfig) DERPConfig {
	merged := DefaultDERPConfig
	if cfg.URL != "" {
		merged.URL = cfg.URL
	}
	if cfg.Path != "" {
		merged.Path = cfg.Path
	}
	if !cfg.Server.Enable {
		merged.Server.Enable = cfg.Server.Enable
	}
	if cfg.Server.RegionID != 0 {
		merged.Server.RegionID = cfg.Server.RegionID
	}
	if cfg.Server.RegionCode != "" {
		merged.Server.RegionCode = cfg.Server.RegionCode
	}
	if cfg.Server.RegionName != "" {
		merged.Server.RegionName = cfg.Server.RegionName
	}
	if len(cfg.Server.STUNAddresses) > 0 {
		merged.Server.STUNAddresses = cfg.Server.STUNAddresses
	}
	return merged
}
