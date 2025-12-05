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

package liveattrs

import (
	"frodo/jobs"
	"time"

	"github.com/czcorpus/mquery-common/corp"
	vteCnf "github.com/czcorpus/vert-tagextract/v3/cnf"
)

const (
	JobType = "liveattrs"
)

type JobInfoArgs struct {
	Append           bool                 `json:"append"`
	VteConf          vteCnf.VTEConf       `json:"vteConf"`
	NoCorpusDBUpdate bool                 `json:"noCorpusDbUpdate"`
	TagsetAttr       string               `json:"tagsetAttr"`
	TagsetName       corp.SupportedTagset `json:"tagsetName"`
}

func (jargs JobInfoArgs) WithoutPasswords() JobInfoArgs {
	ans := jargs
	ans.VteConf = ans.VteConf.WithoutPasswords()
	return ans
}

// LiveAttrsJobInfo collects information about corpus data synchronization job
type LiveAttrsJobInfo struct {
	ID              string        `json:"id"`
	Type            string        `json:"type"`
	CorpusID        string        `json:"corpusId"`
	AliasedCorpusID string        `json:"aliasedCorpusId"`
	Start           jobs.JSONTime `json:"start"`
	Update          jobs.JSONTime `json:"update"`
	Finished        bool          `json:"finished"`
	Error           error         `json:"error,omitempty"`
	ProcessedAtoms  int           `json:"processedAtoms"`
	ProcessedLines  int           `json:"processedLines"`
	NumRestarts     int           `json:"numRestarts"`
	Args            JobInfoArgs   `json:"args"`
}

func (j LiveAttrsJobInfo) GetID() string {
	return j.ID
}

func (j LiveAttrsJobInfo) GetType() string {
	return j.Type
}

func (j LiveAttrsJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j LiveAttrsJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j LiveAttrsJobInfo) GetCorpus() string {
	if j.AliasedCorpusID == "" {
		return j.CorpusID
	}
	return j.AliasedCorpusID
}

func (j LiveAttrsJobInfo) GetDatasetID() string {
	return j.CorpusID
}

func (j LiveAttrsJobInfo) AsFinished() jobs.GeneralJobInfo {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
	return j
}

func (j LiveAttrsJobInfo) IsFinished() bool {
	return j.Finished
}

func (j LiveAttrsJobInfo) FullInfo() any {
	return struct {
		ID              string        `json:"id"`
		Type            string        `json:"type"`
		CorpusID        string        `json:"corpusId"`
		AliasedCorpusID string        `json:"aliasedCorpusId"`
		Start           jobs.JSONTime `json:"start"`
		Update          jobs.JSONTime `json:"update"`
		Finished        bool          `json:"finished"`
		Error           string        `json:"error,omitempty"`
		OK              bool          `json:"ok"`
		ProcessedAtoms  int           `json:"processedAtoms"`
		ProcessedLines  int           `json:"processedLines"`
		NumRestarts     int           `json:"numRestarts"`
		Args            JobInfoArgs   `json:"args"`
	}{
		ID:             j.ID,
		Type:           j.Type,
		CorpusID:       j.CorpusID,
		Start:          j.Start,
		Update:         j.Update,
		Finished:       j.Finished,
		Error:          jobs.ErrorToString(j.Error),
		OK:             j.Error == nil,
		ProcessedAtoms: j.ProcessedAtoms,
		ProcessedLines: j.ProcessedLines,
		NumRestarts:    j.NumRestarts,
		Args:           j.Args.WithoutPasswords(),
	}
}

func (j LiveAttrsJobInfo) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:              j.ID,
		Type:            j.Type,
		CorpusID:        j.CorpusID,
		AliasedCorpusID: j.AliasedCorpusID,
		Start:           j.Start,
		Update:          j.Update,
		Finished:        j.Finished,
		OK:              true,
	}
	item.OK = j.Error == nil
	return item
}

func (j LiveAttrsJobInfo) GetError() error {
	return j.Error
}

// WithError creates a new instance of LiveAttrsJobInfo with
// the Error property set to the value of 'err'.
func (j LiveAttrsJobInfo) WithError(err error) jobs.GeneralJobInfo {
	return LiveAttrsJobInfo{
		ID:              j.ID,
		Type:            JobType,
		CorpusID:        j.CorpusID,
		AliasedCorpusID: j.AliasedCorpusID,
		Start:           j.Start,
		Update:          jobs.JSONTime(time.Now()),
		Error:           err,
		NumRestarts:     j.NumRestarts,
		Args:            j.Args,
		Finished:        true,
	}
}
