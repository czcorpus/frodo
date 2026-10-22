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

	"github.com/czcorpus/cnc-gokit/collections"
)

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

func (actions *Handler) getSearchCandidates(ctx context.Context, corpusId string, searchTerm string) ([]LemmatizedTerm, error) {
	lemmatizedSearchTerms, err := LemmatizeTermWithCorpus(ctx, actions.db.DB(), corpusId, searchTerm)
	if err != nil {
		return nil, err
	}
	if lemmatizedSearchTerms == nil {
		return nil, nil
	}

	lexTerms := make([]LemmatizedTerm, 0, 10)
	for _, lsTerm := range lemmatizedSearchTerms {
		data, err := SearchLexTerms(ctx, actions.db.DB(), lsTerm)
		if err != nil {
			return nil, err
		}
		lexTerms = append(lexTerms, data...)
	}

	// find main source for each candidate
	for i, item := range lexTerms {
		source, err := actions.findMainSource(ctx, item.Term)
		if err != nil {
			return nil, err
		}
		lexTerms[i].Source = source
	}

	// return only candidates available in some source
	return collections.SliceFilter(lexTerms, func(item LemmatizedTerm, i int) bool {
		return item.Source != ""
	}), nil
}
