// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
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

package actions

import (
	"encoding/json"
	"fmt"
	"frodo/liveattrs/db"
	"frodo/liveattrs/request/biblio"
	"frodo/liveattrs/request/query"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

var (
	attrValidRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)
)

// GetBibliography godoc
// @Summary      Get bibliography for specified corpus
// @Accept  	 json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 queryArgs body biblio.Payload true "Query arguments"
// @Success      200 {object} map[string]string
// @Router       /liveAttributes/{corpusId}/getBibliography [post]
func (a *Actions) GetBibliography(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to get bibliography from corpus %s: %w"

	var qry biblio.Payload
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.corpusMeta.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.GetBibliography(a.laDB.DB(), corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

// FindBibTitles godoc
// @Summary      Find bibliography titles in specified corpus
// @Accept  	 json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 queryArgs body biblio.PayloadList true "Query arguments"
// @Success      200 {object} map[string]string
// @Router       /liveAttributes/{corpusId}/findBibTitles [post]
func (a *Actions) FindBibTitles(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to find bibliography titles in corpus %s: %w"

	var qry biblio.PayloadList
	err := json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}
	corpInfo, err := a.corpusMeta.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	laConf, err := a.laConfCache.Get(corpInfo.Name)
	if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	ans, err := db.FindBibTitles(a.laDB.DB(), corpInfo, laConf, qry)
	if err == db.ErrorEmptyResult {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusNotFound)
		return

	} else if err != nil {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, &ans)
}

func isValidAttr(a string) bool {
	return attrValidRegex.MatchString(a)
}

// DocumentList godoc
// @Summary      Download document list for specified corpus
// @Accept       json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 queryArgs body query.Payload true "Query arguments"
// @Param        attr query []string true "???"
// @Param        page query int false "Page" default(1)
// @Param        pageSize query int false "Page size" default(0)
// @Success      200 {object} []db.DocumentRow
// @Router       /liveAttributes/{corpusId}/documentList [post]
func (a *Actions) DocumentList(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to download document list from %s: %w"
	corpInfo, err := a.corpusMeta.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}
	spage := ctx.Request.URL.Query().Get("page")
	if spage == "" {
		spage = "1"
	}
	page, err := strconv.Atoi(spage)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusBadRequest,
		)
		return
	}
	spageSize := ctx.Request.URL.Query().Get("pageSize")
	if spageSize == "" {
		spageSize = "0"
	}
	pageSize, err := strconv.Atoi(spageSize)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusBadRequest,
		)
		return
	}
	if pageSize == 0 && page != 1 || pageSize < 0 || page < 0 {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(
				baseErrTpl,
				corpusID,
				fmt.Errorf("page or pageSize argument incorrect (got: %d and %d)", page, pageSize)),
			http.StatusUnprocessableEntity,
		)
		return
	}

	pginfo := db.PageInfo{Page: page, PageSize: pageSize}

	for _, v := range ctx.Request.URL.Query()["attr"] {
		if !isValidAttr(v) {
			uniresp.WriteJSONErrorResponse(
				ctx.Writer,
				uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("incorrect attribute %s", v)),
				http.StatusUnprocessableEntity,
			)
			return
		}
	}

	var qry query.Payload
	err = json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return

	}

	var ans []*db.DocumentRow
	ans, err = db.GetDocuments(
		a.laDB.DB(),
		corpInfo,
		ctx.Request.URL.Query()["attr"],
		qry.Aligned,
		qry.Attrs,
		pginfo,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// NumMatchingDocuments godoc
// @Summary      Count number of matching documents for specified corpus
// @Accept       json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 queryArgs body query.Payload true "Query arguments"
// @Success      200 {int} int
// @Router       /liveAttributes/{corpusId}/numMatchingDocuments [post]
func (a *Actions) NumMatchingDocuments(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	baseErrTpl := "failed to count number of matching documents in %s: %w"
	corpInfo, err := a.corpusMeta.LoadInfo(corpusID)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	if corpInfo.BibIDAttr == "" {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, fmt.Errorf("bib. ID not defined for %s", corpusID)),
			http.StatusNotFound,
		)
		return
	}

	var qry query.Payload
	err = json.NewDecoder(ctx.Request.Body).Decode(&qry)
	if err != nil && err != io.EOF {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError(baseErrTpl, corpusID, err), http.StatusBadRequest)
		return
	}

	ans, err := db.GetNumOfDocuments(
		a.laDB.DB(),
		corpInfo,
		qry.Aligned,
		qry.Attrs,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError(baseErrTpl, corpusID, err),
			http.StatusInternalServerError,
		)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
