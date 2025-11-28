// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Institute of the Czech National Corpus,
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

package main

import (
	"fmt"
	"frodo/corpus"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/vert-tagextract/v3/db"
	vtedb "github.com/czcorpus/vert-tagextract/v3/db"
)

type apiConf struct {
	BaseURL string `json:"baseUrl"`
}

type DictbuilderConfig struct {
	Logging           logging.LoggingConf `json:"logging"`
	Database          *vtedb.Conf         `json:"database"`
	API               apiConf             `json:"api"`
	NumOfLookbackDays int                 `json:"numOfLookbackDays"`
	VerticalDir       string              `json:"verticalDir"`
	Corpname          string              `json:"corpname"`
	CalcARF           bool                `json:"calcArf"`

	VertColumns *db.VertColumns `json:"vertColumns"`

	// AliasName is a final name for the dataset in case we
	// need a different name than the original corpus. This is typical
	// e.g. for WaG dictionaries which are derived from a single corpus
	// but have different properties (e.g. bigrams vs. unigrams etc.)
	//
	// Please note that if AliasName is non-zero, then TempCorpname
	// must be either of the same value or empty as otherwise, Frodo
	// would be confused what should be generated.
	//
	AliasName string `json:"aliasName"`

	// TempCorpname is used for "atomic" table updates. Frodo will
	// generate a dictionary to tables using TempCorpname and once
	// everything is done, it will drop existing "normal" tables
	// and renames temp ones to those dropped ones.
	// In case the dataset is an "alias" (which means it does not
	// represent the corpus directly, but it is rather derived for
	// other purposes), the renaming is not performed and these
	// "tmp" table names (= also AliasName tables, see AliasName)
	// are actually the final ones.
	TempCorpname string `json:"tmpCorpname"`

	// NGramSize speicifies how long word sequences we want
	// to include in our dictionary. For the cnc, this is typically
	// 1 or 2.
	NGramSize int `json:"ngramSize"`
}

func (dbconf *DictbuilderConfig) GetColMapping() *corpus.QSAttributes {
	ans := &corpus.QSAttributes{}
	var explicitPoS bool
	for _, v := range *dbconf.VertColumns {
		switch v.Role {
		case "word":
			ans.Word = v.Idx
		case "lemma":
			ans.Lemma = v.Idx
		case "sublemma":
			ans.Sublemma = v.Idx
		case "tag":
			ans.Tag = v.Idx
		case "pos":
			explicitPoS = true
			ans.Pos = v.Idx
		}
	}
	if !explicitPoS {
		ans.Pos = ans.Tag
	}
	return ans
}

func (dbconf *DictbuilderConfig) Validate() error {
	if dbconf.AliasName == "" && dbconf.TempCorpname == "" {
		return fmt.Errorf("both aliasName and tempCorpname are empty")
	}
	if dbconf.AliasName != "" && dbconf.TempCorpname != "" && dbconf.AliasName != dbconf.TempCorpname {
		return fmt.Errorf("aliasName and tempCorpname must be either same or the aliasName must be empty")
	}
	return nil
}

func (dbconf *DictbuilderConfig) IsAliasedDataset() bool {
	return dbconf.AliasName != ""
}

// GetDatasetName provides actual name of the dataset,
// no matter if it is a direct representation of a corpus
// (e.g. "liveattrs for  SYN2020") or some derived dataset
// (e.g. "a dictionary for some WaG instance").
func (dbconf *DictbuilderConfig) GetDatasetName() string {
	if dbconf.AliasName != "" {
		return dbconf.AliasName
	}
	return dbconf.TempCorpname
}

type JobStatus struct {
	ID       string `json:"id"`
	Finished bool   `json:"finished"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}
