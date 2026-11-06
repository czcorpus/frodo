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
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/util"
)

// search runs the whole lexicon search pipeline:
//  1. find lexicon candidates for the term (and its "se"/"si" variants) + typo suggestions
//  2. resolve lex items (variants) of the main candidate, including their source data
//  3. join each lex item with corpus data
//  4. append corpus-only sublemma variants
func (actions *Actions) search(ctx context.Context, corpusId, searchTerm string) (LexDictResponse, error) {
	if err := ValidateCorpusName(corpusId); err != nil {
		return LexDictResponse{}, err
	}

	candidates, typoSuggestions, err := actions.findCandidates(ctx, corpusId, searchTerm)
	if err != nil {
		return LexDictResponse{}, err
	}
	if len(candidates) == 0 {
		return LexDictResponse{Matches: []dictionary.Lemma{}, Suggestions: typoSuggestions}, nil
	}

	// first candidate will be used for search, the rest will be used as suggestions
	usedCandidate := candidates[0]
	suggestions := append(collections.SliceMap(candidates[1:], func(item LemmatizedTerm, i int) string {
		return item.Term
	}), typoSuggestions...)

	lexItems, err := actions.resolveLexItems(ctx, usedCandidate)
	if err != nil {
		return LexDictResponse{}, err
	}
	// just in case..., should not happen, since searched item is certainly in dictionary, `variantSource` exists
	// TODO? corpus source
	if len(lexItems) == 0 {
		return LexDictResponse{Matches: []dictionary.Lemma{}, Suggestions: suggestions}, nil
	}

	variants, err := actions.buildCorpusEntries(ctx, corpusId, searchTerm, usedCandidate, lexItems)
	if err != nil {
		return LexDictResponse{}, err
	}

	variants, err = actions.addOtherSublemmaVariants(ctx, corpusId, variants)
	if err != nil {
		return LexDictResponse{}, err
	}

	// remove found variants from suggestions
	suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool {
		return collections.SliceFindIndex(lexItems, func(item LexItem) bool { return item.Key.Lemma == v }) == -1
	})

	return LexDictResponse{Matches: variants, Suggestions: suggestions}, nil
}

func (actions *Actions) generatePossibleSearchTerms(searchTerm string) []string {
	baseTerm := searchTerm
	parts := strings.Fields(searchTerm)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if collections.SliceContains([]string{"si", "sis", "se", "ses"}, lastPart) {
			baseTerm = strings.Join(parts[:len(parts)-1], " ")
		}
	}
	return []string{
		baseTerm,
		baseTerm + " se",
		baseTerm + " si",
	}
}

// findCandidates tries the term and its "se"/"si" variants and returns lexicon candidates
// of the first variant with any match, together with typo suggestions of all tried variants.
func (actions *Actions) findCandidates(ctx context.Context, corpusId, searchTerm string) ([]LemmatizedTerm, []string, error) {
	typoSuggestions := make([]string, 0, 10)
	for _, searchTermVariant := range actions.generatePossibleSearchTerms(searchTerm) {
		values, err := SearchTypoSuggestions(ctx, actions.db.DB(), searchTermVariant)
		if err != nil {
			return nil, nil, err
		}
		for _, v := range values {
			if !collections.SliceContains(typoSuggestions, v) {
				typoSuggestions = append(typoSuggestions, v)
			}
		}

		candidates, err := actions.getSearchCandidates(ctx, corpusId, searchTermVariant)
		if err != nil {
			return nil, nil, err
		}
		if len(candidates) > 0 {
			return candidates, typoSuggestions, nil
		}
	}
	return nil, typoSuggestions, nil
}

