// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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

package response

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

type ListedValue struct {
	ID         string
	Label      string
	ShortLabel string
	Count      int
	Grouping   int
}

type SummarizedValue struct {
	Length int `json:"length"`
}

type QueryAns struct {
	Poscount       int
	AttrValues     map[string]any
	AlignedCorpora []string
	AppliedCutoff  int
}

func (qa *QueryAns) MarshalJSON() ([]byte, error) {
	expAllAttrValues := make(map[string]any)
	for k, v := range qa.AttrValues {
		var attrValues any
		tv, ok := v.([]*ListedValue)
		if ok {
			tAttrValues := make([][5]any, 0, len(qa.AttrValues))
			for _, item := range tv {
				tAttrValues = append(
					tAttrValues,
					[5]any{
						item.ShortLabel,
						item.ID,
						item.Label,
						item.Grouping,
						item.Count,
					},
				)
			}
			attrValues = tAttrValues

		} else {
			attrValues = v
		}
		expAllAttrValues[k] = attrValues

	}
	return json.Marshal(&struct {
		Poscount       int            `json:"poscount"`
		AttrValues     map[string]any `json:"attr_values"`
		AlignedCorpora []string       `json:"aligned"`
		AppliedCutoff  int            `json:"applied_cutoff,omitempty"`
	}{
		Poscount:       qa.Poscount,
		AttrValues:     expAllAttrValues,
		AlignedCorpora: qa.AlignedCorpora,
		AppliedCutoff:  qa.AppliedCutoff,
	})
}

func (qa *QueryAns) AddListedValue(attr string, v *ListedValue) error {
	entry, ok := qa.AttrValues[attr]
	if !ok {
		return fmt.Errorf("failed to add listed value: attribute %s not found", attr)
	}
	tEntry, ok := entry.([]*ListedValue)
	if !ok {
		return fmt.Errorf("failed to add listed value: attribute %s not a list type", attr)
	}
	qa.AttrValues[attr] = append(tEntry, v)
	return nil
}

func (qa *QueryAns) CutoffValues(cutoff int) {
	var cutoffApplied bool
	for attr, items := range qa.AttrValues {
		tEntry, ok := items.([]*ListedValue)
		if ok && len(tEntry) > cutoff {
			qa.AttrValues[attr] = tEntry[:cutoff]
			cutoffApplied = true
		}
	}
	if cutoffApplied {
		qa.AppliedCutoff += cutoff
	}
}

func ExportAttrValues(
	data *QueryAns,
	alignedCorpora []string,
	expandAttrs []string,
	collatorLocale string,
	maxAttrListSize int,
) {
	values := make(map[string]any)
	for k, v := range data.AttrValues {
		switch tVal := v.(type) {
		case []*ListedValue:
			if maxAttrListSize == 0 || len(tVal) <= maxAttrListSize ||
				collections.SliceContains(expandAttrs, k) {
				sort.Slice(
					tVal,
					func(i, j int) bool {
						return strings.Compare(tVal[i].Label, tVal[j].Label) == -1
					},
				)
				values[k] = tVal

			} else {
				values[k] = SummarizedValue{Length: len(tVal)}
			}
		case int:
			values[k] = SummarizedValue{Length: tVal}
		default:
			values[k] = v
		}
	}
	data.AttrValues = values
}
