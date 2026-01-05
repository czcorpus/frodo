package keywords

import "github.com/tomachalek/vertigo/v6"

type TokenCounter struct {
	NumTokens int
}

func (tc *TokenCounter) ProcToken(token *vertigo.Token, line int, err error) error {
	tc.NumTokens++
	return nil
}

func (tc *TokenCounter) ProcStruct(strc *vertigo.Structure, line int, err error) error {
	return nil
}

func (tc *TokenCounter) ProcStructClose(strc *vertigo.StructureClose, line int, err error) error {
	return nil
}
