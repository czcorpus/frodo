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

package common

import (
	"fmt"
	"frodo/liveattrs/db/freqdb"
	"os"
	"path/filepath"

	"github.com/czcorpus/rexplorer/parser"
)

type SupportedTagset string

// Validate tests whether the value is one of known types.
// Please note that the empty value is also considered OK
// (otherwise we wouldn't have a valid zero value)
func (st SupportedTagset) Validate() error {
	if st == TagsetCSCNC2000SPK ||
		st == TagsetCSCNC2000 ||
		st == TagsetCSCNC2020 ||
		st == TagsetUD ||
		st == "" {
		return nil
	}
	return fmt.Errorf("invalid tagset type: %s", st)
}

func (st SupportedTagset) String() string {
	return string(st)
}

const (
	TagsetCSCNC2000SPK SupportedTagset = "cs_cnc2000_spk"
	TagsetCSCNC2000    SupportedTagset = "cs_cnc2000"
	TagsetCSCNC2020    SupportedTagset = "cs_cnc2020"
	TagsetUD           SupportedTagset = "ud"
)

func InferQSAttrMapping(regPath string, tagset SupportedTagset) (freqdb.QSAttributes, error) {
	ans := freqdb.QSAttributes{
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
			case freqdb.AttrWord:
				ans.Word = i
			case freqdb.AttrSublemma:
				ans.Sublemma = i
			case freqdb.AttrLemma:
				ans.Lemma = i
			case freqdb.AttrTag:
				ans.Tag = i
			case freqdb.AttrPos:
				ans.Pos = i
			}
			i++
		}
	}
	return ans, nil
}
