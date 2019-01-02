package config

import (
	"os"
)

// Config represents application configuration
type Config struct {
	// HTTPPort HTTP port
	HTTPPort string
	// HTTPSPort HTTP port
	HTTPSPort string
	// TargetNamespace is the targeted namespace to watch for ingress rules
	TargetNamespace string
	// KubeConfig is used for development purposes
	KubeConfig string
	// LogVerbose log verbose
	LogVerbose bool
	// LogInfo log info
	LogInfo bool
	// LogWarning log warning
	LogWarning bool
}

// FromEnv loads config from environment variables
func FromEnv() (*Config, error) {
	conf := &Config{
		HTTPPort:   "80",
		HTTPSPort:  "443",
		LogVerbose: false,
		LogInfo:    true,
		LogWarning: true,
	}

	conf.TargetNamespace = os.Getenv("TARGET_NAMESPACE")
	if os.Getenv("KUBECONFIG") != "" {
		conf.KubeConfig = os.Getenv("KUBECONFIG")
	}
	if os.Getenv("LOG_LEVEL") == "verbose" {
		conf.LogVerbose = true
	}
	if os.Getenv("LOG_LEVEL") == "warning" {
		conf.LogVerbose = false
		conf.LogInfo = false
	}

	return conf, nil
}
