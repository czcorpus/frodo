// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
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

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// setupColors sets the color profile used for rendering.
// In "auto" mode, the profile is detected from the environment, but
// we work around the fact that terminals inside tmux/screen often
// report TERM=screen, which is detected as monochrome even though
// tmux supports (at least) 256 colors.
func setupColors(mode string) error {
	switch mode {
	case "auto":
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			lipgloss.SetColorProfile(termenv.Ascii)
			return nil
		}
		term := os.Getenv("TERM")
		insideMux := os.Getenv("TMUX") != "" || os.Getenv("STY") != "" ||
			strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux")
		// (in termenv, a higher profile value means fewer colors)
		if insideMux && lipgloss.ColorProfile() > termenv.ANSI256 {
			lipgloss.SetColorProfile(termenv.ANSI256)
		}
	case "none":
		lipgloss.SetColorProfile(termenv.Ascii)
	case "16":
		lipgloss.SetColorProfile(termenv.ANSI)
	case "256":
		lipgloss.SetColorProfile(termenv.ANSI256)
	case "truecolor":
		lipgloss.SetColorProfile(termenv.TrueColor)
	default:
		return fmt.Errorf("invalid color mode '%s' (allowed: auto, none, 16, 256, truecolor)", mode)
	}
	return nil
}

// keyLogger records raw input events for diagnosing terminal
// issues (e.g. keys not recognized inside tmux/ssh).
var keyLogger *log.Logger

func setupKeyLog(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open key log: %w", err)
	}
	keyLogger = log.New(f, "", log.Ltime|log.Lmicroseconds)
	keyLogger.Printf(
		"--- start: TERM=%q TMUX=%q COLORTERM=%q colorProfile=%d",
		os.Getenv("TERM"), os.Getenv("TMUX"), os.Getenv("COLORTERM"), lipgloss.ColorProfile())
	return func() { f.Close() }, nil
}

func logInput(msg tea.Msg) {
	if keyLogger == nil {
		return
	}
	switch msg := msg.(type) {
	case tickMsg, listMsg, detailMsg:
		// app's own messages, not interesting
	case tea.KeyMsg:
		keyLogger.Printf("key: %q type=%d runes=%q alt=%v", msg.String(), msg.Type, msg.Runes, msg.Alt)
	default:
		// this includes Bubble Tea's internal messages for unrecognized
		// input sequences
		keyLogger.Printf("msg: %T %+v", msg, msg)
	}
}
