// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Charles University, Faculty of Arts,
//                Department of Linguistics
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

package ltsearch

import "github.com/czcorpus/vert-tagextract/v3/livetokens"

var profiles = map[string]livetokens.AttrList{

	"intercorp_v13ud": []livetokens.Attr{
		{Name: "upos"},
		{Name: "feats", IsUDFeats: true},
		{Name: "deprel"},
	},
	"intercorp_v16ud": []livetokens.Attr{
		{Name: "upos"},
		{Name: "feats", IsUDFeats: true},
		{Name: "deprel"},
	},
}
