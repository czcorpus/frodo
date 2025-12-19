package keywords

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/czcorpus/vert-tagextract/v3/proc"
	"github.com/czcorpus/vert-tagextract/v3/ptcount"
	"github.com/rs/zerolog/log"
	"github.com/tomachalek/vertigo/v6"
)

var ErrorTooManyParsingErrors = errors.New("too many parsing errors")

// -----------------

type stopWordFilter struct {
	tagColIdx int
}

func (f *stopWordFilter) Apply(tk *vertigo.Token, attrAcc proc.AttrAccumulator) bool {
	w := []rune(tk.Word)
	return !(len(w) == 1 && unicode.IsPunct(w[0])) && (strings.HasPrefix(tk.PosAttrByIndex(f.tagColIdx), "N") ||
		strings.HasPrefix(tk.PosAttrByIndex(f.tagColIdx), "A"))
}

// -----------------

type token struct {
	Word  int
	Lemma int
	Case  int
}

func (tk *token) IsZero() bool {
	return tk.Word == 0 && tk.Lemma == 0
}

func (tk *token) IsNominative() bool {
	return tk.Case == 1
}

// ----------------

type ngram struct {
	tokens       [2]token
	Count        int
	EffectSize   float64
	IsProperName bool
	LastPos      int
	Distances    []int
	arf          float64
}

func (ng *ngram) UniqueID() string {
	return fmt.Sprintf("%d--%d", ng.tokens[0].Lemma, ng.tokens[1].Lemma)
}

func (ng *ngram) IncCount() {
	ng.Count++
}

func (ng *ngram) SafeCount() int {
	if ng == nil {
		return 0
	}
	return ng.Count
}

func (ng *ngram) SafeEffectSize() float64 {
	if ng == nil {
		return 0
	}
	return ng.EffectSize
}

func (ng *ngram) AppendToken(tk token) {
	for i := range len(ng.tokens) {
		if ng.tokens[i].IsZero() {
			ng.tokens[i] = tk
			break
		}
	}
}

func (ng *ngram) SetDistance(currToken int) {
	ng.Distances = append(ng.Distances, currToken-ng.LastPos)
	ng.LastPos = currToken
}

func (ng *ngram) CalcArf(corpSize int) float64 {
	avgDist := float64(corpSize) / float64(ng.Count)
	tmp := 0.0
	for _, v := range ng.Distances {
		tmp += min(float64(v), avgDist)
	}
	ng.arf = tmp / avgDist
	return ng.arf
}

func (ng *ngram) ARF() float64 {
	if ng == nil {
		return 0
	}
	return ng.arf
}

func newNgram() *ngram {
	return &ngram{
		Distances: make([]int, 0, 10),
	}
}

// -----------------

type NgramExtractor struct {
	ctx          context.Context
	attrAccum    proc.AttrAccumulator
	currSentence []token
	valueDict    *ptcount.WordDict
	lineCounter  int
	errorCounter int
	tokenCounter int
	maxNumErrors int
	filter       proc.LineFilter
	conf         KeywordsBuildArgs
	statusChan   chan<- keywordsBuildStatus
	colCounts2   map[string]*ngram
	colCounts1   map[string]*ngram
}

func (tte *NgramExtractor) handleProcError(lineNum int, err error) error {
	tte.statusChan <- keywordsBuildStatus{
		Datetime:     time.Now(),
		NumProcLines: lineNum,
		Error:        err,
	}
	log.Error().Err(err).Int("lineNumber", lineNum).Msg("parsing error")
	tte.errorCounter++
	if tte.errorCounter > tte.maxNumErrors {
		return ErrorTooManyParsingErrors
	}
	return nil
}

func (tte *NgramExtractor) procPrevSent() {

}

func (tte *NgramExtractor) ProcStruct(st *vertigo.Structure, line int, err error) error {
	select {
	case s := <-tte.ctx.Done():
		return fmt.Errorf("received stop signal: %s", s)
	default:
	}
	if err != nil { // error from the Vertigo parser
		return tte.handleProcError(line, err)
	}
	tte.lineCounter = line
	if st.Name == tte.conf.SentenceStruct {
		tte.currSentence = tte.currSentence[:0]
	}
	return nil
}

func (tte *NgramExtractor) ProcStructClose(st *vertigo.StructureClose, line int, err error) error {
	select {
	case s := <-tte.ctx.Done():
		return fmt.Errorf("received stop signal: %s", s)
	default:
	}
	if err != nil { // error from the Vertigo parser
		return tte.handleProcError(line, err)
	}
	return nil
}

