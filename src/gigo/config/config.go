package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gage-technologies/gigo-lib/config"
	"gopkg.in/yaml.v2"
)

type HttpServerConfig struct {
	Hostname                     string              `yaml:"hostname"`
	Domain                       string              `yaml:"domain"`
	Address                      string              `yaml:"address"`
	Port                         string              `yaml:"port"`
	DevelopmentMode              bool                `yaml:"development_mode"`
	LoggerConfig                 config.LoggerConfig `yaml:"logger_config"`
	HostSite                     string              `yaml:"host_site"`
	UseTLS                       bool                `yaml:"use_tls"`
	BasePathHTTP                 string              `yaml:"base_path_http"`
	GitWebhookSecret             string              `yaml:"git_webhook_secret"`
	StripeWebhookSecret          string              `yaml:"stripe_webhook_secret"`
	StripeConnectedWebhookSecret string              `yaml:"stripe_connected_webhook_secret"`
	StableDiffusionHost          string              `yaml:"stable_diffusion_host"`
	StableDiffusionKey           string              `yaml:"stable_diffusion_key"`
	MailGunApiKey                string              `yaml:"mailgun_api_key"`
	MailGunDomain                string              `yaml:"mailgun_domain"`
	MailGunVerificationKey       string              `yaml:"mailgun_verification_key"`
	GigoEmail                    string              `yaml:"gigo_email"`
	InitialRecommendationURl     string              `yaml:"initial_recommendation_url"`
	ForceCdnAccess               bool                `yaml:"force_cdn_access"`
	CdnAccessKey                 string              `yaml:"cdn_access_key"`
	WhitelistedIpRanges          []string            `yaml:"whitelisted_ip_ranges"`
	CuratedSecret                string              `yaml:"curated_secret"`
}

type WorkspaceProvisionerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type OTELConfig struct {
	ServiceName string `yaml:"service_name"`
	EndPoint    string `yaml:"otel_exporter_otlp_endpoint"`
	Insecure    bool   `yaml:"insecure_mode"`
}

type RegistryCacheConfig struct {
	Source string `yaml:"source"`
	Cache  string `yaml:"cache"`
}

type StripeSubscriptionConfig struct {
	MonthlyPriceID string `yaml:"monthly_price_id"`
	YearlyPriceID  string `yaml:"yearly_price_id"`
}

type BackupConfig struct {
	Enabled            bool                     `yaml:"enabled"`
	PD                 string                   `yaml:"pd"`
	Stores             []config.StorageS3Config `yaml:"stores"`
	MaxBackupAgeHours  int                      `yaml:"max_backup_age_hours"`
	MinRetainedBackups int                      `yaml:"min_retained_backups"`
}

type Config struct {
	Cluster            bool                         `yaml:"cluster"`
	StorageConfig      config.StorageConfig         `yaml:"storage_config"`
	TitaniumConfig     config.TitaniumConfig        `yaml:"ti_config"`
	MeiliConfig        config.MeiliConfig           `yaml:"meili_config"`
	ESConfig           config.ElasticConfig         `yaml:"elastic_config"`
	EtcdConfig         config.EtcdConfig            `yaml:"etcd_config"`
	MasterKey          string                       `yaml:"master_key"`
	CaptchaSecret      string                       `yaml:"captcha_secret"`
	JetstreamConfig    config.JetstreamConfig       `yaml:"jetstream_config"`
	HTTPServerConfig   HttpServerConfig             `yaml:"http_server_config"`
	LoggerID           string                       `yaml:"logger_id"`
	RedisConfig        config.RedisConfig           `yaml:"redis_config"`
	GiteaConfig        config.GiteaConfig           `yaml:"gitea_config"`
	WsConfig           []WorkspaceProvisionerConfig `yaml:"ws_config"`
	NodeID             int64                        `yaml:"node_id"`
	StripeKey          string                       `yaml:"stripe_key"`
	AccessUrl          string                       `yaml:"access_url"`
	DerpMeshKey        string                       `yaml:"derp_mesh_key"`
	OTELConfig         OTELConfig                   `yaml:"otel_config"`
	GithubSecret       string                       `yaml:"github_secret"`
	RegistryCaches     []RegistryCacheConfig        `yaml:"registry_caches"`
	ZitiConfig         config.ZitiConfig            `yaml:"ziti_config"`
	StripeSubscription StripeSubscriptionConfig     `yaml:"stripe_subscription_config"`
	BackupConfig       BackupConfig                 `yaml:"backup_config"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file contents: %v", err)
	}

	var cfg Config
	err = yaml.Unmarshal(b, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}

	return &cfg, nil
}
