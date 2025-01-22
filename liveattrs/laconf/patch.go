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

package laconf

import (
	vteCnf "github.com/czcorpus/vert-tagextract/v3/cnf"
	vteDb "github.com/czcorpus/vert-tagextract/v3/db"
)

// PatchArgs is a subset of vert-tagextract's VTEConf
// used to overwrite stored liveattrs configs - either dynamically
// as part of some actions or to PATCH the config via Frodo's REST API.
//
// Please note that when using this type, there is an important distinction
// between an attribute being nil and being of a zero value. The former
// means: "do not update this item in the updated config",
// while the latter says:
// "reset a respective value to its zero value in the updated config"
// This allows us to selectively update different parts of "Liveattrs".
//
// To safely obtain non-nil/non-pointer values, you can use the provided getter methods
// which always replace nil values with respective zero values.
//
// Note: the most important self join functions are: "identity", "intecorp"
type PatchArgs struct {
	VerticalFiles []string            `json:"verticalFiles"`
	MaxNumErrors  *int                `json:"maxNumErrors"`
	AtomStructure *string             `json:"atomStructure"`
	SelfJoin      *vteDb.SelfJoinConf `json:"selfJoin"`
	BibView       *vteDb.BibViewConf  `json:"bibView"`
	Ngrams        *vteCnf.NgramConf   `json:"ngrams"`
}

func (la *PatchArgs) GetVerticalFiles() []string {
	if la.VerticalFiles == nil {
		return []string{}
	}
	return la.VerticalFiles
}

func (la *PatchArgs) GetMaxNumErrors() int {
	if la.MaxNumErrors == nil {
		return 0
	}
	return *la.MaxNumErrors
}

func (la *PatchArgs) GetAtomStructure() string {
	if la.AtomStructure == nil {
		return ""
	}
	return *la.AtomStructure
}

func (la *PatchArgs) GetSelfJoin() vteDb.SelfJoinConf {
	if la.SelfJoin == nil {
		return vteDb.SelfJoinConf{}
	}
	return *la.SelfJoin
}

func (la *PatchArgs) GetBibView() vteDb.BibViewConf {
	if la.BibView == nil {
		return vteDb.BibViewConf{}
	}
	return *la.BibView
}

func (la *PatchArgs) GetNgrams() vteCnf.NgramConf {
	if la.Ngrams == nil {
		return vteCnf.NgramConf{}
	}
	return *la.Ngrams
}
