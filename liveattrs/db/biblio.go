// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
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
	"frodo/liveattrs/laconf"
	"frodo/liveattrs/request/biblio"
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/utils"
	"reflect"
	"strings"

	vteconf "github.com/czcorpus/vert-tagextract/v3/cnf"
	"github.com/rs/zerolog/log"
)

func GetBibliography(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.Payload,
) (map[string]string, error) {
	if corpusInfo.BibIDAttr == "" {
		return map[string]string{}, fmt.Errorf("cannot get bibliography for %s - bibIdAttr/Label not defined", corpusInfo.Name)
	}
	subcorpAttrs := laconf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = utils.ImportKey(attr)
	}

	sql1 := fmt.Sprintf(
		"SELECT %s FROM `%s_liveattrs_entry` WHERE %s = ? LIMIT 1",
		strings.Join(selAttrs, ", "),
		corpusInfo.GroupedName(),
		utils.ImportKey(corpusInfo.BibIDAttr),
	)

	rows := db.QueryRow(sql1, qry.ItemID)
	ans := make(map[string]string)
	ansVals := make([]sql.NullString, len(selAttrs))
	ansPvals := make([]any, len(selAttrs))
	for i := range ansVals {
		ansPvals[i] = &ansVals[i]
	}
	err := rows.Scan(ansPvals...)
	if err == sql.ErrNoRows {
		return map[string]string{}, ErrorEmptyResult
	}
	if err != nil {
		return map[string]string{}, err
	}
	for i, val := range ansVals {
		if val.Valid {
			ans[utils.ExportKey(selAttrs[i])] = val.String
		}
	}
	return ans, nil
}

func FindBibTitles(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	laConf *vteconf.VTEConf,
	qry biblio.PayloadList,
) (map[string]string, error) {
	if corpusInfo.BibIDAttr == "" || corpusInfo.BibLabelAttr == "" {
		return map[string]string{}, fmt.Errorf("no bib.id/bib.label attribute defined for %s", corpusInfo.Name)
	}
	subcorpAttrs := laconf.GetSubcorpAttrs(laConf)
	selAttrs := make([]string, len(subcorpAttrs))
	for i, attr := range subcorpAttrs {
		selAttrs[i] = utils.ImportKey(attr)
	}

	valuesPlaceholders := make([]string, len(qry.ItemIDs))
	for i := range qry.ItemIDs {
		valuesPlaceholders[i] = "?"
	}
	sql1 := fmt.Sprintf(
		"SELECT %s, %s FROM `%s_liveattrs_entry` WHERE %s IN (%s)",
		utils.ImportKey(corpusInfo.BibIDAttr),
		utils.ImportKey(corpusInfo.BibLabelAttr),
		corpusInfo.GroupedName(),
		utils.ImportKey(corpusInfo.BibIDAttr),
		strings.Join(valuesPlaceholders, ", "),
	)
	sqlVals := make([]any, len(qry.ItemIDs))
	for i, v := range qry.ItemIDs {
		sqlVals[i] = v
	}

	rows, err := db.Query(sql1, sqlVals...)
	ans := make(map[string]string)
	if err == sql.ErrNoRows {
		return ans, ErrorEmptyResult

	} else if err != nil {
		return map[string]string{}, err
	}

	for rows.Next() {
		var bibIdVal, bibLabelVal string
		err = rows.Scan(&bibIdVal, &bibLabelVal)
		if err != nil {
			return nil, err
		}
		ans[bibIdVal] = bibLabelVal
	}

	return ans, nil
}

type DocumentRow struct {
	Idx    int               `json:"idx"`
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Attrs  map[string]string `json:"attrs"`
	NumPos int               `json:"numOfPos"`
}

type PageInfo struct {
	Page     int
	PageSize int
	MaxItems int
}

func (pinfo PageInfo) NumItems() int {
	if pinfo.PageSize == 0 {
		return pinfo.MaxItems
	}
	return pinfo.Page*pinfo.PageSize + 1
}

func (pinfo PageInfo) Offset() int {
	return (pinfo.Page - 1) * pinfo.PageSize
}

func attrsWithPrefix(attrs []string) []string {
	sql := make([]string, len(attrs))
	for i, attr := range attrs {
		sql[i] = "t1." + utils.ImportKey(attr)
	}
	return sql
}

func mkPlaceholder(times int) string {
	ans := make([]string, times)
	for i := 0; i < times; i++ {
		ans[i] = "?"
	}
	return strings.Join(ans, ", ")
}

func extractRegexp(v map[string]any) string {
	v2, ok := v["regexp"]
	if ok {
		v3, ok := v2.(string)
		if ok {
			return v3
		}
	}
	return ""
}

func attrsToSQL(attrs query.Attrs) (string, []any) {
	if len(attrs) == 0 {
		return "1", []any{}
	}
	sql := make([]string, 0, len(attrs))
	sqlValues := make([]any, 0, len(attrs)*2)
	for attr, values := range attrs {
		exclude := strings.HasPrefix(attr, "!")
		switch tValues := values.(type) {
		case []any:
			if len(tValues) > 0 {
				if exclude {
					sql = append(
						sql,
						fmt.Sprintf(" t1.%s NOT IN (%s) ", utils.ImportKey(attr), mkPlaceholder(len(tValues))),
					)

				} else {
					sql = append(
						sql,
						fmt.Sprintf(" t1.%s IN (%s) ", utils.ImportKey(attr), mkPlaceholder(len(tValues))),
					)
				}
				for _, v := range tValues {
					sqlValues = append(sqlValues, v)
				}
			}
		case map[string]any:
			v := extractRegexp(tValues)
			if v != "" {
				if exclude {
					sql = append(
						sql,
						fmt.Sprintf("t1.%s NOT REGEXP ?", utils.ImportKey(attr)),
					)

				} else {
					sql = append(
						sql,
						fmt.Sprintf("t1.%s REGEXP ?", utils.ImportKey(attr)),
					)
				}
				sqlValues = append(sqlValues, v)

			} else {
				log.Error().Msgf("Incorrect value passed as attribute value filter - map[string]any should contain only 'regexp'")
			}
		default:
			panic(fmt.Sprintf("cannot process non-list attribute values; found: %s", reflect.TypeOf(values)))
		}
	}
	return strings.Join(sql, " AND "), sqlValues
}

