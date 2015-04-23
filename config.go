package main

import (
	"errors"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type DBconfig struct {
	User     string
	DBname   string
	Port     string
	Password string
	Host     string
}

func GetConfig() (*DBconfig, error) {
	configFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		configFile, err = ioutil.ReadFile("/etc/wsnotify/config.yaml")
		if err != nil {
			return nil, errors.New("could not find config file")
		}
	}
	var config DBconfig
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func ConfigString(config *DBconfig) string {
	configString := "sslmode=disable"
	if config.User != "" {
		configString = configString + " user=" + config.User
	}
	if config.DBname != "" {
		configString = configString + " dbname=" + config.DBname
	}
	if config.Host != "" {
		configString = configString + " host=" + config.Host
	}
	if config.Password != "" {
		configString = configString + " password=" + config.Password
	}
	return configString
}
