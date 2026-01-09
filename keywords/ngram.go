package keywords

import (
	"fmt"
	"unicode"

	"github.com/czcorpus/vert-tagextract/v3/ptcount"
)

type Ngram interface {
	UniqueID() string
	TokenAt(idx int) *token
	IncCount()
	SafeCount() int
	SafeEffectSize() float64
	SetEffectSize(v float64)
	AppendToken(tk token)
	SetDistance(currToken int)
	IsPropname() bool
	TestAndSetPropnameFlag(wordDict *ptcount.WordDict)
	TestAndAdoptNominative(other Ngram)
	ARF() float64
	Len() int
	String() string
	WordAsVector() []int
	Preview(wordDict *ptcount.WordDict) string
}

// ---------

type ngramBase struct {
	corpusSize    int
	Count         int
	EffectSize    float64
	IsProperName  bool
	LastPos       int
	totalDistance int
	minDistance   float64
	arf           float64
}

func (ng *ngramBase) IncCount() {
	ng.Count++
}

func (ng *ngramBase) SafeCount() int {
	if ng == nil {
		return 0
	}
	return ng.Count
}

func (ng *ngramBase) SafeEffectSize() float64 {
	if ng == nil {
		return 0
	}
	return ng.EffectSize
}

func (ng *ngramBase) SetDistance(currToken int) {
	newDist := currToken - ng.LastPos
	ng.totalDistance += newDist
	avgDist := float64(ng.corpusSize) / float64(ng.Count)
	ng.minDistance += min(float64(newDist), avgDist)
	ng.LastPos = currToken
}

func (ng *ngramBase) ARF() float64 {
	if ng == nil {
		return 0
	}
	return ng.minDistance / (float64(ng.corpusSize) / float64(ng.Count))
}

func (ng *ngramBase) SetEffectSize(v float64) {
	ng.EffectSize = v
}

func (ng *ngramBase) IsPropname() bool {
	return ng.IsProperName
}

// ----------------------

type Unigram struct {
	ngramBase
	tokens [1]token
}

func (ng *Unigram) Len() int {
	return 1
}

func newUnigram(corpusSize int) *Unigram {
	return &Unigram{
		ngramBase: ngramBase{corpusSize: corpusSize},
	}
}

func (ng *Unigram) TokenAt(idx int) *token {
	if idx > 0 {
		panic("accessing too high index token in Unigram")
	}
	return &ng.tokens[idx]
}

func (ng *Unigram) UniqueID() string {
	return fmt.Sprintf("[1]:%d", ng.tokens[0].Lemma)
}

func (ng *Unigram) AppendToken(tk token) {
	for i := range len(ng.tokens) {
		if ng.tokens[i].IsZero() {
			ng.tokens[i] = tk
			break
		}
	}
}

func (ng *Unigram) TestAndSetPropnameFlag(wordDict *ptcount.WordDict) {
	tmp1 := []rune(wordDict.Get(ng.tokens[0].Lemma))
	ng.IsProperName = unicode.IsUpper(tmp1[0])
}

func (ng *Unigram) TestAndAdoptNominative(other Ngram) {
	if other.Len() != 1 {
		panic("cannot TestAndAdoptNominative for different n-gram types")
	}
	if !ng.tokens[0].IsNominative() && other.TokenAt(0).IsNominative() {
		ng.tokens[0].Word = other.TokenAt(0).Word
		ng.tokens[0].Case = other.TokenAt(0).Case
	}
}

func (ng *Unigram) Preview(wordDict *ptcount.WordDict) string {
	return fmt.Sprintf(
		"[U]> w: %s, lm: %s, freq: %d, ARF: %.2f, ES: %.2f, CS: %d\n",
		wordDict.Get(ng.tokens[0].Word),
		wordDict.Get(ng.tokens[0].Lemma),
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
		ng.corpusSize,
	)
}

func (ng *Unigram) WordAsVector() []int {
	return []int{ng.tokens[0].Word}
}

func (ng *Unigram) String() string {
	return fmt.Sprintf(
		"[U]> w: %d, lm: %d, freq: %d, ARF: %.2f, ES: %.2f\n",
		ng.tokens[0].Word,
		ng.tokens[0].Lemma,
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
	)
}

func (ng *Unigram) ARF() float64 {
	if ng == nil {
		return 0
	}
	return ng.ngramBase.ARF()
}

// -------------------------

type Bigram struct {
	ngramBase
	tokens [2]token
}

func (ng *Bigram) Len() int {
	return 2
}

func (ng *Bigram) UniqueID() string {
	return fmt.Sprintf("[2]%d:%d", ng.tokens[0].Lemma, ng.tokens[1].Lemma)
}

func (ng *Bigram) TokenAt(idx int) *token {
	if idx > 1 {
		panic("accessing too high index token in Bigram")
	}
	return &ng.tokens[idx]
}

func (ng *Bigram) AppendToken(tk token) {
	for i := range len(ng.tokens) {
		if ng.tokens[i].IsZero() {
			ng.tokens[i] = tk
			break
		}
	}
}

func (ng *Bigram) TestAndSetPropnameFlag(wordDict *ptcount.WordDict) {
	tmp1 := []rune(wordDict.Get(ng.tokens[0].Lemma))
	tmp2 := []rune(wordDict.Get(ng.tokens[1].Lemma))
	ng.IsProperName = unicode.IsUpper(tmp1[0]) || unicode.IsUpper(tmp2[0])
}