// ProcToken is a part of vertigo.LineProcessor implementation.
// It is called by Vertigo parser when a token line is encountered.
func (tte *NgramExtractor) ProcToken(tk *vertigo.Token, line int, err error) error {
	if err != nil {
		return tte.handleProcError(line, err)
	}
	tte.lineCounter = line
	if tte.filter.Apply(tk, tte.attrAccum) {
		tte.tokenCounter = tk.Idx
		rawCase := tk.PosAttrByIndex(tte.conf.TagColIdx)[4:5]
		cse := -1
		if rawCase != "-" {
			cse, err = strconv.Atoi(rawCase)
			if err != nil {
				log.Error().Err(err).Str("tag", tk.PosAttrByIndex(tte.conf.TagColIdx)).Msg("failed to decode token's case")
				cse = -1
			}
		}
		token2 := token{
			Word:  tte.valueDict.Add(tk.PosAttrByIndex(tte.conf.WordColIdx)),
			Lemma: tte.valueDict.Add(tk.PosAttrByIndex(tte.conf.LemmaColIdx)),
			Case:  cse,
		}

		tte.currSentence = append(tte.currSentence, token2)
		if len(tte.currSentence) >= tte.conf.NgramSize {
			ngram2 := newNgram()
			startPos := len(tte.currSentence) - tte.conf.NgramSize
			for i := startPos; i < len(tte.currSentence); i++ {
				ngram2.AppendToken(tte.currSentence[i])
			}
			key := ngram2.UniqueID()
			_, ok := tte.colCounts2[key]
			if !ok {
				tmp1 := []rune(tte.valueDict.Get(ngram2.tokens[0].Lemma))
				tmp2 := []rune(tte.valueDict.Get(ngram2.tokens[1].Lemma))
				ngram2.IsProperName = unicode.IsUpper(tmp1[0]) || unicode.IsUpper(tmp2[0])
				tte.colCounts2[key] = ngram2

			} else if !tte.colCounts2[key].tokens[0].IsNominative() && ngram2.tokens[0].IsNominative() {
				tte.colCounts2[key].tokens[0].Word = ngram2.tokens[0].Word
				tte.colCounts2[key].tokens[0].Case = ngram2.tokens[0].Case
				tte.colCounts2[key].tokens[1].Word = ngram2.tokens[1].Word
				tte.colCounts2[key].tokens[1].Case = ngram2.tokens[1].Case
			}
			tte.colCounts2[key].IncCount()
			tte.colCounts2[key].SetDistance(tk.Idx)
		}
		// unigrams
		//
		token1 := token{
			Word:  tte.valueDict.Add(tk.PosAttrByIndex(tte.conf.WordColIdx)),
			Lemma: tte.valueDict.Add(tk.PosAttrByIndex(tte.conf.LemmaColIdx)),
			Case:  cse,
		}
		ngram1 := newNgram()
		ngram1.AppendToken(token1)
		key := ngram1.UniqueID()
		_, ok := tte.colCounts1[key]
		if !ok {
			tmp1 := []rune(tte.valueDict.Get(ngram1.tokens[0].Lemma))
			ngram1.IsProperName = unicode.IsUpper(tmp1[0])
			tte.colCounts1[key] = ngram1

		} else if !tte.colCounts1[key].tokens[0].IsNominative() && ngram1.tokens[0].IsNominative() {
			tte.colCounts1[key].tokens[0].Word = ngram1.tokens[0].Word
			tte.colCounts1[key].tokens[0].Case = ngram1.tokens[0].Case
		}
		tte.colCounts1[key].IncCount()
		tte.colCounts1[key].SetDistance(tk.Idx)
	}

	if line%100000 == 0 {
		tte.statusChan <- keywordsBuildStatus{
			Datetime:     time.Now(),
			NumProcLines: line,
		}
	}
	return nil
}

func (tte *NgramExtractor) TotalTokens() int {
	return tte.tokenCounter
}

func (tte *NgramExtractor) CalcARF() {
	for _, v := range tte.colCounts1 {
		v.CalcArf(tte.tokenCounter)
	}
	for _, v := range tte.colCounts2 {
		v.CalcArf(tte.tokenCounter)
	}
}

func (tte *NgramExtractor) Preview() {
	i := 0
	for _, v := range tte.colCounts2 {
		if i >= 10 {
			break
		}
		fmt.Printf(
			"w: %s %s, lm: %s %s, freq: %d, ARF: %.2f\n",
			tte.valueDict.Get(v.tokens[0].Word),
			tte.valueDict.Get(v.tokens[1].Word),
			tte.valueDict.Get(v.tokens[0].Lemma),
			tte.valueDict.Get(v.tokens[1].Lemma),
			v.Count,
			v.ARF(),
		)
		i++
	}

}

func NewNgramExtractor(
	ctx context.Context,
	args KeywordsBuildArgs,
	wordDict *ptcount.WordDict,
	statusChan chan<- keywordsBuildStatus,
) *NgramExtractor {
	return &NgramExtractor{
		ctx:  ctx,
		conf: args,
		filter: &stopWordFilter{
			tagColIdx: args.TagColIdx,
		},
		colCounts2: make(map[string]*ngram),
		colCounts1: make(map[string]*ngram),
		valueDict:  wordDict,
		statusChan: statusChan,
	}
}
