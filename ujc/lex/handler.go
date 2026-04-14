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
	"frodo/ujc"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	db   *mysql.Adapter
	conf ujc.Conf
}

func (actions *Handler) getQueryMatches(ctx context.Context, term string) ([]dictionary.Lemma, error) {
	if actions.conf.BoundDict == "" {
		return []dictionary.Lemma{}, nil
	}
	ans, err := dictionary.Search(
		ctx,
		actions.db,
		actions.conf.BoundDict,
		dictionary.SearchWithAnyValue(term),
	)
	if err != nil {
		return []dictionary.Lemma{}, fmt.Errorf("failed to find lemma: %w", err)
	}
	if len(ans) > 0 {
		return ans, nil
	}
	return []dictionary.Lemma{}, nil
}

func (actions *Handler) attachCorpusLemmata(ctx context.Context, lexData *LexData) error {
	if actions.conf.BoundDict == "" {
		return nil
	}
	for i, item := range lexData.LexItems {
		corpusEntry, err := actions.searchCorpusLemma(ctx, item.Lemma, item.Pos)
		if err != nil {
			return fmt.Errorf("failed to search corpus lemma: %w", err)
		}
		lexData.LexItems[i].CorpusEntry = corpusEntry
		log.Debug().Str("lemma", item.Lemma).Str("pos", item.Pos).Interface("corpusEntry", corpusEntry).Msg("Attached corpus entry to lex item")
	}
	return nil
}

func (actions *Handler) searchCorpusLemma(ctx context.Context, lemma, pos string) (*dictionary.Lemma, error) {
	if lemma == "" {
		return nil, nil
	}

	posArg := dictionary.SearchWithNoOp()
	if pos != "" {
		posArg = dictionary.SearchWithPoS(pos)
	}
	ans, err := dictionary.Search(
		ctx,
		actions.db,
		actions.conf.BoundDict,
		dictionary.SearchWithLemma(lemma),
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
	// search corpus for possible lemmata of the word
	matches, err := actions.getQueryMatches(ctx, ctx.Param("term"))
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	for i, match := range matches {
		// search lex dictionary for the first lemma found in the corpus, get list of variants and their PoS
		lexData, err := SearchTerm(ctx, actions.db.DB(), match.Lemma)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		if lexData != nil {
			actions.attachCorpusLemmata(ctx, lexData)
			match.ExtraData = lexData
		}
		matches[i] = match
	}

	ans := map[string]any{
		"matches": matches,
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *mysql.Adapter, conf ujc.Conf) *Handler {
	return &Handler{
		db:   db,
		conf: conf,
	}
}