func (ng *Bigram) TestAndAdoptNominative(other Ngram) {
	if other.Len() != 2 {
		panic("cannot TestAndAdoptNominative for different n-gram types")
	}
	if !ng.tokens[0].IsNominative() && other.TokenAt(0).IsNominative() {
		ng.tokens[0].Word = other.TokenAt(0).Word
		ng.tokens[0].Case = other.TokenAt(0).Case
		ng.tokens[1].Word = other.TokenAt(1).Word
		ng.tokens[1].Case = other.TokenAt(1).Case
	}
}

func (ng *Bigram) WordAsVector() []int {
	ans := make([]int, 0, 2*len(ng.tokens))
	for i, s := range ng.tokens {
		if i > 0 && s.AuxWord > 0 {
			ans = append(ans, s.AuxWord)
		}
		ans = append(ans, s.Word)
	}
	return ans
}

func (ng *Bigram) Preview(wordDict *ptcount.WordDict) string {
	return fmt.Sprintf(
		"[B]> w: %s %s, lm: %s %s, freq: %d, ARF: %.2f, ES: %.2f, CS: %d\n",
		wordDict.Get(ng.tokens[0].Word),
		wordDict.Get(ng.tokens[1].Word),
		wordDict.Get(ng.tokens[0].Lemma),
		wordDict.Get(ng.tokens[1].Lemma),
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
		ng.corpusSize,
	)
}

func (ng *Bigram) String() string {
	return fmt.Sprintf(
		"[B]> w: %d %d, lm: %d %d, freq: %d, ARF: %.2f, ES: %.2f\n",
		ng.tokens[0].Word,
		ng.tokens[1].Word,
		ng.tokens[0].Lemma,
		ng.tokens[1].Lemma,
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
	)
}

func (ng *Bigram) ARF() float64 {
	if ng == nil {
		return 0
	}
	return ng.ngramBase.ARF()
}

// --------------------------

type Trigram struct {
	ngramBase
	tokens [3]token
}

func (ng *Trigram) Len() int {
	return 3
}

func (ng *Trigram) UniqueID() string {
	return fmt.Sprintf("[3]%d:%d:%d", ng.tokens[0].Lemma, ng.tokens[1].Lemma, ng.tokens[2].Lemma)
}

func (ng *Trigram) TokenAt(idx int) *token {
	if idx > 2 {
		panic("accessing too high index token in Trigram")
	}
	return &ng.tokens[idx]
}

func (ng *Trigram) AppendToken(tk token) {
	for i := range len(ng.tokens) {
		if ng.tokens[i].IsZero() {
			ng.tokens[i] = tk
			break
		}
	}
}

func (ng *Trigram) WordAsVector() []int {
	ans := make([]int, 0, 2*len(ng.tokens))
	for i, s := range ng.tokens {
		if i > 0 && s.AuxWord > 0 {
			ans = append(ans, s.AuxWord)
		}
		ans = append(ans, s.Word)
	}
	return ans
}

func (ng *Trigram) TestAndSetPropnameFlag(wordDict *ptcount.WordDict) {
	tmp1 := []rune(wordDict.Get(ng.tokens[0].Lemma))
	tmp2 := []rune(wordDict.Get(ng.tokens[1].Lemma))
	tmp3 := []rune(wordDict.Get(ng.tokens[1].Lemma))
	ng.IsProperName = unicode.IsUpper(tmp1[0]) || unicode.IsUpper(tmp2[0]) || unicode.IsUpper(tmp3[0])
}

func (ng *Trigram) TestAndAdoptNominative(other Ngram) {
	if other.Len() != 3 {
		panic("cannot TestAndAdoptNominative for different n-gram types")
	}
	if !ng.tokens[0].IsNominative() && other.TokenAt(0).IsNominative() {
		ng.tokens[0].Word = other.TokenAt(0).Word
		ng.tokens[0].Case = other.TokenAt(0).Case
		ng.tokens[1].Word = other.TokenAt(1).Word
		ng.tokens[1].Case = other.TokenAt(1).Case
		ng.tokens[2].Word = other.TokenAt(2).Word
		ng.tokens[2].Case = other.TokenAt(2).Case
	}
}

func (ng *Trigram) Preview(wordDict *ptcount.WordDict) string {
	return fmt.Sprintf(
		"[T]> w: %s %s %s, lm: %s %s %s, freq: %d, ARF: %.2f, ES: %.2f, CS: %d\n",
		wordDict.Get(ng.tokens[0].Word),
		wordDict.Get(ng.tokens[1].Word),
		wordDict.Get(ng.tokens[2].Word),
		wordDict.Get(ng.tokens[0].Lemma),
		wordDict.Get(ng.tokens[1].Lemma),
		wordDict.Get(ng.tokens[2].Lemma),
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
		ng.corpusSize,
	)
}

func (ng *Trigram) String() string {
	return fmt.Sprintf(
		"[U]> w: %d %d %d, lm: %d %d %d, freq: %d, ARF: %.2f, ES: %.2f\n",
		ng.tokens[0].Word,
		ng.tokens[1].Word,
		ng.tokens[2].Word,
		ng.tokens[0].Lemma,
		ng.tokens[1].Lemma,
		ng.tokens[2].Lemma,
		ng.Count,
		ng.ARF(),
		ng.EffectSize,
	)
}

func (ng *Trigram) ARF() float64 {
	if ng == nil {
		return 0
	}
	return ng.ngramBase.ARF()
}
