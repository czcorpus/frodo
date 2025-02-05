// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Institute of the Czech National Corpus,
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

package freqdb

import (
	"fmt"
	"sort"
)

const (
	AttrWord     = "word"
	AttrSublemma = "sublemma"
	AttrLemma    = "lemma"
	AttrTag      = "tag"
	AttrPos      = "pos"
)

type QSAttributes struct {
	Word     int `json:"word"`
	Sublemma int `json:"sublemma"`
	Lemma    int `json:"lemma"`
	Tag      int `json:"tag"`
	Pos      int `json:"pos"`
}

func (qsa QSAttributes) String() string {
	return fmt.Sprintf("%#v", qsa)
}

func colIdxToName(idx int) string {
	return fmt.Sprintf("col%d", idx)
}

func (qsa QSAttributes) GetColIndexes() []int {
	ans := []int{
		qsa.Word,
		qsa.Lemma,
		qsa.Sublemma,
		qsa.Tag,
		qsa.Pos,
	}
	sort.SliceStable(ans, func(i, j int) bool {
		return ans[i] < ans[j]
	})
	return ans
}

// ExportCols exports columns based on their universal
// names "word", "lemma", "sublemma", "tag"
// So if e.g. Word == "col0", Lemma == "col3", Sublemma == "col5"
// and one requires ExportCols("word", "sublemma", "lemma", "sublemma")
// then the method returns []string{"col0", "col5", "col3", "col5"}
func (qsa QSAttributes) ExportCols(values ...string) []string {
	ans := make([]string, 0, len(values))
	for _, v := range values {
		switch v {
		case "word":
			ans = append(ans, colIdxToName(qsa.Word))
		case "lemma":
			ans = append(ans, colIdxToName(qsa.Lemma))
		case "sublemma":
			ans = append(ans, colIdxToName(qsa.Sublemma))
		case "tag":
			ans = append(ans, colIdxToName(qsa.Tag))
		case "pos":
			ans = append(ans, colIdxToName(qsa.Pos))
		default:
			panic(fmt.Sprintf("unknown query suggestion attribute: %s", v))
		}
	}
	return ans
}

func (qsa QSAttributes) ExportCol(name string) string {
	return qsa.ExportCols(name)[0]
}
