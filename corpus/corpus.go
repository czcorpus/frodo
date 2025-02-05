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
	"errors"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

var (
	CorpusNotFound = errors.New("corpus not found")
)

// FileMappedValue is an abstraction of a configured file-related
// value where 'Value' represents the value to be inserted into
// some configuration and may or may not be actual file path.
type FileMappedValue struct {
	Value        string  `json:"value"`
	Path         string  `json:"-"`
	FileExists   bool    `json:"exists"`
	LastModified *string `json:"lastModified"`
	Size         int64   `json:"size"`
}

func (fmv FileMappedValue) VisiblePath() string {
	return fmv.Value
}

// RegistryConf wraps registry configuration related info
type RegistryConf struct {
	Paths        []FileMappedValue   `json:"paths"`
	Vertical     FileMappedValue     `json:"vertical"`
	Encoding     string              `json:"encoding"`
	SubcorpAttrs map[string][]string `json:"subcorpAttrs"`
}

// Data wraps information about indexed corpus data/files
type Data struct {
	SizeFiles int64           `json:"sizeFiles"`
	Path      FileMappedValue `json:"path"`
	Error     *string         `json:"error"`
}

type IndexedData struct {
	Primary *Data `json:"primary"`
	Limited *Data `json:"omezeni,omitempty"`
}

// Info wraps information about a corpus installation
type Info struct {
	ID             string       `json:"id"`
	IndexedData    IndexedData  `json:"indexedData"`
	IndexedStructs []string     `json:"indexedStructs"`
	RegistryConf   RegistryConf `json:"registry"`
}

// InfoError is a general corpus data information error.
type InfoError struct {
	error
}

type CorpusError struct {
	error
}

// bindValueToPath creates a new FileMappedValue instance
// using 'value' argument. Then it tests whether the
// 'path' exists and if so then it sets related properties
// (FileExists, LastModified, Size) to proper values
func bindValueToPath(value, path string) (FileMappedValue, error) {
	ans := FileMappedValue{Value: value, Path: path}
	isFile, err := fs.IsFile(path)
	if err != nil {
		return ans, err
	}
	if isFile {
		mTime, err := fs.GetFileMtime(path)
		if err != nil {
			return ans, err
		}
		mTimeString := mTime.Format("2006-01-02T15:04:05-0700")
		size, err := fs.FileSize(path)
		if err != nil {
			return ans, err
		}
		ans.FileExists = true
		ans.LastModified = &mTimeString
		ans.Size = size
	}
	return ans, nil
}

func FindVerticalFile(basePath, corpusID string) (FileMappedValue, error) {
	if basePath == "" {
		panic("FindVerticalFile error - basePath cannot be empty")
	}
	suffixes := []string{".tar.gz", ".tar.bz2", ".tgz", ".tbz2", ".7z", ".gz", ".zip", ".tar", ".rar", ""}
	var verticalPath string
	if IsIntercorpFilename(corpusID) {
		verticalPath = filepath.Join(basePath, GenCorpusGroupName(corpusID), corpusID)

	} else {
		verticalPath = filepath.Join(basePath, corpusID, "vertikala")
	}

	ans := FileMappedValue{Value: verticalPath}
	for _, suff := range suffixes {
		fullPath := verticalPath + suff
		if fs.PathExists(fullPath) {
			mTime, err := fs.GetFileMtime(fullPath)
			if err != nil {
				return ans, err
			}
			mTimeString := mTime.Format("2006-01-02T15:04:05-0700")
			size, err := fs.FileSize(fullPath)
			if err != nil {
				return ans, err
			}
			ans.LastModified = &mTimeString
			ans.Value = fullPath
			ans.Path = fullPath
			ans.FileExists = true
			ans.Size = size
			return ans, nil
		}
	}
	return ans, nil
}
