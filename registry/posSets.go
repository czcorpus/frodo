// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package registry

import (
	"encoding/json"
	"strings"
)

type PosItem struct {
	Label          string `json:"label"`
	TagSrchPattern string `json:"tagSrchPattern"`
}

type PosSimple struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Pos struct {
	ID          string
	Name        string
	Description string
	Values      []PosItem
}

func (p *Pos) ExportWposlist() string {
	ans := make([]string, len(p.Values)*2+1)
	ans[0] = ""
	for i, v := range p.Values {
		ans[2*i+1] = v.Label
		ans[2*i+2] = v.TagSrchPattern
	}
	return strings.Join(ans, ",")
}

func (p Pos) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Values      []PosItem `json:"values"`
		Wposlist    string    `json:"wposlist"`
	}{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Values:      p.Values,
		Wposlist:    p.ExportWposlist(),
	})
}

var (
	posList []Pos = []Pos{
		{
			ID:   "pp_tagset",
			Name: "Prague positional tagset",
			Values: []PosItem{
				{"podstatné jméno", "N.*"},
				{"přídavné jméno", "A.*"},
				{"zájmeno", "P.*"},
				{"číslovka", "C.*"},
				{"sloveso", "V.*"},
				{"příslovce", "D.*"},
				{"předložka", "R.*"},
				{"spojka", "J.*"},
				{"částice", "T.*"},
				{"citoslovce", "I.*"},
				{"interpunkce", "Z.*"},
				{"neznámý", "X.*"},
			},
		},
		{
			ID:   "bnc",
			Name: "BNC tagset",
			Values: []PosItem{
				{"adjective", "AJ."},
				{"adverb", "AV."},
				{"conjunction", "CJ."},
				{"determiner", "AT0"},
				{"noun", "NN."},
				{"noun singular", "NN1"},
				{"noun plural", "NN2"},
				{"preposition", "PR."},
				{"pronoun", "DPS"},
				{"verb", "VV."},
			},
		},
		{
			ID:   "rapcor",
			Name: "PoS from the Rapcor corpus",
			Values: []PosItem{
				{"adjective", "ADJ"},
				{"adverb", "ADV"},
				{"conjunction", "KON"},
				{"determiner", "DET.*"},
				{"interjection", "INT"},
				{"noun", "(NOM|NAM)"},
				{"numeral", "NUM"},
				{"preposition", "PRE.*"},
				{"pronoun", "PRO.*"},
				{"verb", "VER.*"},
				{"full stop", "SENT"},
			},
		},
	}
)
