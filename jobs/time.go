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

package jobs

import (
	"encoding/json"
	"time"
)

// JSONTime is a customized time data type with predefined
// JSON serialization
type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal(nil)
	}
	return []byte("\"" + time.Time(t).Format(time.RFC3339) + "\""), nil
}

func (t JSONTime) Before(t2 JSONTime) bool {
	return time.Time(t).Before(time.Time(t2))
}

func (t JSONTime) Sub(t2 JSONTime) time.Duration {
	return time.Time(t).Sub(time.Time(t2))
}

func (t JSONTime) Format(layout string) string {
	return time.Time(t).Format(layout)
}

func (t JSONTime) IsZero() bool {
	return time.Time(t).IsZero()
}

func (t JSONTime) GobEncode() ([]byte, error) {
	return time.Time(t).MarshalBinary()
}

func (t *JSONTime) GobDecode(data []byte) error {
	v := time.Time(*t)
	return v.UnmarshalBinary(data)
}

func CurrentDatetime() JSONTime {
	return JSONTime(time.Now())
}
