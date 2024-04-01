// Copyright 2024 Chris Bannister

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/zariel/dnist"
	"github.com/zariel/dnist/config"
	"gopkg.in/yaml.v3"
)

func werr(code int, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(code)
}

func main() {
	configPath := os.Getenv("DNIST_CONFIG")
	if configPath == "" {
		configPath = "./dnist.conf.yaml"
	}

	f, err := os.ReadFile(configPath)
	if err != nil {
		werr(2, "dnist: unable to load config: %v", err)
	}

	var conf config.Conf
	if err := yaml.Unmarshal(f, &conf); err != nil {
		werr(2, "dnist: unable to parse config: %v", err)
	}

	if err := dnist.Run(&conf); err != nil {
		werr(1, "dnist: %v", err)
	}
}
