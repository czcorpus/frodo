// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

type CorpusVariant string

type SupportedTagset string

const (
	CorpusVariantPrimary CorpusVariant = "primary"
	CorpusVariantLimited CorpusVariant = "omezeni"

	TagsetCSCNC2000SPK SupportedTagset = "cs_cnc2000_spk"
	TagsetCSCNC2000    SupportedTagset = "cs_cnc2000"
	TagsetCSCNC2020    SupportedTagset = "cs_cnc2020"
	TagsetUD           SupportedTagset = "ud"
)

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

func (cv CorpusVariant) SubDir() string {
	if cv == "primary" {
		return ""
	}
	return string(cv)
}

// CorporaDataPaths describes three
// different ways how paths to corpora
// data are specified:
// 1) CNC - a global storage path (typically slow but reliable)
// 2) Kontext - a special fast storage for KonText
// 3) abstract - a path for data consumers; points to either
// (1) or (2)
type CorporaDataPaths struct {
	Abstract string `json:"abstract"`
	CNC      string `json:"cnc"`
	Kontext  string `json:"kontext"`
}

// CorporaSetup defines Frodo application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPaths []string `json:"registryDirPaths"`
	RegistryTmpDir   string   `json:"registryTmpDir"`
}

func (cs *CorporaSetup) GetFirstValidRegistry(corpusID, subDir string) string {
	for _, dir := range cs.RegistryDirPaths {
		d := filepath.Join(dir, subDir, corpusID)
		pe := fs.PathExists(d)
		isf, _ := fs.IsFile(d)
		if pe && isf {
			return d
		}
	}
	return ""
}

type DatabaseSetup struct {
	Host                     string `json:"host"`
	User                     string `json:"user"`
	Passwd                   string `json:"passwd"`
	Name                     string `json:"db"`
	OverrideCorporaTableName string `json:"overrideCorporaTableName"`
	OverridePCTableName      string `json:"overridePcTableName"`
}
