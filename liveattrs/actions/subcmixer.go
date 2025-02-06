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
	"frodo/common"
	"frodo/general/collections"
	"frodo/liveattrs/subcmixer"
	"net/http"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

const (
	corpusMaxSize = 500000000
)

type subcmixerRatio struct {
	AttrName  string  `json:"attrName"`
	AttrValue string  `json:"attrValue"`
	Ratio     float64 `json:"ratio"`
}

type subcmixerArgs struct {
	Corpora   []string         `json:"corpora"`
	TextTypes []subcmixerRatio `json:"textTypes"`
}

func (sa *subcmixerArgs) validate() error {
	currStruct := ""
	for _, tt := range sa.TextTypes {
		strc := strings.Split(tt.AttrName, ".")
		if currStruct != "" && currStruct != strc[0] {
			return fmt.Errorf("the ratio rules for subcmixer may contain only attributes of a single structure")
		}
		currStruct = strc[0]
	}
	return nil
}

func importTaskArgs(args subcmixerArgs) ([]subcmixer.TaskArgs, error) {
	ans := [][]subcmixer.TaskArgs{
		{
			{
				NodeID:     0,
				ParentID:   common.NewEmptyMaybe[int](),
				Ratio:      1,
				Expression: &subcmixer.CategoryExpression{},
			},
		},
	}
	groupedRatios := collections.NewMultidict[subcmixerRatio]()
	for _, item := range args.TextTypes {
		groupedRatios.Add(item.AttrName, item)
	}
	counter := 1
	err := groupedRatios.ForEach(func(k string, expressions []subcmixerRatio) error {
		tmp := []subcmixer.TaskArgs{}
		for _, pg := range ans[len(ans)-1] {
			for _, item := range expressions {
				sm, err := subcmixer.NewCategoryExpression(item.AttrName, "==", item.AttrValue)
				if err != nil {
					return err
				}
				tmp = append(
					tmp,
					subcmixer.TaskArgs{
						NodeID:     counter,
						ParentID:   common.NewMaybe(pg.NodeID),
						Ratio:      item.Ratio / 100.0,
						Expression: sm,
					},
				)
				counter++
			}
		}
		ans = append(ans, tmp)
		return nil
	})
	if err != nil && err != collections.ErrorStopIteration {
		return []subcmixer.TaskArgs{}, err
	}
	ret := []subcmixer.TaskArgs{}
	for _, item := range ans {
		for _, subitem := range item {
			ret = append(ret, subitem)
		}
	}
	return ret, nil
}

// MixSubcorpus godoc
// @Summary      Mix subcorpus for specified corpus
// @Accept  	 json
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param 		 queryArgs body subcmixerArgs true "Query arguments"
// @Success      200 {object} subcmixer.CorpusComposition
// @Router       /liveAttributes/{corpusId}/mixSubcorpus [post]
func (a *Actions) MixSubcorpus(ctx *gin.Context) {
	var args subcmixerArgs
	err := json.NewDecoder(ctx.Request.Body).Decode(&args)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError("failed to mix subcorpus: %w", err), http.StatusBadRequest)
		return
	}
	baseErrTpl := "failed to mix subcorpus for %s: %w"
	err = args.validate()
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, args.Corpora[0], err), http.StatusUnprocessableEntity)
	}
	conditions, err := importTaskArgs(args)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, args.Corpora[0], err), http.StatusInternalServerError)
	}
	laTableName := fmt.Sprintf("%s_liveattrs_entry", args.Corpora[0])
	catTree, err := subcmixer.NewCategoryTree(
		conditions,
		a.laDB.DB(),
		args.Corpora[0],
		args.Corpora[1:],
		laTableName,
		corpusMaxSize,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, args.Corpora[0], err), http.StatusInternalServerError)
		return
	}
	corpusDBInfo, err := a.cncDB.LoadInfo(args.Corpora[0])
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, args.Corpora[0], err), http.StatusInternalServerError)
		return
	}
	mm, err := subcmixer.NewMetadataModel(
		a.laDB.DB(),
		laTableName,
		catTree,
		corpusDBInfo.BibIDAttr,
	)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer, uniresp.NewActionError(baseErrTpl, args.Corpora[0], err), http.StatusInternalServerError)
		return
	}
	ans := mm.Solve()
	uniresp.WriteJSONResponse(ctx.Writer, ans)
}
