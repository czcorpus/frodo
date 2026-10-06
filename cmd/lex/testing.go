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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
)

// Test config structures
type testServerConf struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type testSpec struct {
	CorpusId string          `json:"corpusId"`
	Term     string          `json:"term"`
	Expected json.RawMessage `json:"expected"`
}

type testConfig struct {
	Server testServerConf `json:"server"`
	Tests  []testSpec     `json:"tests"`
}

// KV is a deterministic representation of a map entry used for canonicalization.
type KV struct {
	K string      `json:"k"`
	V interface{} `json:"v"`
}

func canonicalize(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case bool, string, float64:
		return t
	case []interface{}:
		elems := make([]interface{}, 0, len(t))
		for _, e := range t {
			elems = append(elems, canonicalize(e))
		}
		sort.SliceStable(elems, func(i, j int) bool {
			bi, _ := json.Marshal(elems[i])
			bj, _ := json.Marshal(elems[j])
			return bytes.Compare(bi, bj) < 0
		})
		return elems
	case map[string]interface{}:
		kvs := make([]KV, 0, len(t))
		for k, vv := range t {
			kvs = append(kvs, KV{K: k, V: canonicalize(vv)})
		}
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].K < kvs[j].K })
		out := make([]interface{}, 0, len(kvs))
		for _, kv := range kvs {
			out = append(out, kv)
		}
		return out
	default:
		rb, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("<unmarshalable:%T>", t)
		}
		var tmp interface{}
		if err := json.Unmarshal(rb, &tmp); err != nil {
			return fmt.Sprintf("<unmarshalable:%T>", t)
		}
		return canonicalize(tmp)
	}
}

func runTest(cfgPath string) error {
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
	for i, t := range cfg.Tests {
		u := fmt.Sprintf("%s/dictionary/lex/%s/search/%s", base, url.PathEscape(t.CorpusId), url.PathEscape(t.Term))
		resp, err := http.Get(u)
		if err != nil {
			fmt.Printf("[%d] ERROR requesting %s: %v\n", i, u, err)
			failed++
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("[%d] ERROR reading response: %v\n", i, err)
			failed++
			continue
		}

		// Unmarshal both expected and actual into interface{} for comparison
		var actual any
		if err := json.Unmarshal(body, &actual); err != nil {
			fmt.Printf("[%d] ERROR unmarshalling actual JSON: %v\nBody: %s\n", i, err, string(body))
			failed++
			continue
		}
		var expected any
		if len(t.Expected) > 0 {
			if err := json.Unmarshal(t.Expected, &expected); err != nil {
				fmt.Printf("[%d] ERROR unmarshalling expected JSON: %v\nExpected: %s\n", i, err, string(t.Expected))
				failed++
				continue
			}
		}

		canA := canonicalize(actual)
		canE := canonicalize(expected)

		if !reflect.DeepEqual(canA, canE) {
			fmt.Printf("[%d] FAIL term=%s corpus=%s\n", i, t.Term, t.CorpusId)
			fmt.Printf("URL: %s\n", u)
			expb, _ := json.MarshalIndent(canE, "", "  ")
			actb, _ := json.MarshalIndent(canA, "", "  ")
			fmt.Printf("Expected (canonicalized): %s\n", string(expb))
			fmt.Printf("Actual   (canonicalized): %s\n", string(actb))
			failed++
			continue
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
