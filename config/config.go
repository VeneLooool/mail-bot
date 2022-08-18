package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

type Config struct {
	Server struct {
		Port    string `yaml:"port"`
		Address string `yaml:"address"`
		Network string `yaml:"network"`
	} `yaml:"server"`
	TelegramBot struct {
		Token string `yaml:"token"`
	} `yaml:"telegramBot"`
	AvailableServices struct {
		MailNames []string `yaml:"mail_names"`
	} `yaml:"availableServices"`
	HTTPServer struct {
		Network string `yaml:"network"`
		Address string `yaml:"address"`
	} `yaml:"httpServer"`
}

func GetConfig() (config Config, err error) {
	yamlFile, err := os.ReadFile("./config/config.yaml")
	if err != nil {
		return Config{}, err
	}
	if err = yaml.Unmarshal(yamlFile, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config *Config) GetAddressPort() string {
	return config.Server.Address + ":" + config.Server.Port
}

func (config *Config) GetNetwork() string {
	return config.Server.Network
}

func (config *Config) GetMailServices() []string {
	return config.AvailableServices.MailNames
}

func (config *Config) GetTelegramToken() string {
	return config.TelegramBot.Token
}

func (config *Config) GetHttpNetwork() string {
	return config.HTTPServer.Network
}

func (config *Config) GetHttpAddress() string {
	return ":" + config.HTTPServer.Address
}
