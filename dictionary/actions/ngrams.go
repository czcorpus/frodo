// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Institute of the Czech National Corpus,
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
	"errors"
	"fmt"
	"frodo/corpus"
	"frodo/db/mysql"
	"frodo/liveattrs/db/freqdb"
	"frodo/liveattrs/laconf"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/czcorpus/cnc-gokit/strutil"
	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/corp"
	"github.com/czcorpus/vert-tagextract/v3/cnf"
)

func ShowErrorChain(err error) string {
	// Walk through the entire error chain
	var ans strings.Builder
	ans.WriteString("Error chain:\n")
	current := err
	level := 0
	for current != nil {
		fmt.Fprintf(&ans, "%d: %v\n", level, current)
		current = errors.Unwrap(current)
		level++
	}
	return ans.String()
}

func mergeAliasedConfig(orig, aliased *cnf.VTEConf) *cnf.VTEConf {
	ans := *orig
	if aliased.AtomParentStructure != "" {
		ans.AtomParentStructure = aliased.AtomParentStructure
	}
	if aliased.AtomStructure != "" {
		ans.AtomStructure = aliased.AtomStructure
	}
	if len(aliased.IndexedCols) > 0 {
		ans.IndexedCols = aliased.IndexedCols
	}
	if len(aliased.VerticalFiles) > 0 {
		ans.VerticalFiles = aliased.VerticalFiles
	}
	if aliased.DateAttr != nil {
		ans.DateAttr = aliased.DateAttr
	}
	if aliased.RemoveEntriesBeforeDate != nil {
		ans.RemoveEntriesBeforeDate = aliased.RemoveEntriesBeforeDate
	}
	if aliased.MaxNumErrors > 0 {
		ans.MaxNumErrors = aliased.MaxNumErrors
	}
	if aliased.SelfJoin.IsConfigured() {
		ans.SelfJoin = aliased.SelfJoin
	}
	if aliased.BibView.IsConfigured() {
		ans.BibView = aliased.BibView
	}
	if !aliased.Ngrams.IsZero() {
		ans.Ngrams = aliased.Ngrams
	}
	return &ans
}

type NGramsReqArgs struct {
	ColMapping            *corpus.QSAttributes `json:"colMapping,omitempty"`
	PosTagset             corp.SupportedTagset `json:"posTagset"`
	UsePartitionedTable   bool                 `json:"usePartitionedTable"`
	MinFreq               int                  `json:"minFreq"`
	SkipGroupedNameSearch bool                 `json:"skipGroupedNameSearch"`
}

func (args NGramsReqArgs) Validate() error {
	if args.MinFreq <= 0 {
		args.MinFreq = 1
	}
	if err := args.PosTagset.Validate(); err != nil {
		return fmt.Errorf("failed to validate tagset: %w", err)
	}

	if args.ColMapping != nil {
		tmp := make(map[int]int)
		tmp[args.ColMapping.Lemma]++
		tmp[args.ColMapping.Sublemma]++
		tmp[args.ColMapping.Word]++
		tmp[args.ColMapping.Tag]++

		if !(len(tmp) == 4 || len(tmp) == 3 && args.ColMapping.Sublemma == args.ColMapping.Lemma) {
			return errors.New(
				"each of the lemma, sublemma, word, tag must be mapped to a unique table column with the exception that lemma and sublemma may address the same position")
		}
	}
	return nil
}

func (a *Actions) getNgramArgs(req *http.Request) (NGramsReqArgs, error) {
	var jsonArgs NGramsReqArgs
	err := json.NewDecoder(req.Body).Decode(&jsonArgs)
	if err == io.EOF {
		err = nil
	}
	return jsonArgs, err
}

