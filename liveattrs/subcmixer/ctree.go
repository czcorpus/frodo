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
	"database/sql"
	"fmt"
	"frodo/common"
	"math"
	"strings"
)

type AbstractExpression interface {
	Negate() AbstractExpression
	IsComposed() bool
	Add(other AbstractExpression)
	GetAtoms() []AbstractAtomicExpression
	IsEmpty() bool
	OpSQL() string
}

type AbstractAtomicExpression interface {
	AbstractExpression
	Attr() string
	Value() string
}

type TaskArgs struct {
	NodeID     int                `json:"node_id"`
	ParentID   common.Maybe[int]  `json:"parent_id"`
	Ratio      float64            `json:"ratio"`
	Expression AbstractExpression `json:"expression"`
}

type CategoryTree struct {
	CorpusID       string
	AlignedCorpora []string
	CorpusMaxSize  int
	CategoryList   []TaskArgs
	RootNode       *CategoryTreeNode
	DB             *sql.DB
	TableName      string
}

func (ct *CategoryTree) NumCategories() int {
	return len(ct.CategoryList)
}

func (ct *CategoryTree) addVirtualCats() {
	updatedList := make([]TaskArgs, len(ct.CategoryList))
	copy(updatedList, ct.CategoryList)
	catsUpdated := make([]bool, ct.NumCategories())
	for _, cat := range ct.CategoryList {
		parID := cat.ParentID

		if parID.Matches(func(v int) bool { return v > 0 && !catsUpdated[v] }) {
			i := 0
			mdc := &ExpressionJoin{Op: "AND"}
			for _, otherCat := range ct.CategoryList {
				if otherCat.ParentID == parID {
					mdc.Add(otherCat.Expression.Negate())
					i += 1
				}
			}
			// updated_list.append(TaskArgs(self.num_categories, par_id, 0, mdc))
			updatedList = append(
				updatedList,
				TaskArgs{
					len(updatedList),
					parID,
					0,
					mdc,
				},
			)
			parID.Apply(func(v int) { catsUpdated[v] = true })
		}
	}
	ct.CategoryList = updatedList
}

func (ct *CategoryTree) build() {
	for _, cat := range ct.CategoryList[1:] {
		if cat.ParentID.Empty() || cat.Expression.IsEmpty() {
			continue
		}
		pidValue, _ := cat.ParentID.Value()
		parentNode := ct.getNodeByID(ct.RootNode, pidValue)
		pmc := parentNode.MetadataCondition
		var res []AbstractExpression
		if len(pmc) > 0 && !pmc[0].IsEmpty() {
			res = append([]AbstractExpression{cat.Expression}, pmc...)

		} else {
			res = []AbstractExpression{cat.Expression}
		}
		catNode := CategoryTreeNode{
			NodeID:            cat.NodeID,
			ParentID:          cat.ParentID,
			MetadataCondition: res,
			Ratio:             cat.Ratio,
		}
		parentNode.Children = append(parentNode.Children, &catNode)
	}
}

// getNodeByID returns a node identified by its numeric ID.
// If nothing is found, nil is returned
func (ct *CategoryTree) getNodeByID(node *CategoryTreeNode, wantedID int) *CategoryTreeNode {
	if node.NodeID != wantedID {
		for _, child := range node.Children {
			n := ct.getNodeByID(child, wantedID)
			if n != nil && n.NodeID == wantedID {
				return n
			}
		}

	} else {
		return node
	}
	return nil
}

func (ct *CategoryTree) getMaxGroupSizes(nodes []*CategoryTreeNode, parentSize float64) ([]float64, error) {

	numg := len(nodes)
	childrenSize := common.SumOfMapped(
		nodes, func(v *CategoryTreeNode) float64 { return float64(v.Size) })
	dataSize := common.Min(childrenSize, parentSize)
	requiredSizes := make([]float64, numg)
	var maxSizes []float64
	for {
		for i := 0; i < numg; i++ {
			requiredSizes[i] = float64(dataSize) * nodes[i].Ratio
		}
		sizes := common.MapSlice(
			nodes, func(v *CategoryTreeNode, i int) float64 { return float64(v.Size) })
		ratios := common.MapSlice(
			nodes, func(v *CategoryTreeNode, i int) float64 { return v.Ratio })
		reserves, err := common.Subtract(
			sizes,
			requiredSizes,
		)
		if err != nil {
			return []float64{}, err
		}
		ilr := common.IndexOf(reserves, common.Min(reserves...))
		lowestReserve := reserves[ilr]
		if lowestReserve > -0.001 {
			maxSizes = requiredSizes
			break
		}
		dataSize = sizes[ilr] / ratios[ilr]
		for i := 0; i < numg; i++ {
			if i != ilr {
				sizes[i] = dataSize * ratios[i]
			}
		}
	}
	return maxSizes, nil
}

