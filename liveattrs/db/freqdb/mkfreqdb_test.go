// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
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

package freqdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineSimFreqsScore(t *testing.T) {

	items := []*ngRecord{
		{word: "foo1", lemma: "foo", tag: "A", arf: 1.0},
		{word: "foo2", lemma: "foo", tag: "A", arf: 2.0},
		{word: "foo3", lemma: "foo", tag: "A", arf: 3.0},
		{word: "foo2", lemma: "foo", tag: "B", arf: 10.0},
		{word: "foo4", lemma: "foo", tag: "B", arf: 20.0},
		{word: "bar1", lemma: "bar", tag: "B", arf: 100.0},
		{word: "bar2", lemma: "bar", tag: "B", arf: 200.0},
		{word: "bar3", lemma: "bar", tag: "B", arf: 300.0},
		{word: "baz1", lemma: "baz", tag: "A", arf: 1000.0},
		{word: "baz2", lemma: "baz", tag: "A", arf: 2000.0},
	}
	nfg := &NgramFreqGenerator{}
	nfg.determineSimFreqsScore(items)
	assert.InDelta(t, 6.0, items[0].simFreqsScore, 0.001)
	assert.InDelta(t, 6.0, items[1].simFreqsScore, 0.001)
	assert.InDelta(t, 6.0, items[2].simFreqsScore, 0.001)
	assert.InDelta(t, 30.0, items[3].simFreqsScore, 0.001)
	assert.InDelta(t, 30.0, items[4].simFreqsScore, 0.001)
	assert.InDelta(t, 600.0, items[5].simFreqsScore, 0.001)
	assert.InDelta(t, 600.0, items[6].simFreqsScore, 0.001)
	assert.InDelta(t, 600.0, items[7].simFreqsScore, 0.001)
	assert.InDelta(t, 3000.0, items[8].simFreqsScore, 0.001)
	assert.InDelta(t, 3000.0, items[9].simFreqsScore, 0.001)
}
