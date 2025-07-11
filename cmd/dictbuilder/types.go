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
	"github.com/czcorpus/cnc-gokit/logging"
	vtedb "github.com/czcorpus/vert-tagextract/v3/db"
)

type DictbuilderConfig struct {
	Logging  logging.LoggingConf `json:"logging"`
	Database *vtedb.Conf         `json:"database"`
	API      struct {
		BaseURL string `json:"baseUrl"`
	} `json:"api"`
	NumOfLookbackDays int    `json:"numOfLookbackDays"`
	VerticalDir       string `json:"verticalDir"`
	Corpname          string `json:"corpname"`
	TempCorpname      string `json:"tmpCorpname"`
	NGramSize         int    `json:"ngramSize"`
}

type JobStatus struct {
	ID       string `json:"id"`
	Finished bool   `json:"finished"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}