func buildQuery(
	selection []string,
	corpusInfo *corpus.DBInfo,
	alignedCorpora []string,
	filterAttrs query.Attrs,
) (string, []any) {
	sql := strings.Builder{}
	sql.WriteString(fmt.Sprintf(
		"SELECT %s FROM `%s_liveattrs_entry` AS t1 ",
		strings.Join(selection, ", "), corpusInfo.GroupedName(),
	))
	whereSQL := make([]string, 0, len(alignedCorpora))
	queryArgs := make([]any, 0, len(alignedCorpora)+2)
	for i, item := range alignedCorpora {
		sql.WriteString(fmt.Sprintf(
			" INNER JOIN `%s_liveattrs_entry` AS t%d ON t1.item_id = t%d.item_id AND t%d.corpus_id = ? ", corpusInfo.GroupedName(),
			i+2, i+2, i+2,
		))
		queryArgs = append(queryArgs, item)
	}
	sql.WriteString(" WHERE t1.corpus_id = ? ")
	queryArgs = append(queryArgs, corpusInfo.Name)
	for _, w := range whereSQL {
		sql.WriteString(" AND " + w)
	}
	aSql, aValues := attrsToSQL(filterAttrs)
	sql.WriteString(" AND " + aSql)
	queryArgs = append(queryArgs, aValues...)
	sql.WriteString(fmt.Sprintf(" GROUP BY t1.%s", utils.ImportKey(corpusInfo.BibIDAttr)))
	return sql.String(), queryArgs
}

func GetNumOfDocuments(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	alignedCorpora []string,
	attrs query.Attrs,
) (int, error) {
	sql, args := buildQuery([]string{"t1.*"}, corpusInfo, alignedCorpora, attrs)
	wsql := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS docitems", sql)
	row := db.QueryRow(wsql, args...)
	var ans int
	err := row.Scan(&ans)
	if err != nil {
		return 0, err
	}
	return ans, nil
}

func mkAttrs(attrs []string) map[string]string {
	ans := make(map[string]string)
	for _, v := range attrs {
		ans[v] = ""
	}
	return ans
}

func GetDocuments(
	db *sql.DB,
	corpusInfo *corpus.DBInfo,
	viewAttrs []string,
	alignedCorpora []string,
	filterAttrs query.Attrs,
	page PageInfo,
) ([]*DocumentRow, error) {
	wpAttrs := attrsWithPrefix(viewAttrs)
	selAttrs := make([]string, 0, len(wpAttrs)+2)
	selAttrs = append(
		selAttrs,
		fmt.Sprintf("t1.%s AS item_id", utils.ImportKey(corpusInfo.BibIDAttr)),
	)
	selAttrs = append(
		selAttrs,
		fmt.Sprintf("t1.%s AS item_label", utils.ImportKey(corpusInfo.BibLabelAttr)),
	)
	selAttrs = append(selAttrs, "SUM(t1.poscount)")
	selAttrs = append(selAttrs, wpAttrs...)
	sqlq, args := buildQuery(selAttrs, corpusInfo, alignedCorpora, filterAttrs)
	//page.ToSQL(), TODO
	rows, err := db.Query(sqlq, args...)
	if err == sql.ErrNoRows {
		return []*DocumentRow{}, nil

	} else if err != nil {
		return []*DocumentRow{}, err
	}
	if page.MaxItems == 0 {
		var err error
		page.MaxItems, err = GetNumOfDocuments(db, corpusInfo, alignedCorpora, filterAttrs)
		if err != nil {
			return []*DocumentRow{}, err
		}
	}
	ans := make([]*DocumentRow, 0, page.NumItems())
	attrVals := make([]sql.NullString, len(viewAttrs))
	scanVals := make([]any, 3+len(viewAttrs))

	i := page.Offset()
	for rows.Next() {
		docEntryLabel := sql.NullString{}
		docEntry := &DocumentRow{Idx: i}
		docEntry.Attrs = mkAttrs(viewAttrs)
		scanVals[0] = &docEntry.ID
		scanVals[1] = &docEntryLabel
		scanVals[2] = &docEntry.NumPos

		if docEntryLabel.Valid {
			docEntry.Label = docEntryLabel.String
		}

		for i := range attrVals {
			scanVals[3+i] = &attrVals[i]
		}
		err := rows.Scan(scanVals...)
		if err != nil {
			return []*DocumentRow{}, err
		}
		for i := 3; i < len(scanVals); i++ {
			if v, ok := scanVals[i].(*sql.NullString); ok {
				if v.Valid {
					docEntry.Attrs[viewAttrs[i-3]] = v.String
				}
			}
		}
		ans = append(ans, docEntry)
		i++
	}
	return ans, nil
}
