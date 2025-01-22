// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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

package corpdata

import (
	"frodo/cnf"
	"frodo/general"
)

type registrySubdir struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
}

type registry struct {
	RootPaths []string `json:"rootPaths"`
}

type storageLocation struct {
	Data     string   `json:"data"`
	Registry registry `json:"registry"`
	Aligndef string   `json:"aligndef"`
}

// Actions contains all the fsops-related REST actions
type Actions struct {
	conf    *cnf.Conf
	version general.VersionInfo
}

// NewActions is the default factory
func NewActions(conf *cnf.Conf, version general.VersionInfo) *Actions {
	return &Actions{conf: conf, version: version}
}