func (ct *CategoryTree) computeSizes(node *CategoryTreeNode) error {
	if !node.HasChildren() {
		return nil
	}
	for _, child := range node.Children {
		ct.computeSizes(child)
	}
	maxSizes, err := ct.getMaxGroupSizes(node.Children, float64(node.Size))
	if err != nil {
		return err
	}
	// update group size
	node.Size = int(common.SumOfMapped(maxSizes, func(v float64) float64 { return v }))
	// update child node sizes
	for i, child := range node.Children {
		d := child.Size - int(math.RoundToEven(maxSizes[i]))
		child.Size = int(math.RoundToEven(maxSizes[i]))
		if d > 0 {
			ct.computeSizes(child)
		}
	}
	return nil
}

// getCategorySize computes the maximal available size of category described
// by provided list of metadata conditions
//
// arguments:
// mc: A list of metadata sql conditions that determines if texts belongs
//
//	to this category
func (ct *CategoryTree) getCategorySize(mc []AbstractExpression) (int, error) {
	var sqle strings.Builder
	sqle.WriteString(fmt.Sprintf(
		"SELECT SUM(m1.poscount) FROM %s as m1", ct.TableName),
	)
	var args []any
	ct.appendAlignedCorpSQL(sqle, &args)
	whereSQL := []string{}
	for _, subl := range mc {
		for _, expr := range subl.GetAtoms() {
			whereSQL = append(
				whereSQL,
				fmt.Sprintf("m1.%s %s ?", expr.Attr(), expr.OpSQL()),
			)
		}
	}
	sqle.WriteString(
		fmt.Sprintf(
			" WHERE %s AND m1.corpus_id = ?",
			strings.Join(whereSQL, " AND "),
		),
	)
	for _, subl := range mc {
		for _, expr := range subl.GetAtoms() {
			args = append(args, expr.Value())
		}
	}
	args = append(args, ct.CorpusID)
	row := ct.DB.QueryRow(sqle.String(), args...)
	var csize int
	err := row.Scan(&csize)
	if err == sql.ErrNoRows {
		return 0, nil

	} else if err != nil {
		return -1, err
	}
	return csize, nil
}

// appendAlignedCorpSQL adds one or more JOINs attaching
// required aligned corpora to a partial SQL query
// (query without WHERE and following parts).

// Please note that table is self-joined via
// an artificial attribute 'item_id' which identifies
// a single bibliography item across all the languages
// (i.e. it is language-independent). In case of
// the Czech Nat. Corpus and its InterCorp series
// this is typically achieved by modifying
// id_attr value by stripping its language identification
// prefix (e.g. 'en:Adams-Holisticka_det_k:0' transforms
// into 'Adams-Holisticka_det_k:0').

// arguments:
// sql: a CQL prefix in form 'SELECT ... FROM ...'
// 	   (i.e. no WHERE, LIIMIT, HAVING...)
// args: arguments passed to this partial SQL

// returns:
// a 2-tuple (extended SQL string, extended args list)
func (ct *CategoryTree) appendAlignedCorpSQL(sqle strings.Builder, args *[]any) {
	for i, ac := range ct.AlignedCorpora {
		sqle.WriteString(
			fmt.Sprintf(
				"JOIN %s AS %d ON m%d.item_id = m%d.item_id AND m%d.corpus_id = ?",
				ct.TableName, i+2, i+1, i+2, i+2,
			),
		)
		*args = append(*args, ac)
	}
}

func (ct *CategoryTree) initializeBounds() error {
	for i := 1; i < len(ct.CategoryList); i++ {
		node := ct.getNodeByID(ct.RootNode, i)
		var err error
		node.Size, err = ct.getCategorySize(node.MetadataCondition)
		if err != nil {
			return err
		}
	}
	var sqle strings.Builder
	sqle.WriteString(
		fmt.Sprintf(
			"SELECT SUM(m1.poscount) FROM %s AS m1",
			ct.TableName,
		),
	)
	args := []any{}
	ct.appendAlignedCorpSQL(sqle, &args)
	sqle.WriteString(" WHERE m1.corpus_id = ?")
	args = append(args, ct.CorpusID)
	row := ct.DB.QueryRow(sqle.String(), args...)
	var maxAvailable int
	err := row.Scan(&maxAvailable)
	if err == sql.ErrNoRows || maxAvailable == 0 {
		return fmt.Errorf("failed to initialize bounds: %s", err)
	}
	ct.RootNode.Size = common.Min(ct.CorpusMaxSize, maxAvailable)
	ct.computeSizes(ct.RootNode)
	return nil
}

func NewCategoryTree(
	categoryList []TaskArgs,
	db *sql.DB,
	corpusID string,
	alignedCorpora []string,
	tableName string,
	corpusMaxSize int,
) (*CategoryTree, error) {
	ans := &CategoryTree{
		CorpusID:       corpusID,
		AlignedCorpora: alignedCorpora,
		TableName:      tableName,
		CorpusMaxSize:  corpusMaxSize,
		CategoryList:   categoryList,
		DB:             db,
		RootNode: &CategoryTreeNode{
			NodeID:            categoryList[0].NodeID,
			ParentID:          categoryList[0].ParentID,
			Ratio:             categoryList[0].Ratio,
			MetadataCondition: []AbstractExpression{},
		},
	}
	ans.addVirtualCats()
	ans.build()
	err := ans.initializeBounds()
	return ans, err
}
