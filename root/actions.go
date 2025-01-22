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
// Actions contains all the server HTTP REST actions

package root

import (
	"encoding/json"
	"frodo/cnf"
	"frodo/general"
	"net/http"
	"os"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	Version general.VersionInfo
	Conf    *cnf.Conf
}

func (a *Actions) OnExit() {}

// RootAction is just an information action about the service
func (a *Actions) RootAction(ctx *gin.Context) {
	host, err := os.Hostname()
	if err != nil {
		host = "#failed_to_obtain"
	}
	ans := struct {
		Name     string              `json:"name"`
		Version  general.VersionInfo `json:"version"`
		Host     string              `json:"host"`
		ConfPath string              `json:"confPath"`
	}{
		Name:     "FRODO - Frequency Registry Of Dictionary Objects",
		Version:  a.Version,
		Host:     host,
		ConfPath: a.Conf.GetSourcePath(),
	}

	resp, err := json.Marshal(ans)
	if err != nil {
		uniresp.WriteJSONErrorResponse(
			ctx.Writer,
			uniresp.NewActionError("failed to run the root action: %w", err),
			http.StatusInternalServerError,
		)
	}
	ctx.Writer.Write(resp)
}
