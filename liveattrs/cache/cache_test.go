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

package cache

import (
	"frodo/liveattrs/request/query"
	"frodo/liveattrs/request/response"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createTestingCache create a testing cache with an entry
// for corpus 'corp1' and aligned corpora 'corp2', 'corp3'
func createTestingCache() (*EmptyQueryCache, query.Payload, response.QueryAns) {
	qcache := NewEmptyQueryCache()
	qry := query.Payload{
		Aligned: []string{"corp2", "corp3"},
	}
	value := response.QueryAns{
		AlignedCorpora: []string{"corp2", "corp3"},
		AttrValues: map[string]any{
			"attrA": []string{"valA1", "valA2"},
			"attrB": []string{"valB1", "valB2", "valB3"},
		},
	}
	qcache.Set("corp1", qry, &value)
	return qcache, qry, value
}

func TestCacheGet(t *testing.T) {
	qcache, qry, value := createTestingCache()
	v := qcache.Get("corp1", qry)
	assert.Equal(t, *v, value)
	assert.Contains(t, qcache.corpKeyDeps, "corp1")
	assert.Contains(t, qcache.corpKeyDeps, "corp2")
	assert.Contains(t, qcache.corpKeyDeps, "corp3")
}

func TestCacheSet(t *testing.T) {
	qcache, _, _ := createTestingCache()
	qry2 := query.Payload{
		Aligned: []string{"corp1"},
	}
	value := response.QueryAns{
		AlignedCorpora: []string{"corp1"},
		AttrValues: map[string]any{
			"attrA": []string{"valA1", "valA2"},
		},
	}
	qcache.Set("corpX", qry2, &value)
	assert.Equal(t, &value, qcache.Get("corpX", qry2))
}

func TestCacheDel(t *testing.T) {
	qcache, qry, _ := createTestingCache()
	qry2 := query.Payload{
		Aligned: []string{"corp1"},
	}
	value := response.QueryAns{
		AlignedCorpora: []string{"corp1"},
		AttrValues: map[string]any{
			"attrA": []string{"valA1", "valA2"},
		},
	}
	qcache.Set("corp4", qry2, &value)

	qcache.Del("corp1")
	assert.Nil(t, qcache.Get("corp1", qry))
	assert.NotContains(t, qcache.corpKeyDeps, "corp1")
	assert.Len(t, qcache.corpKeyDeps["corp2"], 0)
	assert.NotContains(t, qcache.corpKeyDeps["corp3"], 0)
	assert.Equal(t, 0, len(qcache.data))
}

func TestCacheDelAligned(t *testing.T) {
	qcache, _, _ := createTestingCache()
	qcache.Del("corp1")
	qcache.Del("corp2")
	qcache.Del("corp3")
	assert.Equal(t, 0, len(qcache.data))
	assert.Equal(t, 0, len(qcache.corpKeyDeps))
}
