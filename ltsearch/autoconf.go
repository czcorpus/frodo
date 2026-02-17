// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Charles University, Faculty of Arts,
//                Department of Linguistics
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

package ltsearch

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/czcorpus/rexplorer/parser"
	"github.com/czcorpus/vert-tagextract/v3/livetokens"
)

func AutoConf(regPath string, attrs livetokens.AttrList) error {
	fdata, err := os.ReadFile(regPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	doc, err := parser.ParseRegistryBytes(filepath.Base(regPath), fdata)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	vertIdx := 0
	for _, posAttr := range doc.PosAttrs {
		for i := range attrs {
			if posAttr.Name == attrs[i].Name {
				attrs[i].VertIdx = vertIdx
				break
			}
		}
		if posAttr.Entries.Get("DYNAMIC").IsEmpty() {
			vertIdx++
		}
	}
	return nil
}
