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
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/rs/zerolog/log"
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

// CorporaSetup defines Frodo application configuration related
// to a corpus
type CorporaSetup struct {
	RegistryDirPaths []string `json:"registryDirPaths"`
	RegistryTmpDir   string   `json:"registryTmpDir"`
	CorporaConfDir   string   `json:"confFilesDir"`
	corpora          []corp.CorpusSetup
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

func (cs *CorporaSetup) Load() error {
	files, err := os.ReadDir(cs.CorporaConfDir)
	if err != nil {
		return fmt.Errorf("failed to load corpora configs: %w", err)
	}
	for _, f := range files {
		confPath := filepath.Join(cs.CorporaConfDir, f.Name())
		tmp, err := os.ReadFile(confPath)
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", confPath).
				Msg("encountered invalid corpus configuration file, skipping")
			continue
		}
		var conf corp.CorpusSetup
		err = sonic.Unmarshal(tmp, &conf)
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", confPath).
				Msg("encountered invalid corpus configuration file, skipping")
			continue
		}
		cs.corpora = append(cs.corpora, conf)
		log.Info().Str("name", conf.ID).Msg("loaded corpus configuration file")
	}
	return nil
}

func (cs *CorporaSetup) Get(name string) corp.CorpusSetup {
	for _, v := range cs.corpora {
		if strings.Contains(v.ID, "*") {
			ptrn := regexp.MustCompile(strings.ReplaceAll(v.ID, "*", ".*"))
			if ptrn.MatchString(name) {
				if v.Variants != nil {
					variant, ok := v.Variants[name]
					if ok {
						// make a copy of CorpusSetup and replace values for specific variant
						merged := v
						merged.Variants = nil
						merged.ID = variant.ID
						if len(variant.FullName) > 0 {
							merged.FullName = variant.FullName
						}
						if len(variant.Description) > 0 {
							merged.Description = variant.Description
						}
						return merged
					}
				}
			}

		} else if v.ID == name {
			return v
		}
	}
	return corp.CorpusSetup{}
}

func (cs *CorporaSetup) GetAllCorpora() []corp.CorpusSetup {
	ans := make([]corp.CorpusSetup, 0, len(cs.corpora)*3)
	for _, v := range cs.corpora {
		if len(v.Variants) > 0 {
			for _, variant := range v.Variants {
				item := cs.Get(variant.ID)
				ans = append(ans, item)
			}

		} else {
			ans = append(ans, v)
		}
	}
	return ans
}

type DatabaseSetup struct {
	Host                     string `json:"host"`
	User                     string `json:"user"`
	Passwd                   string `json:"passwd"`
	Name                     string `json:"db"`
	OverrideCorporaTableName string `json:"overrideCorporaTableName"`
	OverridePCTableName      string `json:"overridePcTableName"`
}
