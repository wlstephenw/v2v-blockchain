package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Node        NodeConfig        `mapstructure:"node"`
	Network     NetworkConfig     `mapstructure:"network"`
	Blockchain  BlockchainConfig  `mapstructure:"blockchain"`
	Consensus   ConsensusConfig   `mapstructure:"consensus"`
	API         APIConfig         `mapstructure:"api"`
	Log         LogConfig         `mapstructure:"log"`
	Certificate CertificateConfig `mapstructure:"certificate"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
}

// NodeConfig holds node-specific configuration
type NodeConfig struct {
	ID          string `mapstructure:"id"`
	Role        string `mapstructure:"role"`
	DataDir     string `mapstructure:"data_dir"`
	ListenAddr  string `mapstructure:"listen_addr"`
}

// NetworkConfig holds P2P network configuration
type NetworkConfig struct {
	BootstrapPeers []string `mapstructure:"bootstrap_peers"`
	MaxConnections int      `mapstructure:"max_connections"`
	MinConnections int      `mapstructure:"min_connections"`
	DHTEnabled     bool     `mapstructure:"dht_enabled"`
	MDNSEnabled    bool     `mapstructure:"mdns_enabled"`
}

// BlockchainConfig holds blockchain configuration
type BlockchainConfig struct {
	GenesisBlock   bool   `mapstructure:"genesis_block"`
	BlockInterval  int    `mapstructure:"block_interval"`
	MaxTxPerBlock  int    `mapstructure:"max_tx_per_block"`
	MaxBlockSize   int    `mapstructure:"max_block_size"`
	LightClient    bool   `mapstructure:"light_client"`
}

// ConsensusConfig holds consensus configuration
type ConsensusConfig struct {
	Type          string `mapstructure:"type"`
	ViewTimeout   int    `mapstructure:"view_timeout"`
	BlockTimeout  int    `mapstructure:"block_timeout"`
	MinValidators int    `mapstructure:"min_validators"`
}

// APIConfig holds API server configuration
type APIConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	HTTPHost         string   `mapstructure:"http_host"`
	HTTPPort         int      `mapstructure:"http_port"`
	WebSocketEnabled bool     `mapstructure:"websocket_enabled"`
	CORSOrigins      []string `mapstructure:"cors_origins"`
	RateLimit        int      `mapstructure:"rate_limit"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// CertificateConfig holds certificate configuration
type CertificateConfig struct {
	CertFile         string `mapstructure:"cert_file"`
	KeyFile          string `mapstructure:"key_file"`
	CAFile           string `mapstructure:"ca_file"`
	AutoRotate       bool   `mapstructure:"auto_rotate"`
	RotateDaysBefore int    `mapstructure:"rotate_days_before"`
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	Type        string `mapstructure:"type"`
	Path        string `mapstructure:"path"`
	CacheSize   int    `mapstructure:"cache_size"`
	Compression bool   `mapstructure:"compression"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	PrometheusPort  int    `mapstructure:"prometheus_port"`
	Namespace       string `mapstructure:"namespace"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Set config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/v2v-blockchain/")
	}

	// Enable environment variables
	v.SetEnvPrefix("V2V")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Node defaults
	v.SetDefault("node.role", "follower")
	v.SetDefault("node.data_dir", "./data")
	v.SetDefault("node.listen_addr", "/ip4/0.0.0.0/tcp/10001")

	// Network defaults
	v.SetDefault("network.max_connections", 12)
	v.SetDefault("network.min_connections", 8)
	v.SetDefault("network.dht_enabled", true)
	v.SetDefault("network.mdns_enabled", true)

	// Blockchain defaults
	v.SetDefault("blockchain.genesis_block", true)
	v.SetDefault("blockchain.block_interval", 1000)
	v.SetDefault("blockchain.max_tx_per_block", 100)
	v.SetDefault("blockchain.max_block_size", 1048576)
	v.SetDefault("blockchain.light_client", false)

	// Consensus defaults
	v.SetDefault("consensus.type", "pbft")
	v.SetDefault("consensus.view_timeout", 10000)
	v.SetDefault("consensus.block_timeout", 5000)
	v.SetDefault("consensus.min_validators", 4)

	// API defaults
	v.SetDefault("api.enabled", true)
	v.SetDefault("api.http_host", "0.0.0.0")
	v.SetDefault("api.http_port", 8080)
	v.SetDefault("api.websocket_enabled", true)
	v.SetDefault("api.cors_origins", []string{"*"})
	v.SetDefault("api.rate_limit", 1000)

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.max_backups", 10)
	v.SetDefault("log.max_age", 30)

	// Certificate defaults
	v.SetDefault("certificate.auto_rotate", true)
	v.SetDefault("certificate.rotate_days_before", 7)

	// Storage defaults
	v.SetDefault("storage.type", "leveldb")
	v.SetDefault("storage.path", "./data/chaindb")
	v.SetDefault("storage.cache_size", 64)
	v.SetDefault("storage.compression", true)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.prometheus_port", 9090)
	v.SetDefault("metrics.namespace", "v2v_blockchain")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate node role
	validRoles := map[string]bool{"leader": true, "validator": true, "follower": true}
	if !validRoles[c.Node.Role] {
		return fmt.Errorf("invalid node role: %s", c.Node.Role)
	}

	// Validate consensus type
	validConsensus := map[string]bool{"pbft": true}
	if !validConsensus[c.Consensus.Type] {
		return fmt.Errorf("invalid consensus type: %s", c.Consensus.Type)
	}

	// Validate network connections
	if c.Network.MinConnections > c.Network.MaxConnections {
		return fmt.Errorf("min_connections cannot be greater than max_connections")
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}

	return nil
}

// GetHTTPAddr returns the HTTP server address
func (c *Config) GetHTTPAddr() string {
	return fmt.Sprintf("%s:%d", c.API.HTTPHost, c.API.HTTPPort)
}

// GetBlockInterval returns the block interval as time.Duration
func (c *Config) GetBlockInterval() time.Duration {
	return time.Duration(c.Blockchain.BlockInterval) * time.Millisecond
}

// GetViewTimeout returns the view timeout as time.Duration
func (c *Config) GetViewTimeout() time.Duration {
	return time.Duration(c.Consensus.ViewTimeout) * time.Millisecond
}
