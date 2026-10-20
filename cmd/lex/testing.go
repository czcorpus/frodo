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
	"path/filepath"
	"reflect"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

// Test config structures
type testServerConf struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type testConfig struct {
	Server       testServerConf    `json:"server"`
	CorpusID     string            `json:"corpusId"`
	Path         string            `json:"path"`
	Skip         []string          `json:"skip"`
	Descriptions map[string]string `json:"descriptions"`
}

func runVariantTest(cfgPath string, verbose bool) error {
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
	if cfg.Path == "" {
		return fmt.Errorf("no test path set")
	}
	testFiles, err := os.ReadDir(cfg.Path)
	if err != nil {
		return fmt.Errorf("cannot read path %q: %w", cfg.Path, err)
	}
	failed := 0
	skipped := 0

tests:
	for i, t := range testFiles {
		term := strings.TrimSuffix(t.Name(), ".json")
		description := cfg.Descriptions[term]
		if collections.SliceContains(cfg.Skip, term) {
			skipped++
			fmt.Printf("[%d] SKIP term=%s | %s\n", i, term, description)
			continue tests
		}

		u := fmt.Sprintf("%s/dictionary/lex/%s/search/%s", base, url.PathEscape(cfg.CorpusID), url.PathEscape(term))
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
				fmt.Printf("[%d] ERROR type mismatch in ExtraData for term=%s | %s: %T\n", i, term, description, match.ExtraData)
				failed++
				continue tests
			}
			var converted lex.LexExtraData
			b, err := json.Marshal(extra)
			if err != nil {
				fmt.Printf("[%d] ERROR marshaling ExtraData for term=%s | %s: %v\n", i, term, description, err)
				failed++
				continue tests
			}
			if err := json.Unmarshal(b, &converted); err != nil {
				fmt.Printf("[%d] ERROR unmarshalling ExtraData for term=%s | %s: %v\n", i, term, description, err)
				failed++
				continue tests
			}
			actualExtraData = append(actualExtraData, converted)
		}

		expectedFilePath := filepath.Join(cfg.Path, t.Name())
		expectedRaw, err := os.ReadFile(expectedFilePath)
		if err != nil {
			fmt.Printf("[%d] ERROR reading expected file %q: %v\n", i, expectedFilePath, err)
			failed++
			continue tests
		}
		var expectedExtraData []lex.LexExtraData
		if err := json.Unmarshal(expectedRaw, &expectedExtraData); err != nil {
			fmt.Printf("[%d] ERROR unmarshalling expected JSON from %q: %v\nBody: %s\n", i, expectedFilePath, err, string(expectedRaw))
			failed++
			continue tests
		}

		if !reflect.DeepEqual(expectedExtraData, actualExtraData) {
			fmt.Printf("[%d] FAIL term=%s | %s\n", i, term, description)
			expb, _ := json.MarshalIndent(expectedExtraData, "", "  ")
			actb, _ := json.MarshalIndent(actualExtraData, "", "  ")
			printDiff(string(expb), string(actb), verbose)
			failed++
			continue tests
		}
		fmt.Printf("[%d] PASS term=%s | %s\n", i, term, description)
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d tests failed (%d skipped)", failed, len(testFiles)-skipped, skipped)
	}
	fmt.Printf("All %d tests passed (%d skipped)\n", len(testFiles)-skipped, skipped)
	return nil
}

func saveVariantData(cfgPath string, term string) error {
	if cfgPath == "" {
		return fmt.Errorf("config path must be specified")
	}
	if term == "" {
		return fmt.Errorf("term must be specified")
	}

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
	if cfg.Path == "" {
		return fmt.Errorf("no test path set")
	}

	base := fmt.Sprintf("http://%s:%d", cfg.Server.Address, cfg.Server.Port)
	u := fmt.Sprintf("%s/dictionary/lex/%s/search/%s", base, url.PathEscape(cfg.CorpusID), url.PathEscape(term))
	resp, err := http.Get(u)
	if err != nil {
		return fmt.Errorf("cannot request %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request %s failed with status %d: %s", u, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cannot read response body: %w", err)
	}

	var actual lex.LexDictResponse
	if err := json.Unmarshal(body, &actual); err != nil {
		return fmt.Errorf("cannot unmarshal response: %w", err)
	}
	out := make([]lex.LexExtraData, 0, len(actual.Matches))
	for _, match := range actual.Matches {
		extra, ok := match.ExtraData.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected ExtraData type for term=%s: %T", term, match.ExtraData)
		}
		var converted lex.LexExtraData
		b, err := json.Marshal(extra)
		if err != nil {
			return fmt.Errorf("cannot marshal ExtraData for term=%s: %w", term, err)
		}
		if err := json.Unmarshal(b, &converted); err != nil {
			return fmt.Errorf("cannot unmarshal ExtraData for term=%s: %w", term, err)
		}
		out = append(out, converted)
	}

	outPath := filepath.Join(cfg.Path, term+".json")
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal expected data: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("cannot write file %q: %w", outPath, err)
	}
	fmt.Printf("Saved %d extra data records to %s\n", len(out), outPath)
	return nil
}

func bytesOrString(b json.RawMessage) string {
	if len(b) == 0 {
		return "null"
	}
	return string(b)
}

func printDiff(expected, actual string, verbose bool) {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")
	maxLines := max(len(actualLines), len(expectedLines))

	for i := range maxLines {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}
		if expLine == actLine {
			if verbose {
				fmt.Printf("  %03d: %s\n", i+1, expLine)
			}
			continue
		}
		if expLine != "" {
			fmt.Printf("- %03d: %s\n", i+1, expLine)
		}
		if actLine != "" {
			fmt.Printf("+ %03d: %s\n", i+1, actLine)
		}
	}
}
