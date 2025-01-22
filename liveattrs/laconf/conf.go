// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
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

package laconf

import (
	"fmt"

	vteconf "github.com/czcorpus/vert-tagextract/v3/cnf"
)

func GetSubcorpAttrs(vteConf *vteconf.VTEConf) []string {
	ans := make([]string, 0, 50)
	for strct, attrs := range vteConf.Structures {
		for _, attr := range attrs {
			ans = append(ans, fmt.Sprintf("%s.%s", strct, attr))
		}
	}
	return ans
}

func LoadConf(path string) (*vteconf.VTEConf, error) {
	return vteconf.LoadConf(path)
}
