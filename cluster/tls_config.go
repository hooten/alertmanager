// Copyright 2020 The Prometheus Authors
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

package cluster

import (
	"crypto/tls"
	"io/ioutil"
	"path/filepath"

	"github.com/prometheus/exporter-toolkit/https"
	"gopkg.in/yaml.v2"
)

func getConfig(configPath string) (*https.Config, error) {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	c := &https.Config{
		TLSConfig: https.TLSStruct{
			MinVersion:               tls.VersionTLS12,
			MaxVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		},
		HTTPConfig: https.HTTPStruct{HTTP2: true},
	}
	err = yaml.UnmarshalStrict(content, c)
	c.TLSConfig.SetDirectory(filepath.Dir(configPath))
	return c, err
}

func GetTLSConfig(configPath string) (*tls.Config, error) {
	if configPath == "" {
		return nil, nil
	}
	c, err := getConfig(configPath)
	if err != nil {
		return nil, err
	}
	return https.ConfigToTLSConfig(&c.TLSConfig)
}
