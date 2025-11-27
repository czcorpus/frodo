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

package laquery

import (
	"database/sql"
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/utils"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

type LAFilter struct {
	CorpusInfo          *corpus.DBInfo
	AttrMap             query.Attrs
	SearchAttrs         []string
	AlignedCorpora      []string
	AutocompleteAttr    string
	EmptyValPlaceholder string
}

func (b *LAFilter) attrToSQL(values []string, prefix string) []string {
	ans := make([]string, len(values))
	for i, v := range values {
		ans[i] = prefix + "." + utils.ImportKey(v)
	}
	return ans
}

func (b *LAFilter) CreateSQL() QueryComponents {
	bibID := utils.ImportKey(b.CorpusInfo.BibIDAttr)
	bibLabel := utils.ImportKey(b.CorpusInfo.BibLabelAttr)
	attrItems := PredicateArgs{
		data:                b.AttrMap,
		bibID:               bibID,
		bibLabel:            bibLabel,
		autocompleteAttr:    b.AutocompleteAttr,
		emptyValPlaceholder: b.EmptyValPlaceholder,
	}
	whereSQL0, whereValues0 := attrItems.ExportSQL("t1", b.CorpusInfo.Name) // TODO py uses 'info.id' here
	whereSQL := make([]string, 0, 20)
	whereSQL = append(whereSQL, whereSQL0)
	whereValues := make([]string, 0, 20+len(whereValues0))
	whereValues = append(whereValues, whereValues0...)
	joinSQL := make([]string, 0, 20)
	for i, item := range b.AlignedCorpora {
		joinSQL = append(
			joinSQL,
			fmt.Sprintf(
				"JOIN `%s_liveattrs_entry` AS t%d ON t1.item_id = t%d.item_id", b.CorpusInfo.GroupedName(),
				i+2, i+2,
			),
		)
		whereSQL = append(whereSQL, fmt.Sprintf(" AND t%d.corpus_id = ?", i+2))
		whereValues = append(whereValues, item)
	}
	hiddenAttrs := collections.NewSet[string]()
	if bibID != "" && !collections.SliceContains(b.SearchAttrs, bibID) {
		hiddenAttrs.Add(bibID)
	}
	selectedAttrs := collections.NewSet(b.SearchAttrs...).Union(*hiddenAttrs)
	var sqlTemplate string
	if len(whereSQL) > 0 {
		sqlTemplate = fmt.Sprintf(
			"SELECT DISTINCT t1.poscount, t1.id, %s FROM `%s_liveattrs_entry` AS t1 %s WHERE %s",
			strings.Join(b.attrToSQL(selectedAttrs.ToOrderedSlice(), "t1"), ", "),
			b.CorpusInfo.GroupedName(),
			strings.Join(joinSQL, " "),
			strings.Join(whereSQL, " "),
		)

	} else {
		sqlTemplate = fmt.Sprintf(
			"SELECT DISTINCT t1.poscount, %s FROM `%s_liveattrs_entry` AS t1 %s",
			strings.Join(b.attrToSQL(selectedAttrs.ToOrderedSlice(), "t1"), ", "),
			b.CorpusInfo.GroupedName(),
			strings.Join(joinSQL, " "),
		)
	}
	return QueryComponents{
		sqlTemplate:   sqlTemplate,
		selectedAttrs: selectedAttrs.ToOrderedSlice(),
		hiddenAttrs:   hiddenAttrs.ToOrderedSlice(),
		whereValues:   whereValues,
	}
}

type ResultRow struct {
	Attrs     map[string]string
	Poscount  int
	Wordcount int
}

type DataIterator struct {
	DB      *sql.DB
	Builder *LAFilter
}

func (di *DataIterator) Iterate(fn func(row ResultRow) error) error {
	qc := di.Builder.CreateSQL()
	args := make([]any, len(qc.whereValues))
	for i, v := range qc.whereValues {
		args[i] = v
	}
	rows, err := di.DB.Query(qc.sqlTemplate, args...)
	if err != nil {
		return err
	}
	colnames, err := rows.Columns()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		pcols := make([]any, len(colnames))
		ansRow := ResultRow{
			Attrs: make(map[string]string, len(colnames)-2),
		}
		ansAttrs := make([]sql.NullString, len(colnames)-1)
		pcols[0] = &ansRow.Poscount
		for i := range ansAttrs {
			pcols[i+1] = &ansAttrs[i]
		}

		if err := rows.Scan(pcols...); err != nil {
			return err
		}
		for i, colname := range colnames[2:] {
			// we ignore 1 st item which is db ID
			if ansAttrs[i+1].Valid {
				ansRow.Attrs[colname] = ansAttrs[i+1].String
			}
		}
		err = fn(ansRow)
		if err != nil {
			return err
		}

	}
	return nil
}
