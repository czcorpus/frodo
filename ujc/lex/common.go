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

type LexID struct {
	ParentID   string `json:"parentId"`
	ID         string `json:"id"`
	GroupOrder int    `json:"groupOrder"`
	Homonym    int    `json:"homonym"`
	Pos        string `json:"pos"`
}

type LexKey struct {
	Lemma       string `json:"lemma"`
	Pos         string `json:"pos"`
	Gender      string `json:"gender"`
	Aspect      string `json:"aspect"`
	Uninflected bool   `json:"uninflected"`
	Plurality   int    `json:"plurality"`
}

func (a *LexKey) UnknownEqual(o *LexKey) bool {
	if a.Lemma != o.Lemma {
		return false
	}
	if a.Pos != o.Pos {
		return false
	}
	if a.Gender != o.Gender {
		return false
	}
	if a.Aspect != o.Aspect {
		return false
	}
	if a.Uninflected != o.Uninflected {
		return false
	}
	if a.Plurality != o.Plurality && a.Plurality != PluralityUnknown && o.Plurality != PluralityUnknown {
		return false
	}
	return true
}

type LexItem struct {
	Key       LexKey             `json:"key"`
	PosSource Source             `json:"posSource"`
	Sources   map[Source][]LexID `json:"sources"`
}

func (li *LexItem) Equal(item *LexItem) bool {
	return li.Key == item.Key
}

func (li *LexItem) HasSource(source Source) bool {
	_, ok := li.Sources[source]
	return ok
}

func hasVariant(key LexKey, variants []LexItem) bool {
	for _, variant := range variants {
		if variant.Key.UnknownEqual(&key) {
			return true
		}
	}
	return false
}
