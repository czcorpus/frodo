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

package query

import (
	"fmt"
)

// Attrs represents a user selection of text types
// The values can be of different types. To handle them
// in a more convenient way, the type contains helper methods
// (GetRegexpAttrVal, GetListingOf).
type Attrs map[string]any

// GetRegexpAttrVal tries to extract value of a regular
// expression from Attrs under the 'attr' key. In case
// the type matches (i.e. there is a regexp value stored
// in q[attr]), a respective value is returned long with true.
// In any other case, false is returned as the second value.
func (q Attrs) GetRegexpAttrVal(attr string) (string, bool) {
	v, ok := q[attr]
	if !ok {
		v, ok = q["!"+attr]
		if !ok {
			return "", false
		}
	}
	tm, ok := v.(map[string]any)
	if ok && tm["regexp"] != "" {
		v := tm["regexp"]
		tv, ok := v.(string)
		if ok {
			return tv, true
		}
	}
	return "", false
}

// GetListingOf returns a list of strings (= selected values) for
// a specified attribute. In case the attribute is not represented
// by a value listing (like e.g. in case of range values), the function
// returns an error.
func (q Attrs) GetListingOf(attr string) ([]string, error) {
	v, ok := q[attr]
	if !ok {
		v, ok = q["!"+attr]
		if !ok {
			return []string{}, nil
		}
	}

	tv, ok := v.([]any)
	if !ok {
		tv, ok := v.(string)
		if ok {
			return []string{tv}, nil
		}
		return []string{}, fmt.Errorf("attribute %s does not contain value listing or string", attr)
	}
	ans := make([]string, len(tv))
	for i, v := range tv {
		tv, ok := v.(string)
		if ok {
			ans[i] = tv

		} else {
			// gracefully ignore typing problems here
			ans[i] = fmt.Sprintf("%v", v)
		}
	}
	return ans, nil
}

// Payload represents a query arguments as required by an HTTP API endpoint
type Payload struct {
	Aligned          []string `json:"aligned"`
	Attrs            Attrs    `json:"attrs"`
	AutocompleteAttr string   `json:"autocompleteAttr"`
	MaxAttrListSize  int      `json:"maxAttrListSize"`

	// ApplyCutoff, if set true, then in case a result returns more than MaxAttrListSize,
	// the list is cut to the MaxAttrListSize and the response is behaving like there
	// is no problem with too much matching items
	ApplyCutoff bool `json:"applyCutoff"`
}
