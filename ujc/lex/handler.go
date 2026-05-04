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
	"frodo/db/mysql"
	"frodo/dictionary"
	dictActions "frodo/dictionary/actions"
	"net/http"
	"sort"

	"github.com/agnivade/levenshtein"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	db          *mysql.Adapter
	dictActions *dictActions.Actions
}

func (actions *Handler) getQueryMatches(ctx context.Context, corpusId, term string) ([]dictionary.Lemma, error) {
	datasetSize, err := actions.dictActions.GetDatasetSize(corpusId)
	if err != nil {
		return nil, err
	}

	ans, err := dictionary.Search(
		ctx,
		actions.db,
		corpusId,
		dictionary.SearchWithAnyValue(term),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
	)
	if err != nil {
		return []dictionary.Lemma{}, fmt.Errorf("failed to find lemma: %w", err)
	}
	if len(ans) > 0 {
		return ans, nil
	}
	return []dictionary.Lemma{}, nil
}

func (actions *Handler) attachCorpusLemmata(ctx context.Context, corpusId string, data []LexItem) ([]LexItem, error) {
	for i, item := range data {
		corpusEntry, err := actions.searchCorpusLemma(ctx, corpusId, item.Lemma, item.Pos)
		if err != nil {
			return nil, fmt.Errorf("failed to search corpus lemma: %w", err)
		}
		data[i].CorpusEntry = corpusEntry
		log.Debug().Str("lemma", item.Lemma).Str("pos", item.Pos).Interface("corpusEntry", corpusEntry).Msg("Attached corpus entry to lex item")
	}
	return data, nil
}

func (actions *Handler) searchCorpusLemma(ctx context.Context, corpusId, lemma, pos string) (*dictionary.Lemma, error) {
	if lemma == "" {
		return nil, nil
	}

	posArg := dictionary.SearchWithNoOp()
	if pos != "" {
		posArg = dictionary.SearchWithPoS(pos)
	}

	datasetSize, err := actions.dictActions.GetDatasetSize(corpusId)
	if err != nil {
		return nil, err
	}

	ans, err := dictionary.Search(
		ctx,
		actions.db,
		corpusId,
		dictionary.SearchWithLemma(lemma),
		dictionary.SearchWithDatasetSizeForIPM(int(datasetSize)),
		posArg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find lemma: %w", err)
	}
	if len(ans) > 0 {
		if len(ans) > 1 {
			log.Warn().Str("lemma", lemma).Str("pos", pos).Int("numMatches", len(ans)).Msg("Multiple matches found for lemma in corpus")
		}
		return &ans[0], nil
	}
	return nil, nil
}

func (actions *Handler) SearchWord(ctx *gin.Context) {
	corpusId := ctx.Param("corpusId")
	term := ctx.Param("term")

	// search corpus for possible lemmata of the word
	matches, err := actions.getQueryMatches(ctx, corpusId, term)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	// if empty corpus matches, use different sources
	if len(matches) == 0 {
		matches, err = SearchMatches(ctx, actions.db.DB(), term, SourceASSC)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return levenshtein.ComputeDistance(term, matches[i].Lemma) < levenshtein.ComputeDistance(term, matches[j].Lemma)
	})

	for i, match := range matches {
		// search lex dictionary for the first lemma found in the corpus, get list of variants and their PoS
		lexItems, err := SearchTerm(ctx, actions.db.DB(), match.Lemma, match.PoS)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		if lexItems != nil {
			actions.attachCorpusLemmata(ctx, corpusId, lexItems)
			match.ExtraData = lexItems
		}
		matches[i] = match
	}

	ans := map[string]any{
		"matches": matches,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *mysql.Adapter, dictActions *dictActions.Actions) *Handler {
	return &Handler{
		db:          db,
		dictActions: dictActions,
	}
}
