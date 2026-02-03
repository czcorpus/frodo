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

package ujc

import (
	"database/sql"
	"fmt"
	"frodo/ujc/ssjc"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	db *sql.DB
}

func (actions *Handler) SearchSSJC(ctx *gin.Context) {
	ans, err := ssjc.SearchTerm(ctx, actions.db, ctx.Param("term"))
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	if ans.IsZero() {
		uniresp.RespondWithErrorJSON(ctx, fmt.Errorf("not found"), http.StatusNotFound)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		db: db,
	}
}