// resolveLexItems finds all lexicon variants of the candidate, joins their source data
// and applies source-specific transformations.
func (actions *Actions) resolveLexItems(ctx context.Context, candidate LemmatizedTerm) ([]LexItem, error) {
	db := actions.db.DB()

	var lexItems []LexItem
	for _, searchTermVariant := range actions.generatePossibleSearchTerms(candidate.Term) {
		searchCandidate := candidate
		searchCandidate.Term = searchTermVariant
		data, err := SearchVariants(ctx, db, searchCandidate)
		if err != nil {
			return nil, err
		}
		lexItems = append(lexItems, data...)
	}
	if len(lexItems) == 0 {
		return nil, nil
	}

	// apply special transformations before getting source data
	lexItems, err := ApplyTransformations(ctx, db, actions.sourcePriority, candidate.Source, lexItems,
		DTIJCR__MergeItems,
	)
	if err != nil {
		return nil, err
	}

	// for each variant, join source data
	for i, item := range lexItems {
		sources, err := SearchSources(ctx, db, item.Key)
		if err != nil {
			return nil, err
		}
		lexItems[i].Sources = sources
	}

	// apply special transformations after getting source data
	return ApplyTransformations(ctx, db, actions.sourcePriority, candidate.Source, lexItems,
		DTIJCR__ResolvePos,
		IJP__C_Is_NAP,
		IJP__RemoveUninflected,
		IJP__ResolveVariantKey,
		SortTransformation(candidate.Source),
		ResolveGroups,
	)
}

// buildCorpusEntries searches corpus entry for each lex item,
// if not found, a new entry with minimal data is created
func (actions *Actions) buildCorpusEntries(
	ctx context.Context,
	corpusId string,
	searchTerm string,
	usedCandidate LemmatizedTerm,
	lexItems []LexItem,
) ([]dictionary.Lemma, error) {
	db := actions.db.DB()
	variants := make([]dictionary.Lemma, 0, len(lexItems))
	for i, item := range lexItems {
		usedLemmatizedTerm, err := LemmatizeTermWithLex(ctx, db, item.Key.Lemma)
		if err != nil {
			return nil, err
		}
		if usedLemmatizedTerm == nil {
			lemmatizedTerms, err := LemmatizeTermWithCorpus(ctx, db, corpusId, item.Key.Lemma)
			if err != nil {
				return nil, err
			} else if len(lemmatizedTerms) == 0 {
				usedLemmatizedTerm = &LemmatizedTerm{Term: item.Key.Lemma, Lemma: item.Key.Lemma, Sublemma: item.Key.Lemma}
			} else {
				usedLemmatizedTerm = &lemmatizedTerms[0]
			}
		}

		specifier := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", cmp.Or(
			// corpus entry needs to replace "B" and "U" gender with "MI"
			util.Ternary(item.Key.Gender == GenderMascAnimInan || item.Key.Gender == GenderMascUnknown, "MI", item.Key.Gender),
			item.Key.Aspect,
		))

		var corpusEntries []*dictionary.Lemma
		var corpusEntry *dictionary.Lemma
		// use lemmatized term only for multi-word terms, otherwise use the lemma from lexicon
		searchLemma := util.Ternary(usedLemmatizedTerm.isMultiWord(), usedLemmatizedTerm.Lemma, item.Key.Lemma)
		searchSublemma := util.Ternary(usedLemmatizedTerm.isMultiWord(), usedLemmatizedTerm.Sublemma, item.Key.Lemma)
		searchPos := util.Ternary(usedLemmatizedTerm.isMultiWord(), "", item.Key.Pos)
		lemmaValues := strings.Fields(searchLemma)
		sublemmaValues := strings.Fields(searchSublemma)
		for i, sublemma := range sublemmaValues {
			lemma := sublemma
			if i < len(lemmaValues) {
				lemma = lemmaValues[i]
			}

			corpusEntry, err = actions.searchCorpusEntry(ctx, corpusId, lemma, sublemma, searchPos)
			if err != nil {
				return nil, err
			}

			if corpusEntry == nil {
				corpusEntry = &dictionary.Lemma{
					ID:        "lex",
					Lemma:     sublemma,
					PoS:       searchPos,
					Specifier: specifier,
					Forms:     []dictionary.Form{{Value: sublemma, Sublemma: sublemma}},
					Sublemmas: []dictionary.Sublemma{{Value: sublemma}},
				}

			} else {
				corpusEntry.ID = "corp"
				corpusEntry.Specifier = cmp.Or(corpusEntry.Specifier, specifier)
			}
			corpusEntries = append(corpusEntries, corpusEntry)
		}
		if corpusEntry == nil {
			corpusEntry = &dictionary.Lemma{
				ID:        "lex",
				Lemma:     item.Key.Lemma,
				PoS:       searchPos,
				Specifier: specifier,
				Forms:     []dictionary.Form{{Value: item.Key.Lemma, Sublemma: item.Key.Lemma}},
				Sublemmas: []dictionary.Sublemma{{Value: item.Key.Lemma}},
			}
		}

		// create multi word corpus entry from multiple single word entries
		if len(corpusEntries) > 1 {
			corpusEntry = actions.mergeMultiWordCorpusEntries(item, corpusEntries)
		}

		if corpusEntry.ID == "corp" {
			item.Sources[SourceCorpus] = []LexID{
				{Key: LexKey{
					Lemma: corpusEntry.Sublemmas[0].Value,
					Pos:   corpusEntry.PoS,
					// TODO, get specifier from corpus
					Gender: util.Ternary(corpusEntry.PoS == PosNoun, corpusEntry.Specifier, ""),
					Aspect: util.Ternary(corpusEntry.PoS == PosVerb, corpusEntry.Specifier, ""),
				}},
			}
			// use corpus PoS if pos source is unknown
			if item.PosSource == SourceEmpty && len(corpusEntries) > 0 {
				item.PosSource = SourceCorpus
			}
		}
		corpusEntry.ID = fmt.Sprintf("%s-%d", corpusEntry.ID, i)

		if collections.SliceFindIndex(corpusEntry.Sublemmas, func(sublemma dictionary.Sublemma) bool {
			return sublemma.Value == usedCandidate.Sublemma
		}) != -1 && collections.SliceFindIndex(corpusEntry.Forms, func(form dictionary.Form) bool {
			return form.Value == searchTerm
		}) == -1 {
			corpusEntry.Forms = append(corpusEntry.Forms, dictionary.Form{Value: searchTerm, Sublemma: usedLemmatizedTerm.Sublemma})
		}

		corpusEntry.ExtraData = LexExtraData{
			CorpusId: corpusId,
			Variant:  item,
		}
		variants = append(variants, *corpusEntry)
	}
	return variants, nil
}

