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

package db

import (
	"database/sql"
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs/db/qbuilder/adhoc"
	"frodo/liveattrs/request/query"
)

func DeleteTable(tx *sql.Tx, groupedName string, corpusName string) error {
	_, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s_liveattrs_entry WHERE corpus_id = ?", groupedName),
		corpusName,
	)
	if err != nil {
		return err
	}
	if groupedName == corpusName {
		_, err = tx.Exec(
			fmt.Sprintf("DROP TABLE %s_liveattrs_entry", groupedName),
		)
	}
	return err
}

func GetSubcSize(laDB *sql.DB, corpusInfo *corpus.DBInfo, corpora []string, attrMap query.Attrs) (int, error) {
	sizeCalc := adhoc.SubcSize{
		CorpusInfo:          corpusInfo,
		AttrMap:             attrMap,
		AlignedCorpora:      corpora[1:],
		EmptyValPlaceholder: "", // TODO !!!!
	}
	sqlq, args := sizeCalc.Query()
	cur := laDB.QueryRow(sqlq, args...)
	var ans sql.NullInt64
	if err := cur.Scan(&ans); err != nil {
		return 0, err
	}
	if ans.Valid {
		return int(ans.Int64), nil
	}
	return 0, nil
}
