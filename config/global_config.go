package config

import (
	"os"
	"path/filepath"
	"strings"

	"proofread-exhibition/pkg/util"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type IConfig interface {
	Validate() []error
}

type GlobalConfig struct {
	Port            int    `json:"port,omitempty" yaml:"port,omitempty"`
	ProofreadApiUrl string `json:"proofreadApiUrl,omitempty" yaml:"proofreadApiUrl,omitempty"`
}

func (g *GlobalConfig) Validate() []error {
	var errs = make([]error, 0)
	if err := util.IsValidPort(g.Port); err != nil {
		errs = append(errs, err)
	}
	return errs
}

func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Port:            3000,
		ProofreadApiUrl: "127.0.0.1:8700",
	}
}
func TryLoadFromDisk(configFilePath string) (*GlobalConfig, error) {
	_, err := os.Stat(configFilePath)
	if err != nil {
		return nil, err
	}
	dir, file := filepath.Split(configFilePath)
	fileType := filepath.Ext(file)
	viper.AddConfigPath(dir)
	viper.SetConfigName(strings.TrimSuffix(file, fileType))
	viper.SetConfigType(strings.TrimPrefix(fileType, "."))
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return nil, err
		}
	}
	cfg := NewDefaultGlobalConfig()
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
