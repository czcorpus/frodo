// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
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

// frodojw (Frodo Job Watch) is a terminal UI for watching Frodo's
// asynchronous jobs (liveattrs, n-grams, keywords, ...).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	version   string
	buildDate string
	gitCommit string
)

// frodoConf contains just the parts of Frodo's configuration
// needed to locate its HTTP API.
type frodoConf struct {
	ListenAddress string `json:"listenAddress"`
	ListenPort    int    `json:"listenPort"`
}

func apiURLFromConf(path string) (string, error) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read config %s: %w", path, err)
	}
	var conf frodoConf
	if err := json.Unmarshal(rawData, &conf); err != nil {
		return "", fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	if conf.ListenPort == 0 {
		return "", fmt.Errorf("config %s does not specify listenPort", path)
	}
	host := conf.ListenAddress
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(conf.ListenPort)), nil
}

func main() {
	apiURL := flag.String("url", "", "Frodo API URL (e.g. http://localhost:8088); overrides the config file")
	interval := flag.Duration("interval", 2*time.Second, "refresh interval of the active view")
	showVersion := flag.Bool("version", false, "show version information and exit")
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			"FRODO Job Watch - terminal UI for watching Frodo's jobs\n\nUsage:\n\t%s [options] [config.json]\n\t%s -url http://host:port [options]\n\nOptions:\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]),
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("frodojw %s\nbuild date: %s\nlast commit: %s\n", version, buildDate, gitCommit)
		return
	}

	baseURL := *apiURL
	if baseURL == "" {
		if flag.NArg() == 0 {
			flag.Usage()
			os.Exit(2)
		}
		var err error
		baseURL, err = apiURLFromConf(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	}
	if *interval < 200*time.Millisecond {
		fmt.Fprintln(os.Stderr, "Error: refresh interval must be at least 200ms")
		os.Exit(2)
	}

	client := newAPIClient(baseURL)
	p := tea.NewProgram(newModel(client, *interval), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
