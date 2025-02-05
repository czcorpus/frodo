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
	"encoding/json"
	"errors"
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs"
	"frodo/liveattrs/utils"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/fs"
	vteconf "github.com/czcorpus/vert-tagextract/v3/cnf"
	vtedb "github.com/czcorpus/vert-tagextract/v3/db"
)

var (
	ErrorNoSuchConfig = errors.New("no such configuration (corpus not installed)")
)

// Create creates a new live attributes extraction configuration based
// on provided args.
// note: bibIdAttr and mergeAttrs use dot notation (e.g. "doc.author")
func Create(
	conf *liveattrs.Conf,
	corpusInfo *corpus.Info,
	corpusDBInfo *corpus.DBInfo,
	jsonArgs *PatchArgs,
) (*vteconf.VTEConf, error) {
	maxNumErr := conf.VertMaxNumErrors
	if jsonArgs.MaxNumErrors != nil {
		maxNumErr = *jsonArgs.MaxNumErrors
	}
	newConf := vteconf.VTEConf{
		Corpus:              corpusInfo.ID,
		ParallelCorpus:      corpusDBInfo.ParallelCorpus,
		AtomParentStructure: "",
		StackStructEval:     false,
		MaxNumErrors:        maxNumErr,
		Ngrams:              jsonArgs.GetNgrams(),
		Encoding:            "UTF-8",
		IndexedCols:         []string{},
	}

	newConf.Structures = corpusInfo.RegistryConf.SubcorpAttrs
	if jsonArgs.BibView != nil {
		bibView := vtedb.BibViewConf{}
		bibView.IDAttr = utils.ImportKey(jsonArgs.BibView.IDAttr)
		for stru, attrs := range corpusInfo.RegistryConf.SubcorpAttrs {
			for _, attr := range attrs {
				bibView.Cols = append(bibView.Cols, fmt.Sprintf("%s_%s", stru, attr))
			}
		}
		newConf.BibView = bibView
		bibIdStruct, bibIdAttr := jsonArgs.BibView.IDAttrElements()
		tmp, ok := newConf.Structures[bibIdStruct]
		if ok {
			if !collections.SliceContains(tmp, bibIdAttr) {
				newConf.Structures[bibIdStruct] = append(newConf.Structures[bibIdStruct], bibIdAttr)
			}

		} else {
			newConf.Structures[bibIdStruct] = []string{bibIdAttr}
		}
	}
	if jsonArgs.AtomStructure == nil {
		if len(newConf.Structures) == 1 {
			for k := range newConf.Structures {
				newConf.AtomStructure = k
				break
			}
			log.Info().Msgf("no atomStructure, inferred value: %s", newConf.AtomStructure)

		} else {
			return nil, fmt.Errorf("no atomStructure specified and the value cannot be inferred due to multiple involved structures")
		}

	} else {
		newConf.AtomStructure = jsonArgs.GetAtomStructure()
	}
	atomExists := false
	for _, st := range corpusInfo.IndexedStructs {
		if st == newConf.AtomStructure {
			atomExists = true
			break
		}
	}
	if !atomExists {
		return nil, fmt.Errorf("atom structure '%s' does not exist in corpus %s", newConf.AtomStructure, corpusInfo.ID)
	}

	if jsonArgs.SelfJoin != nil {
		newConf.SelfJoin.ArgColumns = make([]string, len(jsonArgs.SelfJoin.ArgColumns))
		for i, argCol := range jsonArgs.SelfJoin.ArgColumns {
			tmp := strings.Split(argCol, "_")
			if len(tmp) != 2 {
				return nil, fmt.Errorf("invalid mergeAttr format (must be struct_attr): %s", argCol)
			}
			newConf.SelfJoin.ArgColumns[i] = tmp[0] + "_" + tmp[1]
			_, ok := newConf.Structures[tmp[0]]
			if ok {
				if !collections.SliceContains(newConf.Structures[tmp[0]], tmp[1]) {
					newConf.Structures[tmp[0]] = append(newConf.Structures[tmp[0]], tmp[1])
				}

			} else {
				newConf.Structures[tmp[0]] = []string{tmp[1]}
			}
		}
		newConf.SelfJoin.GeneratorFn = jsonArgs.SelfJoin.GeneratorFn
	}
	if conf.DB.Type == "mysql" {
		newConf.DB = vtedb.Conf{
			Type:           "mysql",
			Host:           conf.DB.Host,
			User:           conf.DB.User,
			Password:       conf.DB.Password,
			PreconfQueries: conf.DB.PreconfQueries,
		}
		if corpusDBInfo.ParallelCorpus != "" {
			newConf.DB.Name = corpusDBInfo.ParallelCorpus

		} else {
			newConf.DB.Name = corpusInfo.ID
		}

	} else {
		return nil, fmt.Errorf("Frodo service does not provide support for SQLite backend")
	}
	return &newConf, nil
}

