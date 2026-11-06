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
	"context"
	"fmt"
	"frodo/dictionary"
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
