package keywords

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/czcorpus/vert-tagextract/v3/ptcount"
)

type Keyword struct {
	NgramSize  int     `json:"ngramSize"`
	Lemma      string  `json:"value"`
	EffectSize float64 `json:"score"`
}

// logLikelihood calculates log-likelihood (G²) for a 2x2 contingency table.
//
// a: frequency of term in corpus 1 (today)
// b: frequency of term in corpus 2 (reference)
// c: total tokens in corpus 1 - a
// d: total tokens in corpus 2 - b
// Returns: G² value (higher = more significant difference)
func logLikelihood(a, b, c, d float64) float64 {

	// Expected frequencies
	E1 := (a + b) * (a + c) / (a + b + c + d)
	E2 := (a + b) * (b + d) / (a + b + c + d)

	// Avoid log(0)
	var g2_a, g2_b float64
	if a == 0 {
		g2_a = 0

	} else {
		g2_a = a * math.Log(a/E1)
	}

	if b == 0 {
		g2_b = 0.0

	} else {
		g2_b = b * math.Log(b/E2)
	}
	return 2 * (g2_a + g2_b)
}

// effectSizeLogRatio calculates effect size as log ratio (also called %DIFF).
//
// This tells you the magnitude of difference, not just statistical significance.
// Positive = overrepresented in corpus 1, negative = underrepresented.
func effectSizeLogRatio(a, b, c, d float64) float64 {
	// Normalized frequencies (per million tokens)
	var freq1, freq2 float64
	if a+c > 0 {
		freq1 = a / (a + c) * 1_000_000
	}
	if b+d > 0 {
		freq2 = b / (b + d) * 1000000.0
	}

	// Add smoothing to avoid log(0)
	freq1 += 0.5
	freq2 += 0.5

	return math.Log2(freq1 / freq2)
}

func wordVecToString(keys []int, wordDict *ptcount.WordDict) string {
	var ans strings.Builder
	for i, v := range keys {
		if i > 0 {
			ans.WriteByte(' ')
		}
		s := wordDict.Get(v)
		ans.WriteString(s)
	}
	return ans.String()
}

func FindKeywords(
	wordsRef map[string]Ngram,
	wordsFocus map[string]Ngram,
	wordDict *ptcount.WordDict,
	minARF float64,
) []Keyword {
	results := make([]Ngram, 0, 1000)
	minLL := 10.83 // ~ p < 0.001
	var c, d float64
	for _, ng := range wordsFocus {
		c += ng.ARF()
	}
	for _, ng := range wordsRef {
		d += ng.ARF()
	}
	fmt.Printf("focus size : %d, ref size: %d\n", len(wordsFocus), len(wordsRef))
	for _, ngram := range wordsFocus {
		if ngram.ARF() < minARF {
			continue
		}
		if _, ok := wordsRef[ngram.UniqueID()]; !ok {
			// word not found in reference corpus
			continue
		}
		/*
					a = today_counts[lemma]  # freq in today
			        b = reference_counts.get(lemma, 0)  # freq in reference
			        c = today_total - a  # other tokens today
			        d = reference_total - b  # othe
		*/
		a := ngram.ARF()
		b := wordsRef[ngram.UniqueID()].ARF()
		c -= a
		d -= b
		//fmt.Printf("a = %d, b = %d, c = %d, d = %d\n", a, b, c, d)
		ll := logLikelihood(a, b, c, d)
		if ngram.IsPropname() {
			ll *= 1.3
		}
		if ll >= minLL {
			ngram.SetEffectSize(effectSizeLogRatio(a, b, c, d))
			// Only keep words overrepresented today (positive effect size)
			if ngram.SafeEffectSize() > 0 {
				results = append(results, ngram)
			}
		}
	}
	slices.SortFunc(results, func(n1, n2 Ngram) int {
		if n1.SafeEffectSize() < n2.SafeEffectSize() {
			return 1
		}
		if n1.SafeEffectSize() > n2.SafeEffectSize() {
			return -1
		}
		return 0
	})

	ans := make([]Keyword, 15)
	for i, v := range results[:15] {
		switch v.Len() {
		case 3:
			ans[i] = Keyword{
				Lemma:      wordVecToString(v.WordAsVector(), wordDict),
				EffectSize: v.SafeEffectSize(),
			}
		case 2:
			ans[i] = Keyword{
				Lemma:      wordVecToString(v.WordAsVector(), wordDict),
				EffectSize: v.SafeEffectSize(),
			}
		case 1:
			ans[i] = Keyword{
				Lemma:      wordVecToString(v.WordAsVector(), wordDict),
				EffectSize: v.SafeEffectSize(),
			}
		}
	}
	return ans
}
