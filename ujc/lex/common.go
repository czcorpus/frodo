// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
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

package lex

import "frodo/dictionary"

type LexID struct {
	ParentID string `json:"parentId"`
	ID       string `json:"id"`
}

type LexItem struct {
	Lemma  string `json:"lemma"`
	Pos    string `json:"pos"`
	Gender string `json:"gender"`
	Aspect string `json:"aspect"`

	Sources     map[Source][]LexID `json:"sources"`
	CorpusEntry *dictionary.Lemma  `json:"corpusEntry"`

	relevanceScore int
}

type SearchResult struct {
	Matches []dictionary.Lemma `json:"matches"`
	Source  Source             `json:"source"`
	Items   []LexItem          `json:"items"`
}
