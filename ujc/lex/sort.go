// Copyright 2026 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
// Faculty of Arts, Charles University
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

package lex

import (
	"sort"
	"strings"

	"github.com/czcorpus/cnc-gokit/collections"
)

const (
	POSOrder    = "NAPCVDRJTI"
	GenderOrder = "MIBFN"
	AspectOrder = "PIB"
)

func morphologySort(item1 LexItem, item2 LexItem) bool {
	var orderMap, orderData1, orderData2 string
	if item1.Pos == "N" && item2.Pos == "N" {
		// order by gender if both items are nouns
		orderMap, orderData1, orderData2 = GenderOrder, item1.Gender, item2.Gender
	} else if item1.Pos == "V" && item2.Pos == "V" {
		// order by aspect if both items are verbs
		orderMap, orderData1, orderData2 = AspectOrder, item1.Aspect, item2.Aspect
	} else {
		// order by PoS for other items
		orderMap, orderData1, orderData2 = POSOrder, item1.Pos, item2.Pos
	}
	orderIndex1 := strings.Index(orderMap, orderData1)
	orderIndex2 := strings.Index(orderMap, orderData2)
	if orderIndex1 == -1 {
		orderIndex1 = len(orderMap)
	}
	if orderIndex2 == -1 {
		orderIndex2 = len(orderMap)
	}
	return orderIndex1 < orderIndex2
}

func sortVariants(data []LexItem, mainSource Source) []LexItem {
	// Get first items of groups
	firstGroupItems := collections.SliceReduce(data, func(acc []LexItem, curr LexItem, i int) []LexItem {
		groupIdx := collections.SliceFindIndex(acc, func(v LexItem) bool {
			return v.Sources[mainSource][0].ID == curr.Sources[mainSource][0].ID
		})
		if groupIdx == -1 {
			return append(acc, curr)
		}
		if acc[groupIdx].Sources[mainSource][0].GroupOrder > curr.Sources[mainSource][0].GroupOrder {
			acc[groupIdx] = curr
		}
		return acc
	}, make([]LexItem, 0, 5))

	// Sort first items of groups
	sort.Slice(firstGroupItems, func(i, j int) bool {
		// first order by Lemma
		if firstGroupItems[i].Lemma != firstGroupItems[j].Lemma {
			return firstGroupItems[i].Lemma < firstGroupItems[j].Lemma
		}
		// then by homonymy
		if firstGroupItems[i].Sources[mainSource][0].Homonym != firstGroupItems[j].Sources[mainSource][0].Homonym {
			return firstGroupItems[i].Sources[mainSource][0].Homonym < firstGroupItems[j].Sources[mainSource][0].Homonym
		}
		return morphologySort(firstGroupItems[i], firstGroupItems[j])
	})

	// groupID order map
	groupOrder := make(map[string]int)
	for i, v := range firstGroupItems {
		groupOrder[v.Sources[mainSource][0].ID] = i
	}

	// sort groups all data
	sort.Slice(data, func(i, j int) bool {
		if data[i].Sources[mainSource][0].ID != data[j].Sources[mainSource][0].ID {
			return groupOrder[data[i].Sources[mainSource][0].ID] < groupOrder[data[j].Sources[mainSource][0].ID]
		}
		if data[i].Sources[mainSource][0].GroupOrder != data[j].Sources[mainSource][0].GroupOrder {
			return data[i].Sources[mainSource][0].GroupOrder < data[j].Sources[mainSource][0].GroupOrder
		}
		return morphologySort(data[i], data[j])
	})

	return data
}
