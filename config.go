package main

import (
	"errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	Command string
	Port    int
	Timeout int
	User    string
}

func readConfig(file string) (Config, error) {
	configFile, err := ioutil.ReadFile(file)
	if err != nil {
		return Config{}, err
	}

	config := Config{Timeout: -1}
	err = yaml.Unmarshal([]byte(configFile), &config)
	if err != nil {
		return Config{}, err
	}

	if config.Command == "" {
		return Config{}, errors.New("Command not specified.")
	} else if config.Port == 0 {
		return Config{}, errors.New("Port not specified.")
	} else if config.User == "" {
		return Config{}, errors.New("User not specified.")
	}

	return config, nil
}
