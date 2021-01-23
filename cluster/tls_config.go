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

	common "github.com/prometheus/common/config"
	"github.com/prometheus/exporter-toolkit/web"
	"gopkg.in/yaml.v2"
)

type TLSTransportConfig struct {
	TLSServerConfig *web.TLSStruct    `yaml:"tls_server_config"`
	TLSClientConfig *common.TLSConfig `yaml:"tls_client_config"`
}

func GetTLSTransportConfig(configPath string) (*TLSTransportConfig, error) {
	if configPath == "" {
		return nil, nil
	}
	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	config := &TLSTransportConfig{
		TLSServerConfig: &web.TLSStruct{
			MinVersion:               tls.VersionTLS12,
			MaxVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		},
	}
	if err := yaml.UnmarshalStrict(bytes, config); err != nil {
		return nil, err
	}
	config.TLSServerConfig.SetDirectory(filepath.Dir(configPath))
	config.TLSClientConfig.SetDirectory(filepath.Dir(configPath))
	return config, nil
}