// LiveAttrsBuildConfProvider is a loader and a cache for
// vert-tagextract configuration files.
// Please note that even if the stored config files contain
// credentials for liveattrs database, the
// LiveAttrsBuildConfProvider always rewrites the stored value
// with its own one (which is defined in Frodo configuration).
// So at least in theory - the stored vte config files should not
// deprecate.
type LiveAttrsBuildConfProvider struct {
	confDirPath  string
	globalDBConf *vtedb.Conf
	data         map[string]*vteconf.VTEConf
}

func (lcache *LiveAttrsBuildConfProvider) loadFromFile(corpname string, storeToCache bool) (*vteconf.VTEConf, error) {
	confPath := path.Join(lcache.confDirPath, corpname+".json")
	isFile, err := fs.IsFile(confPath)
	if err != nil {
		return nil, err
	}
	if isFile {
		v, err := LoadConf(confPath)
		if err != nil {
			return nil, err
		}
		if storeToCache {
			lcache.data[corpname] = v
		}
		if lcache.globalDBConf.Type == "mysql" {
			v.DB = *lcache.globalDBConf
		}
		return v, nil
	}
	return nil, ErrorNoSuchConfig
}

// Get returns an existing liveattrs configuration file. In case the
// file does not exist the method will not create it for you (as it
// requires additional arguments to determine specific properties).
// In case there is no other error but the configuration does not exist,
// the method returns ErrorNoSuchConfig error
func (lcache *LiveAttrsBuildConfProvider) Get(corpname string) (*vteconf.VTEConf, error) {
	if v, ok := lcache.data[corpname]; ok {
		return v, nil
	}
	return lcache.loadFromFile(corpname, true)
}

func (lcache *LiveAttrsBuildConfProvider) withRemovedSensitiveData(conf vteconf.VTEConf) vteconf.VTEConf {
	return conf.WithoutPasswords()
}

// GetWithoutPasswords is a variant of Get with passwords and similar stuff removed
func (lcache *LiveAttrsBuildConfProvider) GetWithoutPasswords(corpname string) (*vteconf.VTEConf, error) {
	entry, err := lcache.Get(corpname)
	if err != nil {
		return nil, err
	}
	ans := lcache.withRemovedSensitiveData(*entry)
	return &ans, nil
}

func (lcache *LiveAttrsBuildConfProvider) GetUncachedWithoutPasswords(corpname string) (*vteconf.VTEConf, error) {
	entry, err := lcache.loadFromFile(corpname, false)
	if err != nil {
		return nil, err
	}
	ans := lcache.withRemovedSensitiveData(*entry)
	return &ans, nil
}

// Save saves a provided configuration to a file for later use
func (lcache *LiveAttrsBuildConfProvider) Save(data *vteconf.VTEConf) error {
	rawData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	confPath := path.Join(lcache.confDirPath, data.Corpus+".json")
	err = os.WriteFile(confPath, rawData, 0777)
	if err != nil {
		return err
	}
	lcache.data[data.Corpus] = data
	if data.DB.Type == "mysql" {
		data.DB = *lcache.globalDBConf
	}
	return nil
}

// Uncache removes item corpusID from cache and returns true if the item
// was present. Otherwise does nothing and returns false.
func (lcache *LiveAttrsBuildConfProvider) Uncache(corpusID string) bool {
	_, ok := lcache.data[corpusID]
	delete(lcache.data, corpusID)
	return ok
}

// Clear removes a configuration from memory and from filesystem
func (lcache *LiveAttrsBuildConfProvider) Clear(corpusID string) error {
	delete(lcache.data, corpusID)
	confPath := path.Join(lcache.confDirPath, corpusID+".json")
	isFile, err := fs.IsFile(confPath)
	if err != nil {
		return err
	}
	if isFile {
		return os.Remove(confPath)
	}
	return nil
}

func NewLiveAttrsBuildConfProvider(confDirPath string, globalDBConf *vtedb.Conf) *LiveAttrsBuildConfProvider {
	return &LiveAttrsBuildConfProvider{
		confDirPath:  confDirPath,
		globalDBConf: globalDBConf,
		data:         make(map[string]*vteconf.VTEConf),
	}
}
