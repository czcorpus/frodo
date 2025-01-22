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

package registry

import (
	"frodo/corpus"
	"net/http"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

// Actions wraps liveattrs-related actions
type Actions struct {
	conf *corpus.CorporaSetup
}

// DynamicFunctions provides a list of Manatee internal + our configured functions
// for generating dynamic attributes
func (a *Actions) DynamicFunctions(ctx *gin.Context) {
	fullList := dynFnList[:]
	fullList = append(fullList, DynFn{
		Name:        "geteachncharbysep",
		Args:        []string{"str", "n"},
		Description: "Separate a string by \"|\" and return all the pos-th elements from respective items",
		Dynlib:      a.conf.ManateeDynlibPath,
	})
	uniresp.WriteCacheableJSONResponse(ctx.Writer, ctx.Request, fullList)
}

func (a *Actions) PosSets(ctx *gin.Context) {
	ans := make([]Pos, len(posList))
	for i, v := range posList {
		ans[i] = v
	}
	uniresp.WriteCacheableJSONResponse(ctx.Writer, ctx.Request, ans)
}

func (a *Actions) GetPosSetInfo(ctx *gin.Context) {
	posID := ctx.Param("posId")
	var srch Pos
	for _, v := range posList {
		if v.ID == posID {
			srch = v
		}
	}
	if srch.ID == "" {
		uniresp.WriteJSONErrorResponse(ctx.Writer, uniresp.NewActionError("Tagset %s not found", posID), http.StatusInternalServerError)

	} else {
		uniresp.WriteJSONResponse(ctx.Writer, srch)
	}
}

func (a *Actions) GetAttrMultivalueDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetAttrMultisepDefaults(ctx *gin.Context) {
	ans := []multisep{
		{Value: "|", Description: "A default value used within the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) GetAttrDynlibDefaults(ctx *gin.Context) {
	ans := []dynlibItem{
		{Value: "internal", Description: "Functions provided by Manatee"},
		{Value: a.conf.ManateeDynlibPath, Description: "Custom functions provided by the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

func (a *Actions) GetAttrTransqueryDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetStructMultivalueDefaults(ctx *gin.Context) {
	uniresp.WriteJSONResponse(ctx.Writer, availBoolValues)
}

func (a *Actions) GetStructMultisepDefaults(ctx *gin.Context) {
	ans := []multisep{
		{Value: "|", Description: "A default value used within the CNC"},
	}
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}

// NewActions is the default factory for Actions
func NewActions(
	conf *corpus.CorporaSetup,
) *Actions {
	return &Actions{
		conf: conf,
	}
}
