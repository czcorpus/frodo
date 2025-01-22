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
	"fmt"
	"frodo/liveattrs/db/qbuilder"
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/utils"
	"strings"

	"github.com/rs/zerolog/log"
)

type PredicateArgs struct {
	data                query.Attrs
	bibID               string
	bibLabel            string
	autocompleteAttr    string
	emptyValPlaceholder string
}

func (args *PredicateArgs) Len() int {
	return len(args.data)
}

func (args *PredicateArgs) importValue(value string) string {
	if value == args.emptyValPlaceholder {
		return ""
	}
	return value
}

func (args *PredicateArgs) ExportSQL(itemPrefix, corpusID string) (string, []string) {
	where := make([]string, 0, 20)
	sqlValues := make([]string, 0, 20)
	for dkey, values := range args.data {
		exclude := strings.HasPrefix(dkey, "!")
		key := utils.ImportKey(dkey)
		if args.autocompleteAttr == args.bibLabel && key == args.bibID {
			continue
		}
		cnfItem := make([]string, 0, 20)
		switch tValues := values.(type) {
		case []any:
			for _, value := range tValues {
				tValue, ok := value.(string)
				if !ok {
					continue
				}
				if len(tValue) == 0 || tValue[0] != '@' {
					cnfItem = append(
						cnfItem,
						fmt.Sprintf(
							"%s.%s %s ?",
							itemPrefix, key, qbuilder.CmpOperator(tValue, exclude),
						),
					)
					sqlValues = append(sqlValues, args.importValue(tValue))

				} else {
					cnfItem = append(
						cnfItem,
						fmt.Sprintf(
							"%s.%s %s ?",
							itemPrefix, args.bibLabel,
							qbuilder.CmpOperator(tValue[1:], exclude),
						),
					)
					sqlValues = append(sqlValues, args.importValue(tValue[1:]))
				}
			}
		case string:
			if exclude {
				cnfItem = append(
					cnfItem,
					fmt.Sprintf(
						"%s.%s NOT LIKE ?",
						itemPrefix, key),
				)

			} else {
				cnfItem = append(
					cnfItem,
					fmt.Sprintf(
						"%s.%s LIKE ?",
						itemPrefix, key),
				)
			}
			sqlValues = append(sqlValues, args.importValue(tValues))
		case map[string]any:
			regexpVal, ok := args.data.GetRegexpAttrVal(dkey)
			if ok {
				if exclude {
					cnfItem = append(cnfItem, fmt.Sprintf("%s.%s NOT REGEXP ?", itemPrefix, key))
				} else {
					cnfItem = append(cnfItem, fmt.Sprintf("%s.%s REGEXP ?", itemPrefix, key))
				}
				sqlValues = append(sqlValues, args.importValue(regexpVal))

				// TODO add support for this
			} else {
				// TODO handle in a better way
				log.Error().Msgf(
					"failed to determine type of liveattrs attribute %s (corpus %s)", key, corpusID)
			}
		default: // TODO can this even happen???
			cnfItem = append(
				cnfItem,
				fmt.Sprintf(
					"LOWER(%s.%s) %s LOWER(?)",
					itemPrefix, key, qbuilder.CmpOperator(fmt.Sprintf("%v", tValues), exclude),
				),
			)
			sqlValues = append(sqlValues, args.importValue(fmt.Sprintf("%v", tValues)))
		}

		if len(cnfItem) > 0 {
			if exclude {
				where = append(where, fmt.Sprintf("(%s)", strings.Join(cnfItem, " AND ")))
			} else {
				where = append(where, fmt.Sprintf("(%s)", strings.Join(cnfItem, " OR ")))
			}
		}
	}
	where = append(where, fmt.Sprintf("%s.corpus_id = ?", itemPrefix))
	sqlValues = append(sqlValues, corpusID)
	return strings.Join(where, " AND "), sqlValues
}

type QueryComponents struct {
	sqlTemplate   string
	selectedAttrs []string
	hiddenAttrs   []string
	whereValues   []string
}
