// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMkTagKey(t *testing.T) {
	conf := &TagKeyConf{
		Default: []int{1, 2},
		ByPos:   map[string][]int{"N": {1, 3}, "V": {1, 2, 11, 13}},
	}
	assert.Equal(t, "N-F------------", conf.MkTagKey("NNFS1-----A----"))
	assert.Equal(t, "VB--------N-I--", conf.MkTagKey("VB-S---3P-NAI--"))
	assert.Equal(t, "AA-------------", conf.MkTagKey("AAFS1----1A----"))
	assert.Equal(t, "AA------------- N-F------------", conf.MkTagKey("AAFS1----1A---- NNFS1-----A----"))
	assert.Equal(t, "N-", conf.MkTagKey("NN"))
	assert.Equal(t, "", conf.MkTagKey(""))
}

func TestTagKeyConfValidate(t *testing.T) {
	assert.NoError(t, (*TagKeyConf)(nil).Validate())
	assert.NoError(t, (&TagKeyConf{ByPos: map[string][]int{"N": {1, 3}}}).Validate())
	assert.Error(t, (&TagKeyConf{Default: []int{0}}).Validate())
	assert.Error(t, (&TagKeyConf{ByPos: map[string][]int{"N": {0}}}).Validate())
	assert.Error(t, (&TagKeyConf{ByPos: map[string][]int{"NN": {1}}}).Validate())
}

func TestMergeTagKeys(t *testing.T) {
	assert.Equal(t, "N-F------------", MergeTagKeys("N-F------------", "N-F------------"))
	assert.Equal(t, "N--------------", MergeTagKeys("N-M------------", "N-I------------"))
	assert.Equal(t, "N-- N-F", MergeTagKeys("N-M N-F", "N-I N-F"))
	assert.Equal(t, "", MergeTagKeys("N-M", "N"))
}
