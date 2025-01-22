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

package adhoc

import (
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs/request/query"
	"strings"
)

// SubcSize is a generator for an SQL query + args for obtaining a subcorpus based
// on ad-hoc selection of text types
type SubcSize struct {
	CorpusInfo          *corpus.DBInfo
	AttrMap             query.Attrs
	AlignedCorpora      []string
	EmptyValPlaceholder string
}

// Query generates the result
// Please note that this is largely similar to laquery.AttrArgs.ExportSQL()
func (ssize *SubcSize) Query() (ansSQL string, whereValues []any) {
	joinSQL := make([]string, 0, 10)
	whereSQL := []string{
		"t1.corpus_id = ?",
		"t1.poscount is NOT NULL",
	}
	whereValues = []any{ssize.CorpusInfo.Name}
	for i, item := range ssize.AlignedCorpora {
		iOffs := i + 2
		joinSQL = append(
			joinSQL,
			fmt.Sprintf(
				"JOIN `%s_liveattrs_entry` AS t%d ON t1.item_id = t%d.item_id",
				ssize.CorpusInfo.GroupedName(), iOffs, iOffs,
			),
		)
		whereSQL = append(
			whereSQL,
			fmt.Sprintf("t%d.corpus_id = ?", iOffs),
		)
		whereValues = append(whereValues, item)
	}

	aargs := PredicateArgs{
		data:                ssize.AttrMap,
		emptyValPlaceholder: ssize.EmptyValPlaceholder,
		bibLabel:            ssize.CorpusInfo.BibLabelAttr,
	}
	where2, args2 := aargs.ExportSQL("t1", ssize.CorpusInfo.Name)
	whereSQL = append(whereSQL, where2)
	whereValues = append(whereValues, args2...)
	ansSQL = fmt.Sprintf(
		"SELECT SUM(t1.poscount) FROM `%s_liveattrs_entry` AS t1 %s WHERE %s",
		ssize.CorpusInfo.GroupedName(),
		strings.Join(joinSQL, " "),
		strings.Join(whereSQL, " AND "),
	)
	return
}
