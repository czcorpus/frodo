// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
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

package corpus

import (
	"fmt"
	"strings"
)

// TagKeyConf specifies which (1-based) positions of a positional tag
// are kept to create a tag key - an additional identifier of words
// with the same lemma, sublemma and PoS (e.g. to distinguish them by gender).
// ByPos is keyed by PoS (= the first tag character), Default is used
// for other PoS values.
//
// Example: {"default": [1], "byPos": {"N": [1, 3], "V": [1, 13]}}
type TagKeyConf struct {
	Default []int            `json:"default"`
	ByPos   map[string][]int `json:"byPos"`
}

func (conf *TagKeyConf) Validate() error {
	if conf == nil {
		return nil
	}
	allPositions := [][]int{conf.Default}
	for pos, positions := range conf.ByPos {
		if len(pos) != 1 {
			return fmt.Errorf("invalid PoS %q in tag key configuration", pos)
		}
		allPositions = append(allPositions, positions)
	}
	for _, positions := range allPositions {
		for _, p := range positions {
			if p < 1 {
				return fmt.Errorf("invalid position %d in tag key configuration", p)
			}
		}
	}
	return nil
}

// MkTagKey creates a tag key from a tag. Positions not selected
// by the configuration are replaced by '-'. N-gram tags (space separated)
// are processed token by token.
func (conf *TagKeyConf) MkTagKey(tag string) string {
	tokens := strings.Split(tag, " ")
	for i, token := range tokens {
		if token == "" {
			continue
		}
		positions, ok := conf.ByPos[token[:1]]
		if !ok {
			positions = conf.Default
		}
		key := []byte(strings.Repeat("-", len(token)))
		for _, p := range positions {
			if p <= len(token) {
				key[p-1] = token[p-1]
			}
		}
		tokens[i] = string(key)
	}
	return strings.Join(tokens, " ")
}

// MergeTagKeys creates a tag key common to both tag keys,
// positions with different values are replaced by '-'.
// Tag keys of different lengths cannot be merged and empty string is returned.
func MergeTagKeys(tagKey1, tagKey2 string) string {
	if len(tagKey1) != len(tagKey2) {
		return ""
	}
	key := []byte(tagKey1)
	for i := range key {
		if key[i] != tagKey2[i] {
			key[i] = '-'
		}
	}
	return string(key)
}
