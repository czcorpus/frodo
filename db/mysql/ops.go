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

package mysql

import (
	"database/sql"
	"time"

	db "github.com/czcorpus/vert-tagextract/v3/db"
	"github.com/go-sql-driver/mysql"
)

type Adapter struct {
	db      *sql.DB
	conf    db.Conf
	dbName  string
	isAdHoc bool
}

func (a *Adapter) DB() *sql.DB {
	return a.db
}

func (a *Adapter) DBName() string {
	return a.dbName
}

func (a *Adapter) Conf() db.Conf {
	return a.conf
}

// Close closes the wrapped database connection.
// Only connections which are not "ad-hoc" can
// be closed this way. This applies e.g. for
// the "import-tuned" connection which is meant
// to live just for the time of import and then
// closed.
// In case the adapter is closed for a non-adhoc
// connection, the method panics.
func (a *Adapter) Close() error {
	if !a.isAdHoc {
		panic("trying to close non-adhoc database Adapter")
	}
	return a.db.Close()
}

func OpenDB(conf db.Conf) (*Adapter, error) {
	mconf := mysql.NewConfig()
	mconf.Net = "tcp"
	mconf.Addr = conf.Host
	mconf.User = conf.User
	mconf.Passwd = conf.Password
	mconf.DBName = conf.Name
	mconf.ParseTime = true
	mconf.Loc = time.Local
	mconf.Params = map[string]string{"autocommit": "true"}
	db, err := sql.Open("mysql", mconf.FormatDSN())
	if err != nil {
		return nil, err
	}
	return &Adapter{db: db, dbName: mconf.DBName, conf: conf}, nil
}

// OpenImportTunedDB creates an Adapter instance with
// undrelying connection session having slightly modified
// parameters suitable for faster data import (unique checks disabled,
// foreign checks disabled).
func OpenImportTunedDB(conf db.Conf) (*Adapter, error) {
	a, err := OpenDB(conf)
	if err != nil {
		return nil, err
	}
	a.isAdHoc = true
	for _, q := range []string{
		"SET SESSION unique_checks = 0",
		"SET SESSION foreign_key_checks = 0",
	} {
		if _, err = a.db.Exec(q); err != nil {
			return nil, err
		}
	}
	return a, nil
}
