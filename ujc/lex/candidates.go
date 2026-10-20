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
	"sort"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/czcorpus/cnc-gokit/collections"
)

type SearchCandidate struct {
	Value    string
	Distance int
	Source   Source
}

func (sc *SearchCandidate) isMultiWord() bool {
	return strings.Contains(sc.Value, " ")
}

func (actions *Handler) findMainSource(ctx context.Context, searchTerm string) (Source, error) {
	// find available sources for the best match, and select the main source based on the priority list
	sources, err := SearchAvailableSources(ctx, actions.db.DB(), searchTerm)
	if err != nil {
		return "", err
	}
	for _, source := range actions.sourcePriority {
		if collections.SliceContains(sources, source) {
			return source, nil
		}
	}
	return "", nil
}

func (actions *Handler) getLematizedTerms(ctx context.Context, corpusId, term string) ([]string, error) {
	lemmatizedTerms := make([]string, 0, 10)
	for _, word := range strings.Fields(term) {
		matches, err := dictionary.Search(
			ctx,
			actions.db,
			corpusId,
			dictionary.SearchWithAnyValue(word),
		)
		if err != nil {
			return []string{}, err
		}
		newLemmatizedTerms := make([]string, 0, 10)
		for _, match := range matches {
			for _, sublemma := range match.Sublemmas {
				if len(lemmatizedTerms) == 0 {
					newLemmatizedTerms = append(newLemmatizedTerms, sublemma.Value)
				} else {
					for _, lemmatizedPart := range lemmatizedTerms {
						newLemmatizedTerms = append(newLemmatizedTerms, lemmatizedPart+" "+sublemma.Value)
					}
				}
			}
		}
		lemmatizedTerms = newLemmatizedTerms
	}
	return lemmatizedTerms, nil
}

func (actions *Handler) getLematizedSearchCandidates(ctx context.Context, corpusId, term string) ([]SearchCandidate, error) {
	lemmatizedTerms, err := actions.getLematizedTerms(ctx, corpusId, term)
	if err != nil {
		return nil, err
	}
	searchCandidates := make([]SearchCandidate, 0, max(len(lemmatizedTerms), 1))
	if len(lemmatizedTerms) == 0 {
		searchCandidates = append(searchCandidates, SearchCandidate{Value: term, Distance: 0})
	} else {
		for _, lemmatizedTerm := range lemmatizedTerms {
			searchCandidates = append(searchCandidates, SearchCandidate{Value: lemmatizedTerm, Distance: levenshtein.ComputeDistance(term, lemmatizedTerm)})
		}
		// sort matches by their similarity to the query term using Levenshtein distance
		sort.Slice(searchCandidates, func(i, j int) bool {
			return searchCandidates[i].Distance < searchCandidates[j].Distance
		})
	}
	return searchCandidates, nil
}

func (actions *Handler) getSearchCandidates(ctx context.Context, corpusId string, term string) ([]SearchCandidate, error) {
	corpusSearchCandidates, err := actions.getLematizedSearchCandidates(ctx, corpusId, term)
	if err != nil {
		return nil, err
	}

	// merge seach candidates, first is exact match, then corpus lematized candidates, remove duplicates
	searchCandidates := append([]SearchCandidate{{Value: term, Distance: 0}}, corpusSearchCandidates...)
	searchCandidates = collections.SliceReduce(searchCandidates, func(acc []SearchCandidate, curr SearchCandidate, i int) []SearchCandidate {
		if collections.SliceFindIndex(acc, func(item SearchCandidate) bool {
			return item.Value == curr.Value
		}) == -1 {
			acc = append(acc, curr)
		}
		return acc
	}, make([]SearchCandidate, 0, len(corpusSearchCandidates)+1))

	// find main source for each candidate
	for i, item := range searchCandidates {
		source, err := actions.findMainSource(ctx, item.Value)
		if err != nil {
			return nil, err
		}
		searchCandidates[i].Source = source
	}

	// return only candidates available in some source
	return collections.SliceFilter(searchCandidates, func(item SearchCandidate, i int) bool {
		return item.Source != ""
	}), nil
}
