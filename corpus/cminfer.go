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

package corpus

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/czcorpus/rexplorer/parser"
)

func InferQSAttrMapping(regPath string, tagset SupportedTagset) (QSAttributes, error) {
	ans := QSAttributes{
		Word:     -1,
		Sublemma: -1,
		Lemma:    -1,
		Tag:      -1,
		Pos:      -1,
	}
	regBytes, err := os.ReadFile(regPath)
	if err != nil {
		return ans, fmt.Errorf("failed to infer qs attribute mapping: %w", err)
	}
	doc, err := parser.ParseRegistryBytes(filepath.Base(regPath), regBytes)
	if err != nil {
		return ans, fmt.Errorf("failed to infer qs attribute mapping: %w", err)
	}
	var i int
	for _, attr := range doc.PosAttrs {
		if attr.GetProperty("DYNAMIC") == "" {
			switch attr.Name {
			case AttrWord:
				ans.Word = i
			case AttrSublemma:
				ans.Sublemma = i
			case AttrLemma:
				ans.Lemma = i
			case AttrTag:
				ans.Tag = i
			case AttrPos:
				ans.Pos = i
			}
			i++
		}
	}
	return ans, nil
}
