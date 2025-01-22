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

package subcmixer

import (
	"fmt"
	"strings"
)

var (
	operators = map[string]string{"==": "<>", "<>": "==", "<=": ">=", ">=": "<="}
)

type CategoryExpression struct {
	attr  string
	Op    string
	value string
}

func (ce *CategoryExpression) String() string {
	return fmt.Sprintf("%s %s '%s'", ce.attr, ce.Op, ce.value)
}

func (ce *CategoryExpression) Negate() AbstractExpression {
	ans, _ := NewCategoryExpression(ce.attr, operators[ce.Op], ce.value)
	return ans
}

func (ce *CategoryExpression) IsComposed() bool {
	return false
}

func (ce *CategoryExpression) GetAtoms() []AbstractAtomicExpression {
	return []AbstractAtomicExpression{ce}
}

func (ce *CategoryExpression) IsEmpty() bool {
	return ce.attr == "" && ce.Op == "" && ce.value == ""
}

func (ce *CategoryExpression) Add(other AbstractExpression) {
	panic("adding value to a non-composed expression type CategoryExpression")
}

func (ce *CategoryExpression) OpSQL() string {
	if ce.Op == "==" {
		return "="
	}
	return ce.Op
}

func (ce *CategoryExpression) Attr() string {
	return ce.attr
}

func (ce *CategoryExpression) Value() string {
	return ce.value
}

func NewCategoryExpression(attr, op, value string) (*CategoryExpression, error) {
	_, ok := operators[op]
	if !ok {
		return &CategoryExpression{}, fmt.Errorf("invalid operator: %s", op)
	}
	return &CategoryExpression{
		attr:  strings.Replace(attr, ".", "_", 1),
		Op:    op,
		value: value,
	}, nil
}
