// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Institute of the Czech National Corpus,
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
	"fmt"
	"frodo/corpus"
	"frodo/general/collections"
	"frodo/liveattrs/db/qbuilder/laquery"
	"frodo/liveattrs/laconf"
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/request/response"
	"frodo/liveattrs/utils"
	"reflect"

	"github.com/rs/zerolog/log"
)

func groupBibItems(data *response.QueryAns, bibLabel string) {
	grouping := make(map[string]*response.ListedValue)
	entry := data.AttrValues[bibLabel]
	tEntry, ok := entry.([]*response.ListedValue)
	if !ok {
		return
	}
	for _, item := range tEntry {
		val, ok := grouping[item.Label]
		if ok {
			grouping[item.Label].Count += val.Count
			grouping[item.Label].Grouping++

		} else {
			grouping[item.Label] = item
		}
		if grouping[item.Label].Grouping > 1 {
			grouping[item.Label].ID = "@" + grouping[item.Label].Label
		}
	}
	data.AttrValues[bibLabel] = make([]*response.ListedValue, 0, len(grouping))
	for _, v := range grouping {
		entry, ok := (data.AttrValues[bibLabel]).([]*response.ListedValue)
		if !ok {
			continue
		}
		data.AttrValues[bibLabel] = append(entry, v)
	}
}

func (a *Actions) getAttrValues(
	corpusInfo *corpus.DBInfo,
	qry query.Payload,
) (*response.QueryAns, error) {

	laConf, err := a.laConfCache.Get(corpusInfo.Name) // set(self._get_subcorp_attrs(corpus))
	if err != nil {
		return nil, err
	}
	srchAttrs := collections.NewSet(laconf.GetSubcorpAttrs(laConf)...)
	expandAttrs := collections.NewSet[string]()
	if corpusInfo.BibLabelAttr != "" {
		srchAttrs.Add(corpusInfo.BibLabelAttr)
	}
	// if in autocomplete mode then always expand list of the target column
	if qry.AutocompleteAttr != "" {
		a := utils.ImportKey(qry.AutocompleteAttr)
		srchAttrs.Add(a)
		expandAttrs.Add(a)
		acVals, err := qry.Attrs.GetListingOf(qry.AutocompleteAttr)
		if err != nil {
			return nil, err
		}
		qry.Attrs[qry.AutocompleteAttr] = fmt.Sprintf("%%%s%%", acVals[0])
	}
	// also make sure that range attributes are expanded to full lists
	for attr := range qry.Attrs {
		if _, air := qry.Attrs.GetRegexpAttrVal(attr); air {
			expandAttrs.Add(utils.ImportKey(attr))
		}
	}
	qBuilder := &laquery.LAFilter{
		CorpusInfo:          corpusInfo,
		AttrMap:             qry.Attrs,
		SearchAttrs:         srchAttrs.ToOrderedSlice(),
		AlignedCorpora:      qry.Aligned,
		AutocompleteAttr:    qry.AutocompleteAttr,
		EmptyValPlaceholder: emptyValuePlaceholder,
	}
	dataIterator := laquery.DataIterator{
		DB:      a.laDB.DB(),
		Builder: qBuilder,
	}

	ans := response.QueryAns{
		Poscount:   0,
		AttrValues: make(map[string]any),
	}

	for _, sattr := range qBuilder.SearchAttrs {
		ans.AttrValues[sattr] = make([]*response.ListedValue, 0, 100)
	}
	// 1) values collected one by one are collected in tmp_ans and then moved to 'ans'
	//    with some exporting tweaks
	// 2) in case of values exceeding max. allowed list size we just accumulate their size
	//    directly to ans[attr]
	// {attr_id: {attr_val: num_positions,...},...}
	tmpAns := make(map[string]map[string]*response.ListedValue)
	bibID := utils.ImportKey(qBuilder.CorpusInfo.BibIDAttr)
	nilCol := make(map[string]int)
	err = dataIterator.Iterate(func(row laquery.ResultRow) error {
		ans.Poscount += row.Poscount
		for dbKey, dbVal := range row.Attrs {
			colKey := utils.ExportKey(dbKey)
			switch tColVal := ans.AttrValues[colKey].(type) {
			case []*response.ListedValue:
				var valIdent string
				if colKey == corpusInfo.BibLabelAttr {
					valIdent = row.Attrs[bibID]

				} else {
					valIdent = row.Attrs[dbKey]
				}
				attrVal := response.ListedValue{
					ID:         valIdent,
					ShortLabel: utils.ShortenVal(dbVal, shortLabelMaxLength),
					Label:      dbVal,
					Grouping:   1,
				}
				_, ok := tmpAns[colKey]
				if !ok {
					tmpAns[colKey] = make(map[string]*response.ListedValue)
				}
				currAttrVal, ok := tmpAns[colKey][attrVal.ID]
				if ok {
					currAttrVal.Count += row.Poscount

				} else {
					attrVal.Count = row.Poscount
					tmpAns[colKey][attrVal.ID] = &attrVal
				}
			case int:
				ans.AttrValues[colKey] = tColVal + row.Poscount
			case nil:
				nilCol[dbKey]++
			default:
				return fmt.Errorf(
					"invalid value type for attr %s for data iterator: %s",
					colKey, reflect.TypeOf(ans.AttrValues[colKey]),
				)
			}
		}
		return nil
	})
	for k, num := range nilCol {
		log.Error().
			Str("column", k).
			Int("occurrences", num).
			Msgf("liveAttributes getAttrValues encountered nil column")
	}
	if err != nil {
		return &ans, err
	}
	for attr, v := range tmpAns {
		for _, c := range v {
			if err := ans.AddListedValue(attr, c); err != nil {
				return nil, fmt.Errorf("failed to execute getAttrValues(): %w", err)
			}
		}
	}
	// now each line contains: (shortened_label, identifier, label, num_grouped_items, num_positions)
	// where num_grouped_items is initialized to 1
	if corpusInfo.BibGroupDuplicates > 0 {
		groupBibItems(&ans, corpusInfo.BibLabelAttr)
	}
	maxAttrListSize := qry.MaxAttrListSize
	if maxAttrListSize == 0 {
		maxAttrListSize = dfltMaxAttrListSize
	}

	if qry.ApplyCutoff {
		ans.CutoffValues(maxAttrListSize)
	}

	response.ExportAttrValues(
		&ans,
		qBuilder.AlignedCorpora,
		expandAttrs.ToOrderedSlice(),
		corpusInfo.Locale,
		maxAttrListSize,
	)
	return &ans, nil
}
