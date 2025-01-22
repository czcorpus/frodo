// Copyright 2020 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2020 Martin Zimandl <martin.zimandl@gmail.com>
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

package utils

import "strings"

func ShortenVal(v string, maxLength int) string {
	if len(v) <= maxLength {
		return v
	}

	words := strings.Split(v, " ")
	length := 0
	for i, word := range words {
		length += len(word)
		if length > maxLength {
			if i == 0 {
				return word[:maxLength] + "..."
			}
			return strings.Join(words[:i], " ") + "..."
		}
	}
	return v
}

func ImportKey(k string) string {
	return strings.Replace(strings.TrimPrefix(k, "!"), ".", "_", 1)
}

func ExportKey(k string) string {
	return strings.Replace(k, "_", ".", 1)
}
