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
	"frodo/jobs"
	"time"
)

// JobInfo collects information about corpus data synchronization job
type JobInfo struct {
	ID          string        `json:"id"`
	Type        string        `json:"type"`
	CorpusID    string        `json:"corpusId"`
	Start       jobs.JSONTime `json:"start"`
	Update      jobs.JSONTime `json:"update"`
	Finished    bool          `json:"finished"`
	Error       error         `json:"error,omitempty"`
	Result      *syncResponse `json:"result"`
	NumRestarts int           `json:"numRestarts"`
}

func (j JobInfo) GetID() string {
	return j.ID
}

func (j JobInfo) GetType() string {
	return j.Type
}

func (j JobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j JobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j JobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j JobInfo) IsFinished() bool {
	return j.Finished
}

func (j JobInfo) AsFinished() jobs.GeneralJobInfo {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
	return j
}

func (j JobInfo) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	if j.Error != nil || (j.Result != nil && !j.Result.OK) {
		item.OK = false
	}
	return item
}

func (j JobInfo) FullInfo() any {
	return struct {
		ID          string        `json:"id"`
		Type        string        `json:"type"`
		CorpusID    string        `json:"corpusId"`
		Start       jobs.JSONTime `json:"start"`
		Update      jobs.JSONTime `json:"update"`
		Finished    bool          `json:"finished"`
		Error       string        `json:"error,omitempty"`
		OK          bool          `json:"ok"`
		Result      *syncResponse `json:"result"`
		NumRestarts int           `json:"numRestarts"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       jobs.ErrorToString(j.Error),
		OK:          j.Error == nil,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}

func (j JobInfo) GetError() error {
	return j.Error
}

func (j JobInfo) WithError(err error) jobs.GeneralJobInfo {
	return JobInfo{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      jobs.JSONTime(time.Now()),
		Finished:    true,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}
