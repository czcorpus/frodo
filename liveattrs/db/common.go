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
	"errors"
	"fmt"
	"strings"
)

// ErrorEmptyResult is a general representation
// of "nothing found" for any liveattrs db operation.
// It is up to a concrete implementation whether this
// applies for multi-row return values too.
var ErrorEmptyResult = errors.New("no result")

type StructAttr struct {
	Struct string
	Attr   string
}

func (sattr StructAttr) Values() [2]string {
	return [2]string{sattr.Struct, sattr.Attr}
}

func (sattr StructAttr) Key() string {
	return fmt.Sprintf("%s.%s", sattr.Struct, sattr.Attr)
}

// --

func ImportStructAttr(v string) StructAttr {
	tmp := strings.Split(v, ".")
	return StructAttr{Struct: tmp[0], Attr: tmp[1]}
}