// GenerateNgrams godoc
// @Summary      Generate n-grams for a specified corpus
// @Produce      json
// @Param        corpusId path string true "Used corpus"
// @Param        append query int false "Append mode" default(0)
// @Param        ngramSize query int false "N-gram size" default(1)
// @Success      200 {object} any
// @Router       /dictionary/{corpusId}/ngrams [post]
func (a *Actions) GenerateNgrams(ctx *gin.Context) {
	corpusID := ctx.Param("corpusId")
	aliasOf := ctx.Query("aliasOf")
	appendMode := ctx.Request.URL.Query().Get("append") == "1"
	ngramSize, ok := unireq.GetURLIntArgOrFail(ctx, "ngramSize", 1)
	if !ok {
		return
	}
	var laConf *cnf.VTEConf
	var err error
	if aliasOf != "" {
		laConf, err = a.laConfCache.Get(aliasOf)
		laConf.Corpus = corpusID

		// TODO this is kind of a debatable solution. When generating liveattrs,
		// using 'aliasOf' along with 'reconfigure=1' will produce regular vte
		// config file as for a normal db-backed config. But it still cannot
		// be used to generate ngrams as is. It is still rather a patch for
		// the original aliased corpus config. So here we try to load the alias
		// and use it to rewrite relevant attributres.
		// Possible solution: make the aliased config with a special attribute
		// specifying that it is an alias and hide the merging in the loading
		// process.
		laAlias, err := a.laConfCache.Get(corpusID)
		if err == nil {
			laConf = mergeAliasedConfig(laConf, laAlias)

		} else if err != laconf.ErrorNoSuchConfig {
			uniresp.RespondWithErrorJSON(
				ctx,
				err,
				http.StatusInternalServerError,
			)
			return
		}

	} else {
		laConf, err = a.laConfCache.Get(corpusID)
	}
	if err == laconf.ErrorNoSuchConfig {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusNotFound,
		)
		return

	} else if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}

	args, err := a.getNgramArgs(ctx.Request)
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
		return
	}
	if err = args.Validate(); err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return
	}

	var tagset corp.SupportedTagset

	if args.ColMapping == nil {

		if len(laConf.Ngrams.VertColumns) > 0 {
			args.ColMapping = &corpus.QSAttributes{}
			for _, v := range laConf.Ngrams.VertColumns {
				switch v.Role {
				case "word":
					args.ColMapping.Word = v.Idx
				case "lemma":
					args.ColMapping.Lemma = v.Idx
				case "tag":
					args.ColMapping.Tag = v.Idx
				case "pos":
					args.ColMapping.Pos = v.Idx
				case "sublemma":
					args.ColMapping.Sublemma = v.Idx
				}
			}
			tagset = args.PosTagset

		} else {

			regPath := filepath.Join(a.corpConf.RegistryDirPaths[0], corpusID) // TODO the [0]

			var corpTagsets []corp.SupportedTagset
			var err error

			if args.PosTagset != "" {
				corpTagsets = []corp.SupportedTagset{args.PosTagset}

			} else {
				corpTagsets, err = a.corpusMeta.GetCorpusTagsets(corpusID)
				if err != nil {
					uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
					return
				}
			}
			tagset = corpus.GetFirstSupportedTagset(corpTagsets)
			if tagset == "" {
				avail := strutil.JoinAny(corpTagsets, func(v corp.SupportedTagset) string { return v.String() }, ", ")
				uniresp.RespondWithErrorJSON(
					ctx, fmt.Errorf(
						"cannot find a suitable default tagset for %s (found: %s)",
						corpusID, avail,
					),
					http.StatusUnprocessableEntity,
				)
				return
			}
			attrMapping, err := corpus.InferQSAttrMapping(regPath, tagset)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}
			args.ColMapping = &attrMapping
			// now we need to revalidate to make sure the inference provided correct setup
			if err = args.Validate(); err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
				return
			}
		}

	} else {
		tagset = args.PosTagset
	}

	// the args.ColMapping.Tag arg below is likely OK,
	// but in such case, do we need args.ColMapping.Tag?
	// TODO !!! we probably do not need the ApplyPosProperties at all,
	// because the transformation is performed earlier in the liveattrs part
	// ([corpus]_colcounts table)
	posFn, err := corpus.ApplyPosProperties(&laConf.Ngrams, args.ColMapping.Tag, tagset)
	if err == corpus.ErrorPosNotDefined {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusUnprocessableEntity)
		return

	} else if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusInternalServerError,
		)
		return
	}

	groupedName := corpusID
	if !args.SkipGroupedNameSearch {
		corpusDBInfo, err := a.corpusMeta.LoadAliasedInfo(corpusID, aliasOf)
		if err != nil {
			uniresp.RespondWithErrorJSON(
				ctx,
				err,
				http.StatusInternalServerError,
			)
			return
		}
		corpusDBInfo.Name = corpusID
		groupedName = corpusDBInfo.GroupedName()
	}

	tunedDb, err := mysql.OpenImportTunedDB(a.laDB.Conf())
	if err != nil {
		uniresp.RespondWithErrorJSON(
			ctx,
			err,
			http.StatusInternalServerError,
		)
		return
	}
	generator := freqdb.NewNgramFreqGenerator(
		tunedDb,
		a.jobActions,
		groupedName,
		corpusID,
		a.laCustomNgramDataDirPath,
		args.UsePartitionedTable,
		appendMode,
		ngramSize,
		posFn,
		*args.ColMapping,
		args.MinFreq,
	)
	jobInfo, err := generator.GenerateAfter(ctx.Request.URL.Query().Get("parentJobId"))
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	uniresp.WriteJSONResponse(ctx.Writer, jobInfo.FullInfo())
}
