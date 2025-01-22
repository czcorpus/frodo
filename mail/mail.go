// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package mail

import (
	"fmt"
	"strings"

	cncmail "github.com/czcorpus/cnc-gokit/mail"
)

var (
	NotFoundMsgPlaceholder = "??"
)

type EmailNotification struct {
	cncmail.NotificationConf
}

// LocalizedSignature returns a mail signature based on configuration
// and provided language. It is able to search for 2-character codes
// in case 5-ones are not matching.
// In case nothing is found, the returned message is NotFoundMsgPlaceholder
// (and an error is returned).
func (enConf EmailNotification) LocalizedSignature(lang string) (string, error) {
	if msg, ok := enConf.Signature[lang]; ok {
		return msg, nil
	}
	lang2 := strings.Split(lang, "-")[0]
	for k, msg := range enConf.Signature {
		if strings.Split(k, "-")[0] == lang2 {
			return msg, nil
		}
	}
	return NotFoundMsgPlaceholder, fmt.Errorf("e-mail signature for language %s not found", lang)
}

func (enConf EmailNotification) HasSignature() bool {
	return len(enConf.Signature) > 0
}

func (enConf EmailNotification) DefaultSignature(lang string) string {
	if lang == "cs" || lang == "cs-CZ" {
		return "Váš Frodo"
	}
	return "Yours Frodo"
}
