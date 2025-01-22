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

package jobs

import (
	"golang.org/x/text/message"
)

func extractJobDescription(printer *message.Printer, info GeneralJobInfo) string {
	desc := "??"
	switch info.GetType() {
	case "ngram-and-qs-generating":
		desc = printer.Sprintf("N-grams and query suggestion data generation")
	case "liveattrs":
		desc = printer.Sprintf("Live attributes data extraction and generation")
	case "dummy-job":
		desc = printer.Sprintf("Testing and debugging empty job")
	default:
		desc = printer.Sprintf("Unknown job")
	}
	return desc
}

func localizedStatus(printer *message.Printer, info GeneralJobInfo) string {
	if info.GetError() == nil {
		return printer.Sprintf("Job finished without errors")
	}
	return printer.Sprintf("Job finished with error: %s", info.GetError())
}
