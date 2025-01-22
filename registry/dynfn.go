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

package registry

import (
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
)

// note: the function description data are taken from https://www.sketchengine.eu/dynamic-functions/

// DynFn describes Manatee dynamic function
// (either an internal one or some external)
type DynFn struct {
	Name        string
	Args        []string
	Dynlib      string
	Description string
}

// Funtype produces function type codes as required in
// corpus registry files. Please note that the codes are
// based only on arguments starting from the second one as
// the first argument is always the same (the source string).
func (df *DynFn) Funtype() string {
	usedArgs := df.Args[1:]
	ans := make([]string, len(usedArgs))
	for i, arg := range usedArgs {
		switch arg {
		case "str":
			ans[i] = "s"
		case "n":
			ans[i] = "i"
		case "c":
			ans[i] = "c"
		case "pointer":
			ans[i] = "0"
		case "locale":
			ans[i] = "s"
		case "enc":
			ans[i] = "s"
		default:
			log.Warn().Msgf("Invalid Arg type: %s", arg)
			ans[i] = "?"
		}
	}
	return strings.Join(ans, "")
}

func (df DynFn) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Name        string   `json:"name"`
		Args        []string `json:"args"`
		Dynlib      string   `json:"dynlib"`
		Description string   `json:"description"`
		Funtype     string   `json:"funtype"`
	}{
		Name:        df.Name,
		Args:        df.Args,
		Dynlib:      df.Dynlib,
		Description: df.Description,
		Funtype:     df.Funtype(),
	})
}

var dynFnList = []DynFn{
	{"striplastn", []string{"str", "n"}, "internal", "returns str striped from last n characters"},
	{"lowercase", []string{"str", "locale"}, "internal", "returns str in lowercase (for any single-byte encoding and the corresponding locale)"},
	{"utf8lowercase", []string{"str"}, "internal", "returns str in lowercase (for any utf-8 encoded string str)"},
	{"utf8uppercase", []string{"str"}, "internal", "returns str in uppercase (for any utf-8 encoded string str)"},
	{"utf8capital", []string{"str"}, "internal", "returns str with first character capitalized (for any utf-8 encoded string str)"},
	{"getfirstn", []string{"str", "n"}, "internal", "returns first n characters of str"},
	{"getlastn", []string{"str", "n"}, "internal", "returns last n characters of str (for any single-byte encoding)"},
	{"utf8getlastn", []string{"str", "n"}, "internal", "returns last n characters of str (for any utf-8 encoded string)"},
	{"getfirstbysep", []string{"str", "c"}, "internal", "returns prefix of str up to the character c (excluding)"},
	{"getnbysep", []string{"str", "c", "n"}, "internal", "returns n-th component of str according to the delimiter c (excluding)"},
	{"getnchar", []string{"str", "n"}, "internal", "returns n-th character of str"},
	{"getnextchars", []string{"str", "c", "n"}, "internal", "returns n characters after character c"},
	{"getnextchar", []string{"str", "c"}, "internal", "returns the character after character c"},
	{"url2domain", []string{"str", "n"}, "internal", "returns n-th component of the URL (0 = web domain, 1 = top level domain, 2 = second level domain)"},
	{"ascii", []string{"str", "enc", "locale"}, "internal", "returns ASCII transliteration of the string according to the given encoding and locale"},
}

type dynlibItem struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}
