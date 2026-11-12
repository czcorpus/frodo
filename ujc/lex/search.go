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
	"frodo/dictionary"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

// search runs the whole lexicon search pipeline:
//  1. find lexicon candidates for the term (and its "se"/"si" variants) + typo suggestions
//  2. resolve lex items (variants) of the main candidate, including their source data
//  3. join each lex item with corpus data
//  4. append corpus-only sublemma variants
//
// If no lexicon candidate is found, only corpus entries matching the term are returned.
func (actions *Actions) search(ctx context.Context, corpusId, searchTerm string) (LexDictResponse, error) {
	if err := ValidateCorpusName(corpusId); err != nil {
		return LexDictResponse{}, err
	}

	candidates, typoSuggestions, err := actions.findCandidates(ctx, corpusId, searchTerm)
	if err != nil {
		return LexDictResponse{}, err
	}
	if len(candidates) == 0 {
		// term is not in lexicon, return just corpus entries
		corpus, err := actions.loadCorpusRef(corpusId)
		if err != nil {
			return LexDictResponse{}, err
		}
		matches, err := actions.searchCorpusOnlyEntries(ctx, corpus, searchTerm)
		if err != nil {
			return LexDictResponse{}, err
		}
		return LexDictResponse{Matches: matches, Suggestions: typoSuggestions}, nil
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

	corpus, err := actions.loadCorpusRef(corpusId)
	if err != nil {
		return LexDictResponse{}, err
	}

	variants, err := actions.matchCorpusEntries(ctx, corpus, searchTerm, usedCandidate, lexItems)
	if err != nil {
		return LexDictResponse{}, err
	}

	variants, err = actions.addCorpusSublemmaVariants(ctx, corpus, variants)
	if err != nil {
		return LexDictResponse{}, err
	}

	// remove found variants from suggestions
	foundLemmas := make(map[string]struct{}, len(lexItems))
	for _, item := range lexItems {
		foundLemmas[item.Key.Lemma] = struct{}{}
	}
	suggestions = collections.SliceFilter(suggestions, func(v string, i int) bool {
		_, found := foundLemmas[v]
		return !found
	})

	return LexDictResponse{Matches: variants, Suggestions: suggestions}, nil
}

// reflexive particles stripped from the end of the searched term
var reflexiveParticles = []string{"si", "sis", "se", "ses"}

// generatePossibleSearchTerms returns the term without reflexive particle
// followed by its "se" and "si" variants
func generatePossibleSearchTerms(searchTerm string) []string {
	baseTerm := searchTerm
	parts := strings.Fields(searchTerm)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if collections.SliceContains(reflexiveParticles, lastPart) {
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
	for _, searchTermVariant := range generatePossibleSearchTerms(searchTerm) {
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
	for _, searchTermVariant := range generatePossibleSearchTerms(candidate.Term) {
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

	env := TransformEnv{DB: db, SourcePriority: actions.sourcePriority, VariantSource: candidate.Source}
	lexItems, err := ApplyTransformations(ctx, env, lexItems, preSourceTransforms...)
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

	return ApplyTransformations(ctx, env, lexItems, postSourceTransforms...)
}

// matchCorpusEntries joins each lex item with its corpus entry,
// if not found, a new entry with minimal data is created
func (actions *Actions) matchCorpusEntries(
	ctx context.Context,
	corpus corpusRef,
	searchTerm string,
	usedCandidate LemmatizedTerm,
	lexItems []LexItem,
) ([]dictionary.Lemma, error) {
	lexLemmatized, err := LemmatizeTermsWithLex(
		ctx,
		actions.db.DB(),
		collections.SliceMap(lexItems, func(item LexItem, i int) string { return item.Key.Lemma }),
	)
	if err != nil {
		return nil, err
	}

	variants := make([]dictionary.Lemma, 0, len(lexItems))
	for i, item := range lexItems {
		lemmatized, err := actions.lemmatizeLexItem(ctx, corpus, item.Key.Lemma, lexLemmatized)
		if err != nil {
			return nil, err
		}

		entry, err := actions.findCorpusEntry(ctx, corpus, item, lemmatized)
		if err != nil {
			return nil, err
		}

		if entry.ID == entryOriginCorpus {
			attachCorpusSource(&item, entry)
		}
		entry.ID = entryID(entry.ID, i)
		ensureSearchedForm(entry, usedCandidate.Sublemma, searchTerm, lemmatized.Sublemma)
		entry.ExtraData = LexExtraData{
			CorpusId: corpus.ID,
			Variant:  item,
		}
		variants = append(variants, *entry)
	}
	return variants, nil
}

func (actions *Actions) addCorpusSublemmaVariants(ctx context.Context, corpus corpusRef, variants []dictionary.Lemma) ([]dictionary.Lemma, error) {
	result := make([]dictionary.Lemma, 0, len(variants))
	result = append(result, variants...)

	// variants may share lemma, sublemma and PoS (e.g. homonyms),
	// repeated search would only find already added variants
	type searchKey struct{ lemma, sublemma, pos string }
	searched := make(map[searchKey]struct{}, len(variants))

	for _, variant := range variants {
		if len(variant.Sublemmas) == 0 {
			continue
		}
		currentSublemma := variant.Sublemmas[0].Value
		key := searchKey{variant.Lemma, currentSublemma, variant.PoS}
		if _, ok := searched[key]; ok {
			continue
		}
		searched[key] = struct{}{}
		otherVariants, err := actions.searchOtherCorpusSublemmata(ctx, corpus, key.lemma, key.sublemma, key.pos)
		if err != nil {
			return nil, err
		}
		for _, candidate := range otherVariants {
			if len(candidate.Sublemmas) == 0 {
				continue
			}
			newSublemma := candidate.Sublemmas[0].Value

			// ignore candidates that are already in results
			// lemma is not compared - entries not found in corpus use lexicon lemma,
			// which may differ from corpus lemma (e.g. lex "rekursivní" vs. corp "rekurzivní")
			resultHasSublema := collections.SliceFindIndex(result, func(item dictionary.Lemma) bool {
				return item.PoS == candidate.PoS &&
					specifiersMatch(tagKeySpecifier(item), tagKeySpecifier(candidate)) &&
					len(item.Sublemmas) > 0 && item.Sublemmas[0].Value == newSublemma
			}) != -1
			if resultHasSublema {
				continue
			}

			// ignore candidates that can be found in lex dictionary
			if sources, err := SearchAvailableSources(ctx, actions.db.DB(), newSublemma); err != nil {
				return nil, err
			} else if len(sources) != 0 {
				continue
			}

			// add candidate as new variant
			result = append(result, newCorpusOnlyVariant(corpus, candidate, len(result)))
		}
	}
	return result, nil
}
