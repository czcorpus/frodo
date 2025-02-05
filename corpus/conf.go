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
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

const (
	CorpusVariantPrimary CorpusVariant = "primary"
	CorpusVariantLimited CorpusVariant = "omezeni"
)

type CorpusVariant string

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
