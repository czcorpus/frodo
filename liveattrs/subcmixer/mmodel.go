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

package subcmixer

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"frodo/common"
	"frodo/liveattrs/utils"
	"io"
	"math"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/rs/zerolog/log"
)

const (
	pulpSolverTimeoutSecs = 60
)

type CategorySize struct {
	Total      int     `json:"total"`
	Ratio      float64 `json:"ratio"`
	Expression string  `json:"expression"`
}

type CorpusComposition struct {
	Error         string         `json:"error,omitempty"`
	DocIDs        []string       `json:"docIds"`
	SizeAssembled int            `json:"sizeAssembled"`
	CategorySizes []CategorySize `json:"categorySizes"`
}

type MetadataModel struct {
	db        *sql.DB
	tableName string
	cTree     *CategoryTree
	idAttr    string
	textSizes []int
	idMap     map[string]int
	numTexts  int
	b         []float64
	a         [][]float64
}

func (mm *MetadataModel) getAllConditions(node *CategoryTreeNode) [][2]string {
	sqlArgs := [][2]string{}
	for _, subl := range node.MetadataCondition {
		for _, mc := range subl.GetAtoms() {
			sqlArgs = append(sqlArgs, [2]string{mc.Attr(), mc.Value()})
		}
	}
	for _, child := range node.Children {
		sqlArgs = append(sqlArgs, mm.getAllConditions(child)...)
	}
	return sqlArgs
}

// List all the texts matching main corpus. This will be the
// base for the 'A' matrix in the optimization problem.
// In case we work with aligned corpora we still want
// the same result here as the non-aligned items from
// the primary corpus will not be selected in
// _init_ab() due to applied self JOIN
// (append_aligned_corp_sql())

// Also generate a map "db_ID -> row index" to be able
// to work with db-fetched subsets of the texts and
// matching them with the 'A' matrix (i.e. in a filtered
// result a record has a different index then in
// all the records list).
func (mm *MetadataModel) getTextSizes() ([]int, map[string]int, error) {
	allCond := mm.getAllConditions(mm.cTree.RootNode)
	allCondSQL := make([]string, len(allCond))
	allCondArgsSQL := make([]any, len(allCond))
	for i, v := range allCond {
		allCondSQL[i] = fmt.Sprintf("%s = ?", v[0])
		allCondArgsSQL[i] = v[1]
	}
	var sqle strings.Builder
	sqle.WriteString(fmt.Sprintf(
		"SELECT MIN(m1.id) AS db_id, SUM(poscount) FROM %s AS m1 ",
		mm.tableName,
	))
	args := []any{}
	sqle.WriteString(fmt.Sprintf(
		" WHERE m1.corpus_id = ? AND (%s) GROUP BY %s ORDER BY db_id",
		strings.Join(allCondSQL, " OR "),
		utils.ImportKey(mm.idAttr),
	))
	args = append(args, mm.cTree.CorpusID)
	args = append(args, allCondArgsSQL...)
	sizes := []int{}
	idMap := make(map[string]int)
	rows, err := mm.db.Query(sqle.String(), args...)
	if err != nil {
		return []int{}, map[string]int{}, err
	}
	i := 0
	for rows.Next() {
		var minCount int
		var docID string
		err := rows.Scan(&docID, &minCount)
		if err != nil {
			return []int{}, map[string]int{}, err
		}
		sizes = append(sizes, minCount)
		idMap[docID] = i
		i++
	}
	return sizes, idMap, nil
}

func (mm *MetadataModel) initABNonalign(usedIDs *collections.Set[string]) {
	// Now we process items with no aligned counterparts.
	// In this case we must define a condition which will be
	// fulfilled iff X[i] == 0
	for k, v := range mm.idMap {
		if !usedIDs.Contains(k) {
			for i := 1; i < len(mm.b); i++ {
				mult := 10000.0
				if mm.b[i] > 0 {
					mult = 2.0
				}
				mm.a[i][v] = mm.b[i] * mult
			}
		}
	}
}

func (mm *MetadataModel) PrintA(m [][]float64) {
	for _, v := range m {
		fmt.Println(v)
	}
}

func (mm *MetadataModel) initAB(node *CategoryTreeNode, usedIDs *collections.Set[string]) error {
	if len(node.MetadataCondition) > 0 {
		sqlItems := []string{}
		for _, subl := range node.MetadataCondition {
			for _, mc := range subl.GetAtoms() {
				sqlItems = append(
					sqlItems,
					fmt.Sprintf("m1.%s %s ?", mc.Attr(), mc.OpSQL()),
				)
			}
		}
		sqlArgs := []any{}
		var sqle strings.Builder
		sqle.WriteString(fmt.Sprintf(
			"SELECT m1.id AS db_id, SUM(m1.poscount) FROM %s AS m1 ",
			mm.tableName,
		))
		mm.cTree.appendAlignedCorpSQL(sqle, &sqlArgs)
		sqle.WriteString(fmt.Sprintf(
			"WHERE %s AND m1.corpus_id = ? GROUP BY %s ORDER BY db_id",
			strings.Join(sqlItems, " AND "), utils.ImportKey(mm.idAttr),
		))
		// mc.value for subl in node.metadata_condition for mc in subl
		for _, subl := range node.MetadataCondition {
			for _, mc := range subl.GetAtoms() {
				sqlArgs = append(sqlArgs, mc.Value())
			}
		}
		sqlArgs = append(sqlArgs, mm.cTree.CorpusID)
		rows, err := mm.db.Query(sqle.String(), sqlArgs...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var minCount int
			var docID string
			err := rows.Scan(&docID, &minCount)
			if err != nil {
				return err
			}
			mcf := float64(minCount)
			mm.a[node.NodeID-1][mm.idMap[docID]] = mcf
			usedIDs.Add(docID)
		}
		mm.b[node.NodeID-1] = float64(node.Size)
	}
	if len(node.Children) > 0 {
		for _, child := range node.Children {
			mm.initAB(child, usedIDs)
		}
	}
	return nil
}

