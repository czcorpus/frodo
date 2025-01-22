// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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

package jobs

import (
	"time"
)

type DummyJobResult struct {
	Payload string `json:"payload"`
}

// DummyJobInfo collects information about corpus data synchronization job
type DummyJobInfo struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	CorpusID    string          `json:"corpusId"`
	Start       JSONTime        `json:"start"`
	Update      JSONTime        `json:"update"`
	Finished    bool            `json:"finished"`
	Error       error           `json:"error,omitempty"`
	Result      *DummyJobResult `json:"result"`
	NumRestarts int             `json:"numRestarts"`
}

func (j DummyJobInfo) GetID() string {
	return j.ID
}

func (j DummyJobInfo) GetType() string {
	return j.Type
}

func (j DummyJobInfo) GetStartDT() JSONTime {
	return j.Start
}

func (j DummyJobInfo) GetNumRestarts() int {
	return j.NumRestarts
}

func (j DummyJobInfo) GetCorpus() string {
	return j.CorpusID
}

func (j DummyJobInfo) IsFinished() bool {
	return j.Finished
}

func (j DummyJobInfo) AsFinished() GeneralJobInfo {
	j.Update = CurrentDatetime()
	j.Finished = true
	return j
}

func (j DummyJobInfo) CompactVersion() JobInfoCompact {
	item := JobInfoCompact{
		ID:       j.ID,
		Type:     j.Type,
		CorpusID: j.CorpusID,
		Start:    j.Start,
		Update:   j.Update,
		Finished: j.Finished,
		OK:       true,
	}
	if j.Error != nil || (j.Result == nil) {
		item.OK = false
	}
	return item
}

func (j DummyJobInfo) FullInfo() any {
	return struct {
		ID          string          `json:"id"`
		Type        string          `json:"type"`
		CorpusID    string          `json:"corpusId"`
		Start       JSONTime        `json:"start"`
		Update      JSONTime        `json:"update"`
		Finished    bool            `json:"finished"`
		Error       string          `json:"error,omitempty"`
		OK          bool            `json:"ok"`
		Result      *DummyJobResult `json:"result"`
		NumRestarts int             `json:"numRestarts"`
	}{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      j.Update,
		Finished:    j.Finished,
		Error:       ErrorToString(j.Error),
		OK:          j.Error == nil,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}

func (j DummyJobInfo) GetError() error {
	return j.Error
}

func (j DummyJobInfo) WithError(err error) GeneralJobInfo {
	return DummyJobInfo{
		ID:          j.ID,
		Type:        j.Type,
		CorpusID:    j.CorpusID,
		Start:       j.Start,
		Update:      JSONTime(time.Now()),
		Finished:    true,
		Error:       err,
		Result:      j.Result,
		NumRestarts: j.NumRestarts,
	}
}
