package config

import (
    "io/ioutil"
    "log"
    "gopkg.in/yaml.v2"
)

func (c *tracerConfig) Parse(data []byte) error {
    return yaml.Unmarshal(data, c)
}

func readFile(cp configPath) ([]byte, error) {
    var content []byte
    path, err := cp()
    if err != nil {
        log.Println(err)
        return nil, err
    }
    content, err = ioutil.ReadFile(path + "ntraceConfig.yml")
    if err != nil {
        return nil, err
    }
    return content, nil
}

func Read() (*tracerConfig, error) {
    var data []byte
    var err  error
    
    data, err = readFile(configFromRunDir)

    if err != nil {
        data, err = readFile(configFromUserHomeDir)

        if err != nil {
            return nil, err
        }
    }

    var config tracerConfig
    if err := config.Parse(data); err != nil {
        return nil, err
    }
    
    return &config, err
}