func (mm *MetadataModel) isZeroVector(m []float64) bool {
	for i := 0; i < len(m); i++ {
		if m[i] > 0 {
			return false
		}
	}
	return true
}

func (mm *MetadataModel) getCategorySize(results []float64, catID int) (float64, error) {
	return common.DotProduct(results, mm.a[catID])
}

func (mm *MetadataModel) getAssembledSize(results []float64) float64 {
	var ans float64
	for i := 0; i < mm.numTexts; i++ {
		ans += results[i] * float64(mm.textSizes[i])
	}
	return ans
}

// Solve calculates a task of mixing texts with
// defined type ratios. The core LP logic is
// in the scripts/subcmixer_solve.py file
// (based on the Pulp library). Please note that
// the current implementation forces a hardcoded
// timeout specified with the constant [pulpSolverTimeoutSecs].
func (mm *MetadataModel) Solve() *CorpusComposition {
	ctx, cancel := context.WithTimeout(context.Background(), pulpSolverTimeoutSecs*time.Second)
	defer cancel()

	if mm.isZeroVector(mm.b) {
		return &CorpusComposition{}
	}
	c := make([]float64, mm.numTexts)
	for i := 0; i < mm.numTexts; i++ {
		c[i] = 1.0
	}

	// here we use external python solver
	json_data, err := json.Marshal(map[string]any{
		"A": mm.a,
		"b": mm.b,
	})
	if err != nil {
		return &CorpusComposition{Error: err.Error()}
	}

	_, currPath, _, _ := runtime.Caller(0)
	currPath = filepath.Dir(currPath)
	cmd := exec.CommandContext(ctx, path.Join(currPath, "..", "..", "scripts/subcmixer_solve.py"))
	var out bytes.Buffer
	cmd.Stdout = &out

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return &CorpusComposition{Error: err.Error()}
	}

	err = cmd.Start()
	if err != nil {
		return &CorpusComposition{Error: err.Error()}
	}

	io.WriteString(stdin, string(json_data))
	stdin.Close()

	err = cmd.Wait()
	if err == context.DeadlineExceeded {
		return &CorpusComposition{
			Error: fmt.Sprintf("Pulp LP solver timeout after %ds", pulpSolverTimeoutSecs),
		}

	} else if err != nil {
		return &CorpusComposition{Error: err.Error()}
	}

	variables := []float64{}
	err = json.Unmarshal(out.Bytes(), &variables)
	if err != nil {
		log.Err(err).Msg("")
	}
	log.Debug().Msgf("variables: %v", variables)

	var simplexErr error
	selections := common.MapSlice(
		variables,
		func(v float64, i int) float64 { return math.RoundToEven(v) },
	)
	categorySizes := make([]float64, mm.cTree.NumCategories()-1)

	for c := 0; c < mm.cTree.NumCategories()-1; c++ {
		catSize, err := mm.getCategorySize(selections, c)
		if err != nil {
			log.Err(err).Msgf("Failed to get cat size")
		}
		categorySizes[c] = catSize
	}
	docIDs := make([]string, 0, len(selections))
	for docID, idx := range mm.idMap {
		if selections[idx] == 1 {
			docIDs = append(docIDs, docID)
		}
	}
	var errDesc string
	if simplexErr != nil {
		errDesc = simplexErr.Error()
	}
	allCond := mm.getAllConditions(mm.cTree.RootNode)
	total := mm.getAssembledSize(selections)
	return &CorpusComposition{
		Error:         errDesc,
		DocIDs:        docIDs,
		SizeAssembled: int(total),
		CategorySizes: common.MapSlice(
			categorySizes,
			func(v float64, i int) CategorySize {
				var ratio float64
				if total > 0 {
					ratio = v / total
				}
				return CategorySize{
					Total: int(v),
					Ratio: ratio,
					Expression: fmt.Sprintf(
						"%s == '%s'",
						utils.ExportKey(allCond[i][0]),
						utils.ExportKey(allCond[i][1]),
					),
				}
			},
		),
	}
}

func NewMetadataModel(
	metaDB *sql.DB,
	tableName string,
	cTree *CategoryTree,
	idAttr string,
) (*MetadataModel, error) {
	ans := &MetadataModel{
		db:        metaDB,
		tableName: tableName,
		cTree:     cTree,
		idAttr:    idAttr,
	}
	ts, idMap, err := ans.getTextSizes()
	if err != nil {
		return nil, err
	}
	ans.idMap = idMap
	ans.textSizes = ts
	ans.numTexts = len(ts)
	ans.b = make([]float64, ans.cTree.NumCategories()-1)
	usedIDs := collections.NewSet[string]()
	ans.a = make([][]float64, ans.cTree.NumCategories()-1)
	for i := 0; i < len(ans.a); i++ {
		ans.a[i] = make([]float64, ans.numTexts)
	}
	ans.initAB(cTree.RootNode, usedIDs)
	// for items without aligned counterparts we create
	// conditions fulfillable only for x[i] = 0
	ans.initABNonalign(usedIDs)
	return ans, nil
}
