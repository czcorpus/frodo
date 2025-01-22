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
)

type IdxJobInfoArgs struct {
	MaxColumns int `json:"maxColumns"`
}

type idxJobResult struct {
	UsedIndexes    []string `json:"usedIndexes"`
	RemovedIndexes []string `json:"removedIndexed"`
}

// IdxUpdateJobInfo collects information about corpus data synchronization job
type IdxUpdateJobInfo struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	CorpusID    string         `json:"corpusId"`
	Start       jobs.JSONTime  `json:"start"`
	Update      jobs.JSONTime  `json:"update"`
	Finished    bool           `json:"finished"`
	Error       error          `json:"error,omitempty"`
	NumRestarts int            `json:"numRestarts"`
	Args        IdxJobInfoArgs `json:"args"`
	Result      idxJobResult   `json:"result"`
}

func (j IdxUpdateJobInfo) GetID() string {
	return j.ID
}

func (j IdxUpdateJobInfo) GetType() string {
	return j.Type
}

func (j IdxUpdateJobInfo) GetStartDT() jobs.JSONTime {
	return j.Start
}

func (j IdxUpdateJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j IdxUpdateJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j IdxUpdateJobInfo) AsFinished() jobs.GeneralJobInfo {
	j.Update = jobs.CurrentDatetime()
	j.Finished = true
	return j
}

func (j IdxUpdateJobInfo) IsFinished() bool {
	return j.Finished
}

func (j IdxUpdateJobInfo) FullInfo() any {
	return struct {
		ID          string         `json:"id"`
		Type        string         `json:"type"`
		CorpusID    string         `json:"corpusId"`
		Start       jobs.JSONTime  `json:"start"`
		Update      jobs.JSONTime  `json:"update"`
		Finished    bool           `json:"finished"`
		Error       string         `json:"error,omitempty"`
		OK          bool           `json:"ok"`
		NumRestarts int            `json:"numRestarts"`
		Args        IdxJobInfoArgs `json:"args"`
		Result      idxJobResult   `json:"result"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       jobs.ErrorToString(j.Error),
		OK:          j.Error == nil,
		NumRestarts: j.NumRestarts,
		Args:        j.Args,
		Result:      j.Result,
	}
}

func (j IdxUpdateJobInfo) CompactVersion() jobs.JobInfoCompact {
	item := jobs.JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	item.OK = j.Error == nil
	return item
}

func (j IdxUpdateJobInfo) GetError() error {
	return j.Error
}

func (j IdxUpdateJobInfo) WithError(err error) jobs.GeneralJobInfo {
	return IdxUpdateJobInfo{
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
