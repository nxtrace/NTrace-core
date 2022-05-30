package config

import "os"

type tracerConfig struct {
    Token `yaml:"Token"`
    Preference `yaml:"Preference"`
}

type Token struct {
	LeoMoeAPI string `yaml:"LeoMoeAPI"`
	IPInfo    string `yaml:"IPInfo"`
}

type Preference struct {
    AlwaysRoutePath bool `yaml:"AlwaysRoutePath"`
}

type configPath func() (string, error)

func configFromRunDir() (string, error) {
    return "./", nil
}

func configFromUserHomeDir() (string, error) {
    dir, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return dir + "/.nexttrace/", nil
}