func (actions *Actions) addOtherSublemmaVariants(ctx context.Context, corpusId string, variants []dictionary.Lemma) ([]dictionary.Lemma, error) {
	result := make([]dictionary.Lemma, 0, len(variants))
	result = append(result, variants...)

	for _, variant := range variants {
		if len(variant.Sublemmas) == 0 {
			continue
		}
		currentSublemma := variant.Sublemmas[0].Value
		otherVariants, err := actions.searchOtherSublemmata(ctx, corpusId, variant.Lemma, currentSublemma, variant.PoS)
		if err != nil {
			return nil, err
		}
		for _, otherVariant := range otherVariants {
			if len(otherVariant.Sublemmas) == 0 {
				continue
			}
			otherSublemma := otherVariant.Sublemmas[0].Value
			// lemma is not compared - entries not found in corpus use lexicon lemma,
			// which may differ from corpus lemma (e.g. lex "rekursivní" vs. corp "rekurzivní")
			if collections.SliceFindIndex(result, func(item dictionary.Lemma) bool {
				return item.PoS == otherVariant.PoS &&
					len(item.Sublemmas) > 0 && item.Sublemmas[0].Value == otherSublemma
			}) != -1 {
				continue
			}
			candidate := otherVariant
			// continue numbering after main variants, so IDs stay unique
			candidate.ID = fmt.Sprintf("corp-%d", len(result))
			candidate.ExtraData = LexExtraData{
				CorpusId: corpusId,
				Variant: LexItem{
					Key: LexKey{
						Lemma:  otherSublemma,
						Pos:    candidate.PoS,
						Gender: util.Ternary(candidate.PoS == PosNoun, candidate.Specifier, ""),
						Aspect: util.Ternary(candidate.PoS == PosVerb, candidate.Specifier, ""),
					},
					PosSource:     SourceCorpus,
					VariantSource: SourceCorpus,
					Sources: map[Source][]LexID{
						SourceCorpus: {{
							Key: LexKey{
								Lemma:  otherSublemma,
								Pos:    candidate.PoS,
								Gender: util.Ternary(candidate.PoS == PosNoun, candidate.Specifier, ""),
								Aspect: util.Ternary(candidate.PoS == PosVerb, candidate.Specifier, ""),
							},
						}},
					},
					// corpus variants are special group
					// set to something high, but different than other variants
					Group: 999,
				},
			}
			result = append(result, candidate)
		}
	}
	return result, nil
}
