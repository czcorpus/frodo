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
	Word    int
	Lemma   int
	Case    int
	AuxWord int
}

func (tk *token) IsZero() bool {
	return tk.Word == 0 && tk.Lemma == 0
}

func (tk *token) IsNominative() bool {
	return tk.Case == 1
}

// ----------------

type auxWord struct {
	Value    int
	Position int
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
	colCounts3   map[string]Ngram
	colCounts2   map[string]Ngram
	colCounts1   map[string]Ngram
	corpusSize   int
	currAux      auxWord
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

func (tte *NgramExtractor) newNgram(size int) Ngram {
	switch size {
	case 1:
		return &Unigram{ngramBase: ngramBase{corpusSize: tte.corpusSize}}
	case 2:
		return &Bigram{ngramBase: ngramBase{corpusSize: tte.corpusSize}}
	case 3:
		return &Trigram{ngramBase: ngramBase{corpusSize: tte.corpusSize}}
	default:
		return nil
	}
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
	if strings.HasPrefix(tk.PosAttrByIndex(tte.conf.TagColIdx), "J") ||
		strings.HasPrefix(tk.PosAttrByIndex(tte.conf.TagColIdx), "R") {

		val := tte.valueDict.Add(tk.PosAttrByIndex(tte.conf.WordColIdx))
		tte.currAux = auxWord{Value: val, Position: line}

	} else if tte.filter.Apply(tk, tte.attrAccum) {
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
		if tte.currAux.Position+1 == line {
			token2.AuxWord = tte.currAux.Value
		}
		tte.currAux = auxWord{}
		tte.currSentence = append(tte.currSentence, token2)

		if len(tte.currSentence) >= 3 {
			ngram3 := tte.newNgram(3)
			startPos := len(tte.currSentence) - 3
			for i := startPos; i < len(tte.currSentence); i++ {
				ngram3.AppendToken(tte.currSentence[i])
			}
			key := ngram3.UniqueID()
			ngram3Curr, ok := tte.colCounts3[key]
			if ok {
				ngram3Curr.TestAndAdoptNominative(ngram3)
				ngram3 = ngram3Curr

			} else {
				ngram3.TestAndSetPropnameFlag(tte.valueDict)
			}
			ngram3.IncCount()
			ngram3.SetDistance(tk.Idx)
			tte.colCounts3[key] = ngram3
		}

		if len(tte.currSentence) >= 2 {
			ngram2 := tte.newNgram(2)
			startPos := len(tte.currSentence) - 2
			for i := startPos; i < len(tte.currSentence); i++ {
				ngram2.AppendToken(tte.currSentence[i])
			}
			key := ngram2.UniqueID()
			ngram2Curr, ok := tte.colCounts2[key]
			if ok {
				ngram2Curr.TestAndAdoptNominative(ngram2)
				ngram2 = ngram2Curr

			} else {
				ngram2.TestAndSetPropnameFlag(tte.valueDict)
			}
			ngram2.IncCount()
			ngram2.SetDistance(tk.Idx)
			tte.colCounts2[key] = ngram2
		}
		// unigrams
		//
		ngram1 := tte.newNgram(1)
		ngram1.AppendToken(token2)
		key := ngram1.UniqueID()
		ngram1Curr, ok := tte.colCounts1[key]
		if ok {
			ngram1Curr.TestAndAdoptNominative(ngram1)
			ngram1 = ngram1Curr

		} else {
			ngram1.TestAndSetPropnameFlag(tte.valueDict)
		}
		ngram1.IncCount()
		ngram1.SetDistance(tk.Idx)
		tte.colCounts1[key] = ngram1
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

func (tte *NgramExtractor) Preview() {
	i := 0
	for _, v := range tte.colCounts2 {
		if i >= 10 {
			break
		}
		fmt.Println(v.Preview(tte.valueDict))
		i++
	}

}

func NewNgramExtractor(
	ctx context.Context,
	args KeywordsBuildArgs,
	wordDict *ptcount.WordDict,
	corpusSize int,
	statusChan chan<- keywordsBuildStatus,
) *NgramExtractor {
	return &NgramExtractor{
		ctx:  ctx,
		conf: args,
		filter: &stopWordFilter{
			tagColIdx: args.TagColIdx,
		},
		colCounts3: make(map[string]Ngram),
		colCounts2: make(map[string]Ngram),
		colCounts1: make(map[string]Ngram),
		valueDict:  wordDict,
		statusChan: statusChan,
		corpusSize: corpusSize,
	}
}
