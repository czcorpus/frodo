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
//
// This struct tracks column usage in liveattrs search.
// We use it to optimize db indexes.

package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"frodo/corpus"
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/utils"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type RequestData struct {
	CorpusID string
	Payload  query.Payload
	Created  time.Time
	IsCached bool
	ProcTime time.Duration
}

func (rd RequestData) toZeroLog(evt *zerolog.Event) {
	evt.
		Bool("isQuery", true).
		Str("corpus", rd.CorpusID).
		Strs("alignedCorpora", rd.Payload.Aligned).
		Bool("isAutocomplete", rd.Payload.AutocompleteAttr != "").
		Bool("isCached", rd.IsCached).
		Float64("procTimeSecs", rd.ProcTime.Seconds()).
		Msg("")
}

type StructAttrUsage struct {
	db      *sql.DB
	channel <-chan RequestData
}

func (sau *StructAttrUsage) RunHandler() {
	for data := range sau.channel {
		data.toZeroLog(log.Info())
		if !data.IsCached {
			err := sau.save(data)
			if err != nil {
				log.Error().Err(err).Msg("Unable to save struct. attrs usage data")
			}
		}
	}
}

func (sau *StructAttrUsage) save(data RequestData) error {
	sql_template := "INSERT INTO `usage` (`corpus_id`, `structattr_name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `num_used`=`num_used`+1"
	context, err := sau.db.Begin()
	if err != nil {
		return err
	}
	for attr := range data.Payload.Attrs {
		_, err := context.Query(sql_template, data.CorpusID, utils.ImportKey(attr))
		if err != nil {
			return err
		}
	}
	context.Commit()
	return nil
}

func NewStructAttrUsage(laDB *sql.DB, saveData <-chan RequestData) *StructAttrUsage {
	return &StructAttrUsage{
		db:      laDB,
		channel: saveData,
	}
}

func LoadUsage(laDB *sql.DB, corpusId string) (map[string]int, error) {
	rows, err := laDB.Query("SELECT `structattr_name`, `num_used` FROM `usage` WHERE `corpus_id` = ?", corpusId)
	ans := make(map[string]int)
	if err == sql.ErrNoRows {
		return ans, nil

	} else if err != nil {
		return nil, err
	}

	for rows.Next() {
		var structattrName string
		var numUsed int
		if err := rows.Scan(&structattrName, &numUsed); err != nil {
			return nil, err
		}
		ans[structattrName] = numUsed
	}
	return ans, nil
}

// --

type updIdxResult struct {
	UsedIndexes    []string
	RemovedIndexes []string
	Error          error
}

func (res *updIdxResult) MarshalJSON() ([]byte, error) {
	var errStr string
	if res.Error != nil {
		errStr = res.Error.Error()
	}
	return json.Marshal(struct {
		UsedIndexes    []string `json:"usedIndexes"`
		RemovedIndexes []string `json:"removedIndexes"`
		Error          string   `json:"error,omitempty"`
	}{
		UsedIndexes:    res.UsedIndexes,
		RemovedIndexes: res.RemovedIndexes,
		Error:          errStr,
	})
}

// --

func UpdateIndexes(laDB *sql.DB, corpusInfo *corpus.DBInfo, maxColumns int) updIdxResult {
	// get most used columns
	rows, err := laDB.Query(
		"SELECT structattr_name "+
			"FROM `usage` "+
			"WHERE corpus_id = ? AND num_used > 0 ORDER BY num_used DESC LIMIT ?",
		corpusInfo.Name, maxColumns,
	)
	if err != nil && err != sql.ErrNoRows {
		return updIdxResult{Error: err}
	}
	columns := make([]string, 0, maxColumns)
	for rows.Next() {
		var structattrName string
		if err := rows.Scan(&structattrName); err != nil {
			return updIdxResult{Error: err}
		}
		columns = append(columns, structattrName)
	}

	// create indexes if necessary with `_autoindex` appendix
	var sqlTemplate string
	if corpusInfo.GroupedName() == corpusInfo.Name {
		sqlTemplate = "CREATE INDEX IF NOT EXISTS `%s` ON `%s_liveattrs_entry` (`%s`)"
	} else {
		sqlTemplate = "CREATE INDEX IF NOT EXISTS `%s` ON `%s_liveattrs_entry` (`%s`, `corpus_id`)"
	}
	usedIndexes := make([]any, len(columns))
	context, err := laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for i, column := range columns {
		usedIndexes[i] = fmt.Sprintf("%s_autoindex", column)
		_, err := context.Query(fmt.Sprintf(sqlTemplate, usedIndexes[i], corpusInfo.GroupedName(), column))
		if err != nil {
			return updIdxResult{Error: err}
		}
	}
	context.Commit()

	// get remaining unused indexes with `_autoindex` appendix
	valuesPlaceholders := make([]string, len(usedIndexes))
	for i := 0; i < len(valuesPlaceholders); i++ {
		valuesPlaceholders[i] = "?"
	}
	sqlTemplate = fmt.Sprintf(
		"SELECT INDEX_NAME FROM information_schema.statistics where TABLE_NAME = ? AND INDEX_NAME LIKE '%%_autoindex' AND INDEX_NAME NOT IN (%s)",
		strings.Join(valuesPlaceholders, ", "),
	)
	values := append([]any{fmt.Sprintf("`%s_liveattrs_entry`", corpusInfo.GroupedName())}, usedIndexes...)
	rows, err = laDB.Query(sqlTemplate, values...)
	if err != nil && err != sql.ErrNoRows {
		return updIdxResult{Error: err}
	}
	unusedIndexes := make([]string, 0, 10)
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return updIdxResult{Error: err}
		}
		unusedIndexes = append(unusedIndexes, indexName)
	}

	// drop unused indexes
	sqlTemplate = "DROP INDEX %s ON `%s_liveattrs_entry`"
	context, err = laDB.Begin()
	if err != nil {
		return updIdxResult{Error: err}
	}
	for _, index := range unusedIndexes {
		_, err := context.Query(fmt.Sprintf(sqlTemplate, index, corpusInfo.GroupedName()))
		if err != nil {
			return updIdxResult{Error: err}
		}
	}
	context.Commit()

	ans := updIdxResult{
		UsedIndexes:    make([]string, len(usedIndexes)),
		RemovedIndexes: unusedIndexes,
	}
	for i, v := range usedIndexes {
		ans.UsedIndexes[i] = v.(string)
	}
	return ans
}
