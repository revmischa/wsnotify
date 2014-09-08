package main

import (
	"errors"
	"io/ioutil"

	"launchpad.net/goyaml"
)

type DBconfig struct {
	User        string
	DBname      string
	Port        string
	Password    string
	Host        string
	SSLCertPath string
	SSLKeyPath  string
	ServerPath  string
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
	err = goyaml.Unmarshal(configFile, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func ConfigString(config *DBconfig) string {
	configString := "user=" + config.User + " dbname=" + config.DBname + " sslmode=disable"
	if config.Host != "" {
		configString = configString + " host=" + config.Host
	}
	if config.Password != "" {
		configString = configString + " password=" + config.Password
	}
	return configString
}
