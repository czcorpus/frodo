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

package lex

import (
	"frodo/dictionary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagKeySpecifier(t *testing.T) {
	assert.Equal(t, "F", tagKeySpecifier(dictionary.Lemma{PoS: "N", TagKey: "N-F------------"}))
	assert.Equal(t, "Y", tagKeySpecifier(dictionary.Lemma{PoS: "N", TagKey: "N-Y------------"}))
	assert.Equal(t, "P", tagKeySpecifier(dictionary.Lemma{PoS: "V", TagKey: "V-----------P--"}))
	assert.Equal(t, "", tagKeySpecifier(dictionary.Lemma{PoS: "N", TagKey: "N--------------"}))
	assert.Equal(t, "", tagKeySpecifier(dictionary.Lemma{PoS: "N", TagKey: "N"}))
	assert.Equal(t, "", tagKeySpecifier(dictionary.Lemma{PoS: "A", TagKey: "A-F------------"}))
}

func TestSetTagKeySpecifier(t *testing.T) {
	entry := dictionary.Lemma{PoS: "N"}
	setTagKeySpecifier(&entry, "F")
	assert.Equal(t, "N-F------------", entry.TagKey)
	setTagKeySpecifier(&entry, "")
	assert.Equal(t, "N--------------", entry.TagKey)

	entry = dictionary.Lemma{PoS: "V", TagKey: "V---------N----"}
	setTagKeySpecifier(&entry, "P")
	assert.Equal(t, "V---------N-P--", entry.TagKey)

	entry = dictionary.Lemma{PoS: "A", TagKey: "A"}
	setTagKeySpecifier(&entry, "F")
	assert.Equal(t, "A", entry.TagKey)
}

func TestSpecifiersMatch(t *testing.T) {
	assert.True(t, specifiersMatch("", "F"))
	assert.True(t, specifiersMatch("F", "F"))
	assert.True(t, specifiersMatch("Y", "I"))
	assert.True(t, specifiersMatch("M", "Y"))
	assert.False(t, specifiersMatch("F", "M"))
	assert.False(t, specifiersMatch("Y", "F"))
}

func TestMergeSpecifiers(t *testing.T) {
	assert.Equal(t, "F", mergeSpecifiers("F", "F"))
	assert.Equal(t, "Y", mergeSpecifiers("M", "I"))
	assert.Equal(t, "Y", mergeSpecifiers("Y", "I"))
	assert.Equal(t, "", mergeSpecifiers("M", "F"))
	assert.Equal(t, "", mergeSpecifiers("P", "I"))
	assert.Equal(t, "", mergeSpecifiers("", "M"))
}

func TestCorpusLexKey(t *testing.T) {
	assert.Equal(t, LexKey{Lemma: "pes", Pos: "N", Gender: "M"}, corpusLexKey("pes", dictionary.Lemma{PoS: "N", TagKey: "N-M------------"}))
	assert.Equal(t, LexKey{Lemma: "pes", Pos: "N", Gender: GenderMascUnknown}, corpusLexKey("pes", dictionary.Lemma{PoS: "N", TagKey: "N-Y------------"}))
	assert.Equal(t, LexKey{Lemma: "dělat", Pos: "V", Aspect: "I"}, corpusLexKey("dělat", dictionary.Lemma{PoS: "V", TagKey: "V-----------I--"}))
}

func TestMergeTagKeyVariants(t *testing.T) {
	ans := mergeTagKeyVariants([]dictionary.Lemma{
		{
			Lemma: "pes", PoS: "N", TagKey: "N-M------------",
			Forms:     []dictionary.Form{{Value: "pes", Sublemma: "pes", Count: 10}},
			Sublemmas: []dictionary.Sublemma{{Value: "pes", Count: 10}},
			Count:     10, SimFreqScore: 5,
		},
		{Lemma: "pes", PoS: "A", TagKey: "A--------------", Count: 1},
		{
			Lemma: "pes", PoS: "N", TagKey: "N-I------------",
			Forms: []dictionary.Form{
				{Value: "pes", Sublemma: "pes", Count: 2},
				{Value: "psy", Sublemma: "pes", Count: 1},
			},
			Sublemmas: []dictionary.Sublemma{{Value: "pes", Count: 3}},
			Count:     3, SimFreqScore: 1,
		},
	})
	assert.Len(t, ans, 2)
	assert.Equal(t, "N-Y------------", ans[0].TagKey)
	assert.Equal(t, 13, ans[0].Count)
	assert.InDelta(t, 6.0, ans[0].SimFreqScore, 0.001)
	assert.ElementsMatch(t, []dictionary.Form{
		{Value: "pes", Sublemma: "pes", Count: 12},
		{Value: "psy", Sublemma: "pes", Count: 1},
	}, ans[0].Forms)
	assert.Equal(t, []dictionary.Sublemma{{Value: "pes", Count: 13}}, ans[0].Sublemmas)
	assert.Equal(t, "A", ans[1].PoS)
}
