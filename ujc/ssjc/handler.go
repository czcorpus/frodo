// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
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

package ssjc

import (
	"context"
	"fmt"
	"frodo/db/mysql"
	"frodo/dictionary"
	"frodo/ujc"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	db   *mysql.Adapter
	conf ujc.Conf
}

// TODO: PoS support not implemented
func (actions *Handler) findCorpusLemma(ctx context.Context, lemma, pos string) (dictionary.Lemma, error) {
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
		return dictionary.Lemma{}, fmt.Errorf("failed to find lemma: %w", err)
	}
	if len(ans) > 0 {
		return ans[0], nil
	}
	return dictionary.Lemma{}, nil
}

func (actions *Handler) SearchSSJC(ctx *gin.Context) {
	ans, err := SearchTerm(ctx, actions.db.DB(), ctx.Param("term"))
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if ans.IsZero() {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("not found"), http.StatusNotFound)
		return
	}
	// TODO posOpts := dictionary.SearchWithPos
	corpLemma, err := actions.findCorpusLemma(ctx, ans.Headword, ans.Pos)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	ans.CorpusEntry = corpLemma
	for i, child := range ans.Children {
		corpLemma, err := actions.findCorpusLemma(ctx, child.Headword, child.Pos)
		if err != nil {
			uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
			return
		}
		child.CorpusEntry = corpLemma
		ans.Children[i] = child
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *mysql.Adapter, conf ujc.Conf) *Handler {
	return &Handler{
		db:   db,
		conf: conf,
	}
}
