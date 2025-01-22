// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
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

package query

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/fs"
	"github.com/rs/zerolog/log"
)

const (
	DfltConcBackoffInitialInterval = 200 * time.Millisecond
	DfltConcBackoffMaxElapsedTime  = 2 * time.Minute
)

var (
	ErrEntryNotFound    = errors.New("cache entry not found")
	ErrEntryNotReadyYet = errors.New("cache entry not ready yet")
)

type CacheEntry struct {
	PromisedAt  time.Time
	FulfilledAt time.Time
	FilePath    string
	Err         error
}

type Cache struct {
	data              *collections.ConcurrentMap[string, CacheEntry]
	loc               *time.Location
	rootPath          string
	maxEntriesPerCorp int
	nextListenerId    int
	waitLimit         time.Duration
	waitCheckInterval time.Duration
}

func (cache *Cache) mkKey(corpusID, query string) string {
	enc := sha1.New()
	enc.Write([]byte(corpusID))
	enc.Write([]byte(query))
	return hex.EncodeToString(enc.Sum(nil))
}

func (cache *Cache) mkPath(corpusID, query string) string {
	return filepath.Join(cache.rootPath, corpusID, cache.mkKey(corpusID, query))
}

func (cache *Cache) Contains(corpusID, query string) bool {
	return cache.data.HasKey(cache.mkKey(corpusID, query))
}

func (cache *Cache) RestoreUnboundEntries() error {
	log.Info().
		Str("cachePath", cache.rootPath).
		Msg("trying to restore all the unbound cache files")
	corpDirs, err := fs.ListDirsInDir(cache.rootPath, false)
	if err != nil {
		return fmt.Errorf("failed to restore unbound cache records: %w", err)
	}
	now := time.Now().In(cache.loc)
	var iterErr error
	corpDirs.ForEach(func(dirInfo os.FileInfo, _ int) bool {
		subdir := path.Base(dirInfo.Name())
		files, err := fs.ListFilesInDir(path.Join(cache.rootPath, subdir), false)
		if err != nil {
			iterErr = fmt.Errorf("failed to restore unbound cache records: %w", err)
			return false
		}
		files.ForEach(func(finfo os.FileInfo, _ int) bool {
			file := path.Base(finfo.Name())
			entry := CacheEntry{
				PromisedAt:  now,
				FulfilledAt: now,
				FilePath:    path.Join(cache.rootPath, subdir, file),
			}
			cache.data.Set(file, entry)
			return true
		})
		return true
	})
	log.Info().
		Int("numEntries", cache.data.Len()).
		Msg("restored unbound cache entries")
	return iterErr
}

func (cache *Cache) restoreIfUnboundEntry(corpusID, query string) CacheEntry {
	targetPath := cache.mkPath(corpusID, query)
	now := time.Now().In(cache.loc)
	ans := CacheEntry{PromisedAt: now, FulfilledAt: now}
	isFile, err := fs.IsFile(targetPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("path", targetPath).
			Msg("failed to determine cache file status")
		ans.Err = err

	} else if !isFile {
		ans.Err = ErrEntryNotFound

	} else {
		ans.FilePath = targetPath
	}
	return ans
}

func (cache *Cache) Promise(corpusID, query string, fn func(path string) error) <-chan CacheEntry {
	targetPath := cache.mkPath(corpusID, query)
	entry := CacheEntry{
		PromisedAt: time.Now().In(cache.loc),
		FilePath:   targetPath,
	}
	entryKey := cache.mkKey(corpusID, query)
	cache.data.Set(entryKey, entry)
	ans := make(chan CacheEntry)
	go func(entry2 CacheEntry) {
		err := fn(targetPath)
		if err != nil {
			entry2.Err = err
		}
		entry2.FulfilledAt = time.Now().In(cache.loc)
		cache.data.Set(entryKey, entry2)
		ans <- entry2
		close(ans)
	}(entry)
	return ans
}

func (cache *Cache) Get(corpusID, query string) (CacheEntry, error) {
	entryKey := cache.mkKey(corpusID, query)
	operation := func() (CacheEntry, error) {
		entry, ok := cache.data.GetWithTest(entryKey)
		if !ok {
			e := CacheEntry{
				Err:         ErrEntryNotFound,
				FulfilledAt: time.Now().In(cache.loc),
			}
			return e, backoff.Permanent(ErrEntryNotFound)
		}
		if entry.FulfilledAt.IsZero() {
			entry.Err = ErrEntryNotReadyYet
			return entry, nil
		}
		return entry, nil
	}
	bkoff := backoff.NewExponentialBackOff()
	bkoff.InitialInterval = DfltConcBackoffInitialInterval
	bkoff.MaxElapsedTime = DfltConcBackoffMaxElapsedTime
	return backoff.RetryWithData(operation, bkoff)
}

func NewCache(rootPath string, location *time.Location) *Cache {
	return &Cache{
		rootPath:          rootPath,
		loc:               location,
		data:              collections.NewConcurrentMap[string, CacheEntry](),
		waitCheckInterval: time.Millisecond * 500,
		waitLimit:         time.Second * 10,
	}
}
