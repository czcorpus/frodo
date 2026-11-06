// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
// Faculty of Arts, Charles University
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
	"cmp"
	"context"
	"fmt"
	"frodo/dictionary"
	"slices"
	"strings"

	"github.com/czcorpus/cnc-gokit/util"
	"github.com/rs/zerolog/log"
)

func (actions *Actions) searchCorpusEntry(ctx context.Context, corpusId, lemma, sublemma, pos string) (*dictionary.Lemma, error) {
	if sublemma == "" {
		return nil, nil
	}

	datasetSize, err := actions.dictActions.GetDatasetSize(corpusId)
	if err != nil {
		return nil, err
	}

	ans, err := dictionary.Search(
		ctx,
		actions.db,
		corpusId,
		dictionary.SearchWithSublemma(sublemma),
		util.Ternary(lemma != "", dictionary.SearchWithLemma(lemma), dictionary.SearchWithNoOp()),
		util.Ternary(pos != "", dictionary.SearchWithPoS(pos), dictionary.SearchWithNoOp()),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find sublemma: %w", err)
	}
	if len(ans) > 0 {
		if len(ans) > 1 {
			variantInfo := make([]string, 0, len(ans))
			for _, item := range ans {
				variantInfo = append(variantInfo, fmt.Sprintf("%s/%s", item.Lemma, item.PoS))
			}
			log.Warn().
				Str("sublemma", sublemma).
				Str("pos", pos).
				Strs("variants", variantInfo).
				Int("numMatches", len(ans)).
				Msg("Multiple matches found for sublemma in corpus")
			entry := ans[0]
			for _, item := range ans[1:] {
				entry.PoS += item.PoS
				entry.Specifier += item.Specifier
			}
			return &entry, nil
		}
		return &ans[0], nil
	}
	return nil, nil
}

func (actions *Actions) searchOtherSublemmata(ctx context.Context, corpusId, lemma, sublemma, pos string) ([]dictionary.Lemma, error) {
	if sublemma == "" {
		return nil, nil
	}

	datasetSize, err := actions.dictActions.GetDatasetSize(corpusId)
	if err != nil {
		return nil, err
	}

	ans, err := dictionary.Search(
		ctx,
		actions.db,
		corpusId,
		util.Ternary(lemma != "", dictionary.SearchWithLemma(lemma), dictionary.SearchWithNoOp()),
		dictionary.SearchWithoutSublemma(sublemma),
		util.Ternary(pos != "", dictionary.SearchWithPoS(pos), dictionary.SearchWithNoOp()),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find sublemma: %w", err)
	}

	// expand results to have one entry per sublemma, with forms filtered to only those matching the sublemma
	results := make([]dictionary.Lemma, 0, len(ans))
	for _, item := range ans {
		for _, sublemma := range item.Sublemmas {
			entry := item
			entry.Sublemmas = []dictionary.Sublemma{sublemma}
			if len(entry.Forms) > 0 {
				filteredForms := make([]dictionary.Form, 0, len(entry.Forms))
				for _, form := range entry.Forms {
					if form.Sublemma == "" || form.Sublemma == sublemma.Value {
						filteredForms = append(filteredForms, form)
					}
				}
				entry.Forms = filteredForms
			}
			results = append(results, entry)
		}
	}

	return results, nil
}

func (actions *Actions) mergeMultiWordCorpusEntries(item LexItem, corpusEntries []*dictionary.Lemma) *dictionary.Lemma {
	mergedSublemmas := make([]string, 0, len(corpusEntries))
	mergedLemmas := make([]string, 0, len(corpusEntries))
	mergedPos := make([]string, 0, len(corpusEntries))
	mergedSpecifiers := make([]string, 0, len(corpusEntries))
	allCorp := true
	for _, subentry := range corpusEntries {
		if allCorp && !strings.HasPrefix(subentry.ID, "corp") {
			allCorp = false
		}
		mergedLemmas = append(mergedLemmas, subentry.Lemma)
		mergedPos = append(mergedPos, subentry.PoS)
		mergedSpecifiers = append(mergedSpecifiers, subentry.Specifier)
		if len(subentry.Sublemmas) > 0 && subentry.Sublemmas[0].Value != "" {
			mergedSublemmas = append(mergedSublemmas, subentry.Sublemmas[0].Value)
		} else if subentry.Lemma != "" {
			mergedSublemmas = append(mergedSublemmas, subentry.Lemma)
		}
	}
	mergedLemmaText := strings.Join(mergedLemmas, " ")
	mergedSublemmaText := strings.Join(mergedSublemmas, " ")
	return &dictionary.Lemma{
		ID:        util.Ternary(allCorp, "corp", "lex"),
		Lemma:     mergedLemmaText,
		PoS:       strings.Join(mergedPos, " "),
		Specifier: strings.Join(mergedSpecifiers, " "),
		Forms:     []dictionary.Form{{Value: item.Key.Lemma, Sublemma: mergedSublemmaText}},
		Sublemmas: []dictionary.Sublemma{{Value: mergedSublemmaText}},
	}
}

