// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
// Faculty of Arts, Charles University
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

package main

import (
	"encoding/json"
	"fmt"
	"frodo/ujc/lex"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
)

// Test config structures
type testServerConf struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type testSpec struct {
	CorpusId         string             `json:"corpusId"`
	Term             string             `json:"term"`
	ExpectedVariants []lex.LexExtraData `json:"expectedVariants"`
}

type testConfig struct {
	Server testServerConf `json:"server"`
	Tests  []testSpec     `json:"tests"`
}

func runVariantTest(cfgPath string) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot read test config: %w", err)
	}
	var cfg testConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("cannot parse test config: %w", err)
	}
	if cfg.Server.Port == 0 {
		return fmt.Errorf("server.port must be specified in config")
	}

	base := fmt.Sprintf("http://%s:%d", cfg.Server.Address, cfg.Server.Port)
	var failed int

tests:
	for i, t := range cfg.Tests {
		u := fmt.Sprintf("%s/dictionary/lex/%s/search/%s", base, url.PathEscape(t.CorpusId), url.PathEscape(t.Term))
		resp, err := http.Get(u)
		if err != nil {
			fmt.Printf("[%d] ERROR requesting %s: %v\n", i, u, err)
			failed++
			continue tests
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("[%d] ERROR reading response: %v\n", i, err)
			failed++
			continue tests
		}

		// Unmarshal both expected and actual into interface{} for comparison
		var actual lex.LexDictResponse
		if err := json.Unmarshal(body, &actual); err != nil {
			fmt.Printf("[%d] ERROR unmarshalling actual JSON: %v\nBody: %s\n", i, err, string(body))
			failed++
			continue tests
		}

		actualExtraData := make([]lex.LexExtraData, 0, len(actual.Matches))
		for _, match := range actual.Matches {
			extra, ok := match.ExtraData.(map[string]any)
			if !ok {
				fmt.Printf("[%d] ERROR type mismatch in ExtraData for term=%s corpus=%s: %T\n", i, t.Term, t.CorpusId, match.ExtraData)
				failed++
				continue tests
			}
			var converted lex.LexExtraData
			b, err := json.Marshal(extra)
			if err != nil {
				fmt.Printf("[%d] ERROR marshaling ExtraData for term=%s corpus=%s: %v\n", i, t.Term, t.CorpusId, err)
				failed++
				continue tests
			}
			if err := json.Unmarshal(b, &converted); err != nil {
				fmt.Printf("[%d] ERROR unmarshalling ExtraData for term=%s corpus=%s: %v\n", i, t.Term, t.CorpusId, err)
				failed++
				continue tests
			}
			actualExtraData = append(actualExtraData, converted)
		}

		if !reflect.DeepEqual(t.ExpectedVariants, actualExtraData) {
			fmt.Printf("[%d] FAIL term=%s corpus=%s\n", i, t.Term, t.CorpusId)
			fmt.Printf("URL: %s\n", u)
			expb, _ := json.MarshalIndent(actualExtraData, "", "  ")
			actb, _ := json.MarshalIndent(t.ExpectedVariants, "", "  ")
			fmt.Printf("Expected: %s\n", string(expb))
			fmt.Printf("Actual: %s\n", string(actb))
			failed++
			continue tests
		}
		fmt.Printf("[%d] PASS term=%s corpus=%s\n", i, t.Term, t.CorpusId)
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d tests failed", failed, len(cfg.Tests))
	}
	fmt.Printf("All %d tests passed\n", len(cfg.Tests))
	return nil
}

func bytesOrString(b json.RawMessage) string {
	if len(b) == 0 {
		return "null"
	}
	return string(b)
}
