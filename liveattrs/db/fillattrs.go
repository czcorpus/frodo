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
	"frodo/liveattrs/request/fillattrs"
	"frodo/liveattrs/utils"
	"strings"
)

// For a structattr and its values find values of structattrs specified in fill list
// Returns a dict of dicts {search_attr_value: {attr: value}}.
// In case nothing is found, ErrorEmptyResult is returned
func FillAttrs(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	qry fillattrs.Payload,
) (map[string]map[string]string, error) {

	selAttrs := make([]string, len(qry.Fill)+1)
	selAttrs[0] = utils.ImportKey(qry.Search)
	for i, f := range qry.Fill {
		selAttrs[i+1] = utils.ImportKey(f)
	}
	valuesPlaceholders := make([]string, len(qry.Values))
	for i := range qry.Values {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s FROM `%s_liveattrs_entry` WHERE %s IN (%s)",
		strings.Join(selAttrs, ", "),
		corpusInfo.GroupedName(),
		utils.ImportKey(qry.Search),
		strings.Join(valuesPlaceholders, ", "),
	)
	sqlVals := make([]any, len(qry.Values))
	for i, v := range qry.Values {
		sqlVals[i] = v
	}

	rows, err := db.Query(sql1, sqlVals...)
	ans := make(map[string]map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]map[string]string{}, err
	}
	defer rows.Close()
	isEmpty := true
	for rows.Next() {
		isEmpty = isEmpty && false
		ansVals := make([]sql.NullString, len(selAttrs))
		ansPvals := make([]any, len(selAttrs))
		for i := range ansVals {
			ansPvals[i] = &ansVals[i]
		}
		if err := rows.Scan(ansPvals...); err != nil {
			return map[string]map[string]string{}, err
		}
		srchVal := ansVals[0].String
		ans[srchVal] = make(map[string]string)
		for i := 1; i < len(ansVals); i++ {
			if ansVals[i].Valid {
				ans[srchVal][selAttrs[i]] = ansVals[i].String
			}
		}
	}
	if isEmpty {
		return ans, ErrorEmptyResult
	}
	return ans, nil
}
