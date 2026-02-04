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

package ssjc

import "frodo/dictionary"

type SrcFileRow struct {
	ParentID     string
	ID           string
	Headword     string
	HeadwordType string
	Pos          string
	Gender       string
	Aspect       string
}

type SubHeadWord struct {
	Headword    string           `json:"headword"`
	Pos         string           `json:"pos"`
	Gender      string           `json:"gender"`
	Aspect      string           `json:"aspect"`
	CorpusEntry dictionary.Lemma `json:"corpusEntry"`
}

type HeadWordEntry struct {
	ID          string           `json:"id"`
	Headword    string           `json:"headword"`
	Pos         string           `json:"pos"`
	Gender      string           `json:"gender"`
	Aspect      string           `json:"aspect"`
	CorpusEntry dictionary.Lemma `json:"corpusEntry"`
	Children    []SubHeadWord    `json:"children"`
}

func (e HeadWordEntry) IsZero() bool {
	return e.ID == ""
}
