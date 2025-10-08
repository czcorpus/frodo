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

package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/czcorpus/rexplorer/parser"
)

type DBInfo struct {
	Name              string
	Size              int64
	Active            int
	Locale            string
	HasLimitedVariant bool

	ParallelCorpus string

	// BibLabelAttr contains both structure and attribute (e.g. 'doc.id')
	BibLabelAttr string

	// BibIDAttr contains both structure and attribute (e.g. 'doc.id')
	BibIDAttr          string
	BibGroupDuplicates int
}

// GroupedName returns corpus name in a form compatible with storing multiple
// (aligned) corpora together in a single table. E.g. for InterCorp corpora
// this means stripping a language code suffix (e.g. intercorp_v13_en => intercorp_v13).
// For single corpora, this returns the original name.
func (info *DBInfo) GroupedName() string {
	if info.ParallelCorpus != "" {
		return info.ParallelCorpus
	}
	return info.Name
}

func GetRegistry(regPath string) (*parser.Document, error) {
	regBytes, err := os.ReadFile(regPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading registry file %s: %w", regPath, err)
	}
	doc, err := parser.ParseRegistryBytes(filepath.Base(regPath), regBytes)
	if err != nil {
		return nil, fmt.Errorf("failed parse registry file %s: %w", regPath, err)
	}
	return doc, nil
}

func getDirFilesSize(path string) (int64, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return -1, err
	}
	var total int64
	for _, v := range files {
		if !v.IsDir() {
			fstat, err := os.Stat(filepath.Join(path, v.Name()))
			if err != nil {
				return -1, fmt.Errorf("failed to get directory size: %w", err)
			}
			total += fstat.Size()

		}
	}
	return total, nil

}

func getCorpusDirInfo(regDoc *parser.Document) (*Data, error) {
	ans := new(Data)
	var err error
	regPath := regDoc.GetProperty("PATH").String()
	ans.SizeFiles, err = getDirFilesSize(regPath)
	if err != nil {
		errStr := err.Error()
		ans.Error = &errStr
		return ans, nil
	}
	dataDirPath := filepath.Clean(regPath)
	dataDirMtime, err := fs.GetFileMtime(dataDirPath)
	if err != nil {
		return nil, InfoError{err}
	}
	dataDirMtimeR := dataDirMtime.Format("2006-01-02T15:04:05-0700")
	isDir, err := fs.IsDir(dataDirPath)
	if err != nil {
		return nil, InfoError{err}
	}
	size, err := fs.FileSize(dataDirPath)
	if err != nil {
		return nil, InfoError{err}
	}
	ans.Path = FileMappedValue{
		Value:        dataDirPath,
		LastModified: &dataDirMtimeR,
		FileExists:   isDir,
		Size:         size,
	}
	return ans, nil
}

// GetCorpusInfo provides miscellaneous corpus installation information mostly
// related to different data files.
// It should return an error only in case Manatee or filesystem produces some
// error (i.e. not in case something is just not found).
func GetCorpusInfo(corpusID string, setup *CorporaSetup, tryLimited bool) (*Info, error) {
	ans := &Info{ID: corpusID}
	ans.IndexedData = IndexedData{}
	ans.RegistryConf = RegistryConf{Paths: make([]FileMappedValue, 0, 10)}
	ans.RegistryConf.SubcorpAttrs = make(map[string][]string)
	corpReg1 := setup.GetFirstValidRegistry(corpusID, CorpusVariantPrimary.SubDir())
	value, err := bindValueToPath(corpReg1, corpReg1)
	if err != nil {
		return nil, InfoError{err}
	}
	ans.RegistryConf.Paths = append(ans.RegistryConf.Paths, value)

	corp1, err := GetRegistry(corpReg1)
	if err != nil {
		return nil, InfoError{err}
	}

	ans.IndexedData.Primary, err = getCorpusDirInfo(corp1)
	if err != nil {
		return ans, fmt.Errorf("failed to get info about %s: %w", corpusID, err)
	}

	if tryLimited {
		corpReg2 := setup.GetFirstValidRegistry(corpusID, CorpusVariantLimited.SubDir())
		corp2, err := GetRegistry(corpReg2)
		if err != nil {
			return nil, InfoError{err}
		}
		ans.IndexedData.Limited, err = getCorpusDirInfo(corp2)
		if err != nil {
			return ans, fmt.Errorf("failed to get info about %s: %w", corpusID, err)
		}

	}

	// -------

	// get encoding
	ans.RegistryConf.Encoding = corp1.GetProperty("ENCODING").String()

	// parse SUBCORPATTRS
	subcorpAttrsString := corp1.GetProperty("SUBCORPATTRS").String()
	if subcorpAttrsString != "" {
		for _, attr1 := range strings.Split(subcorpAttrsString, "|") {
			for _, attr2 := range strings.Split(attr1, ",") {
				split := strings.Split(attr2, ".")
				ans.RegistryConf.SubcorpAttrs[split[0]] = append(ans.RegistryConf.SubcorpAttrs[split[0]], split[1])
			}
		}
	}

	unparsedStructs := corp1.GetProperty("STRUCTLIST").String()
	if unparsedStructs != "" {
		structs := strings.Split(unparsedStructs, ",")
		ans.IndexedStructs = make([]string, len(structs))
		copy(ans.IndexedStructs, structs)
	}

	// try registry's VERTICAL
	regVertical := corp1.GetProperty("VERTICAL").String()
	ans.RegistryConf.Vertical, err = bindValueToPath(regVertical, regVertical)
	if err != nil {
		return nil, InfoError{err}
	}

	return ans, nil
}
