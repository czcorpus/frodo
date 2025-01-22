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

package kontext

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

func SendSoftReset(conf *Conf) error {
	if conf == nil || len(conf.SoftResetURL) == 0 {
		log.Warn().Msgf("The kontextSoftResetURL configuration not set - ignoring the action")
		return nil
	}
	for _, instance := range conf.SoftResetURL {
		resp, err := http.Post(instance, "application/json", nil)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("kontext instance `%s` soft reset failed - unexpected status code %d", instance, resp.StatusCode)
		}
	}
	return nil
}
