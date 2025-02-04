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

package common

import (
	"errors"

	vteCnf "github.com/czcorpus/vert-tagextract/v3/cnf"
	"github.com/czcorpus/vert-tagextract/v3/ptcount/modders"
)

var (
	ErrorPosNotDefined = errors.New("PoS not defined")
)

func appendPosModder(prev string, curr SupportedTagset) string {
	if prev == "" {
		return string(curr)
	}
	return prev + ":" + string(curr)
}

// posExtractorFactory creates a proper modders.StringTransformer instance
// to extract PoS in FRODO and also a string representation of it for proper
// vert-tagexract configuration.
func posExtractorFactory(
	currMods string,
	tagsetName SupportedTagset,
) (*modders.StringTransformerChain, string) {
	modderSpecif := appendPosModder(currMods, tagsetName)
	return modders.NewStringTransformerChain(modderSpecif), modderSpecif
}

// ApplyPosProperties takes posIdx and posTagset and adds a column modification
// function to conf.VertColumns[i] specified as holding data for "PoS".
// It still respects user-defined  one (preserving string modders
// already configured there!).
// In case posIdx argument points to a non-existing vertical column,
// the function returns errorPosNotDefined.
func ApplyPosProperties(
	conf *vteCnf.NgramConf,
	posIdx int,
	posTagset SupportedTagset,
) (*modders.StringTransformerChain, error) {
	for i, col := range conf.VertColumns {
		if posIdx == col.Idx {
			fn, modderSpecif := posExtractorFactory(col.ModFn, posTagset)
			col.ModFn = modderSpecif
			conf.VertColumns[i] = col
			return fn, nil
		}
	}
	return modders.NewStringTransformerChain(""), ErrorPosNotDefined
}

func GetFirstSupportedTagset(values []SupportedTagset) SupportedTagset {
	for _, v := range values {
		if v.Validate() == nil {
			return v
		}
	}
	return ""
}
