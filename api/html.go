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

package api

import (
	"html/template"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
)

type AlarmPage struct {
	InstanceID string
	RootURL    string
	AlarmID    string
	Error      error
}

const (
	alarmPage = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>KonText alarm</title>
		<style type="text/css">
		html {
			background-color: #000000;
		}
		body {
			font-size: 1.2em;
			width: 40em;
			margin: 0 auto;
			font-family: sans-serif;
			color: #EEEEEE;
		}
		h1 {
			font-size: 1.7em;
		}
		</style>
	</head>
	<body>
		<h1>FRODO</h1>
		{{ if .Error }}
		<p><strong className="err">ERROR:</strong> {{ .Error }}</p>
		{{ else }}
		<p>The active ALARM {{ .AlarmID }} of a monitored KonText instance <strong>{{ .InstanceID}}</strong> has been
		turned OFF. Please make sure the actual problem is solved.</p>
		{{ end }}
	</body>
</html>`
)

var (
	initOnce sync.Once
	tpl      *template.Template
)

func compileAlarmPage() {
	initOnce.Do(func() {
		var err error
		tpl, err = template.New("alarm").Parse(alarmPage)
		if err != nil {
			log.Fatal().Msg("Failed to parse the template")
		}
	})
}

// WriteHTMLResponse writes 'value' to an HTTP response encoded as JSON
func WriteHTMLResponse(w http.ResponseWriter, data *AlarmPage) error {
	compileAlarmPage()
	w.Header().Add("Content-Type", "text/html")
	if data.Error != nil {
		w.WriteHeader(http.StatusBadGateway)
	}
	err := tpl.Execute(w, data)
	if err != nil {
		return err
	}
	return nil
}