// lemmatizeLexItem finds lemma and sublemma of the lexicon lemma,
// lexicon lemmatization is preferred, corpus is used as a fallback
func (actions *Actions) lemmatizeLexItem(ctx context.Context, corpusId, lemma string) (LemmatizedTerm, error) {
	lemmatized, err := LemmatizeTermWithLex(ctx, actions.db.DB(), lemma)
	if err != nil {
		return LemmatizedTerm{}, err
	}
	if lemmatized != nil {
		return *lemmatized, nil
	}
	lemmatizedTerms, err := LemmatizeTermWithCorpus(ctx, actions.db.DB(), corpusId, lemma)
	if err != nil {
		return LemmatizedTerm{}, err
	}
	if len(lemmatizedTerms) > 0 {
		return lemmatizedTerms[0], nil
	}
	return LemmatizedTerm{Term: lemma, Lemma: lemma, Sublemma: lemma}, nil
}

// findCorpusEntry finds corpus entry for the lex item. Multi-word terms are searched
// word by word and merged into a single entry. Words not found in corpus are replaced
// with minimal entries created from lexicon data.
func (actions *Actions) findCorpusEntry(ctx context.Context, corpusId string, item LexItem, lemmatized LemmatizedTerm) (*dictionary.Lemma, error) {
	multiWord := lemmatized.isMultiWord()
	specifier := util.Ternary(multiWord, "", corpusSpecifier(item.Key))
	// use lemmatized term only for multi-word terms, otherwise use the lemma from lexicon
	lemmas := strings.Fields(util.Ternary(multiWord, lemmatized.Lemma, item.Key.Lemma))
	sublemmas := strings.Fields(util.Ternary(multiWord, lemmatized.Sublemma, item.Key.Lemma))
	pos := util.Ternary(multiWord, "", item.Key.Pos)

	if len(sublemmas) == 0 {
		return newLexEntry(item.Key.Lemma, pos, specifier), nil
	}

	entries := make([]*dictionary.Lemma, 0, len(sublemmas))
	for i, sublemma := range sublemmas {
		lemma := sublemma
		if i < len(lemmas) {
			lemma = lemmas[i]
		}
		entry, err := actions.searchCorpusEntry(ctx, corpusId, lemma, sublemma, pos)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			entry = newLexEntry(sublemma, pos, specifier)
		} else {
			entry.ID = "corp"
			entry.Specifier = cmp.Or(entry.Specifier, specifier)
		}
		entries = append(entries, entry)
	}

	if len(entries) == 1 {
		return entries[0], nil
	}
	return actions.mergeMultiWordCorpusEntries(item, entries), nil
}

// corpusSpecifier converts lexicon gender/aspect to corpus specifier,
// corpus uses "MI" for both "B" and "U" masculine genders
func corpusSpecifier(key LexKey) string {
	if key.Gender == GenderMascAnimInan || key.Gender == GenderMascUnknown {
		return "MI"
	}
	return cmp.Or(key.Gender, key.Aspect)
}

// newLexEntry creates minimal entry for a term not found in corpus
func newLexEntry(value, pos, specifier string) *dictionary.Lemma {
	return &dictionary.Lemma{
		ID:        "lex",
		Lemma:     value,
		PoS:       pos,
		Specifier: specifier,
		Forms:     []dictionary.Form{{Value: value, Sublemma: value}},
		Sublemmas: []dictionary.Sublemma{{Value: value}},
	}
}

// attachCorpusSource adds the corpus entry as a source of the lex item
func attachCorpusSource(item *LexItem, entry *dictionary.Lemma) {
	item.Sources[SourceCorpus] = []LexID{
		{Key: LexKey{
			Lemma: entry.Sublemmas[0].Value,
			Pos:   entry.PoS,
			// TODO, get specifier from corpus
			Gender: util.Ternary(entry.PoS == PosNoun, entry.Specifier, ""),
			Aspect: util.Ternary(entry.PoS == PosVerb, entry.Specifier, ""),
		}},
	}
	// use corpus PoS if pos source is unknown
	if item.PosSource == SourceEmpty {
		item.PosSource = SourceCorpus
	}
}

// ensureSearchedForm adds the searched term to forms of the entry,
// if the entry belongs to the searched candidate and the form is missing
func ensureSearchedForm(entry *dictionary.Lemma, candidateSublemma, searchTerm, formSublemma string) {
	belongsToCandidate := slices.ContainsFunc(entry.Sublemmas, func(s dictionary.Sublemma) bool {
		return s.Value == candidateSublemma
	})
	hasForm := slices.ContainsFunc(entry.Forms, func(f dictionary.Form) bool {
		return f.Value == searchTerm
	})
	if belongsToCandidate && !hasForm {
		entry.Forms = append(entry.Forms, dictionary.Form{Value: searchTerm, Sublemma: formSublemma})
	}
}
