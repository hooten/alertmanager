// Copyright 2019 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file has been forked from https://github.com/prometheus/node_exporter/blob/master/https/tls_config.go
// Once tls_config.go is available in prometheus/common, alertmanager should vendor it in.

// Facilitates the implementation of TLS over TCP for gossip communications.

package config

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type TLSGossipConfig struct {
	TLSConfig TLSStruct `yaml:"tlsConfig"`
}

type TLSStruct struct {
	TLSCertPath        string `yaml:"tlsCertPath"`
	TLSKeyPath         string `yaml:"tlsKeyPath"`
	ClientAuth         string `yaml:"clientAuth"`
	ClientCAs          string `yaml:"clientCAs"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// GetTLSConfig unmarshals TLS configuration at the given configPath and returns a *tls.Config.
func GetTLSConfig(configPath string) (*tls.Config, error) {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	c := &TLSGossipConfig{}
	err = yaml.Unmarshal(content, c)
	if err != nil {
		return nil, err
	}
	return configToTLSConfig(&c.TLSConfig)
}

func configToTLSConfig(c *TLSStruct) (*tls.Config, error) {
	cfg := &tls.Config{}
	if len(c.TLSCertPath) == 0 {
		return nil, errors.New("missing TLSCertPath")
	}
	if len(c.TLSKeyPath) == 0 {
		return nil, errors.New("missing TLSKeyPath")
	}
	loadCert := func() (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(c.TLSCertPath, c.TLSKeyPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load X509KeyPair")
		}
		return &cert, nil
	}
	// Confirm that certificate and key paths are valid.
	if _, err := loadCert(); err != nil {
		return nil, err
	}
	cfg.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return loadCert()
	}

	cfg.InsecureSkipVerify = c.InsecureSkipVerify

	if len(c.ClientCAs) > 0 {
		clientCAPool := x509.NewCertPool()
		clientCAFile, err := ioutil.ReadFile(c.ClientCAs)
		if err != nil {
			return nil, err
		}
		clientCAPool.AppendCertsFromPEM(clientCAFile)
		cfg.ClientCAs = clientCAPool
	}
	if len(c.ClientAuth) > 0 {
		switch s := (c.ClientAuth); s {
		case "NoClientCert":
			cfg.ClientAuth = tls.NoClientCert
		case "RequestClientCert":
			cfg.ClientAuth = tls.RequestClientCert
		case "RequireClientCert":
			cfg.ClientAuth = tls.RequireAnyClientCert
		case "VerifyClientCertIfGiven":
			cfg.ClientAuth = tls.VerifyClientCertIfGiven
		case "RequireAndVerifyClientCert":
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		case "":
			cfg.ClientAuth = tls.NoClientCert
		default:
			return nil, errors.New("Invalid ClientAuth: " + s)
		}
	}
	if len(c.ClientCAs) > 0 && cfg.ClientAuth == tls.NoClientCert {
		return nil, errors.New("Client CA's have been configured without a Client Auth Policy")
	}
	return cfg, nil
}
