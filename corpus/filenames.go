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
	"fmt"
	"regexp"
)

var (
	icReg = regexp.MustCompile("intercorp_v([\\d]+)_\\w{2}")
)

// GenCorpusGroupName generates a proper name for corpus
// group name according to CNC's internal rules
// (e.g. intercorp_v11_en => intercorp_v11, foo => foo)
func GenCorpusGroupName(corpusID string) string {
	if v := icReg.FindStringSubmatch(corpusID); len(v) > 0 {
		return fmt.Sprintf("intercorp_v%s", v[1])
	}
	return corpusID
}

// IsIntercorpFilename tests whether the provided corpus identifier
// matches InterCorp naming patter.
func IsIntercorpFilename(corpusID string) bool {
	return icReg.MatchString(corpusID)
}
