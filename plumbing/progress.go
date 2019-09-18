// This came from https://raw.githubusercontent.com/BTBurke/clt/7f5151b6bef1b0da4b03f965b47cbe2de87a3ac0/progress.go
package plumbing

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	success int = iota
	fail
)

const (
	spinner int = iota
	bar
	loading
)

// Spinner is a set of unicode strings that show a moving progress indication in the terminal
type Spinner []string

var (
	// Wheel created with pipes and slashes
	Wheel Spinner = []string{"|", "/", "-", "\\"}
	// Bouncing dots
	Bouncing Spinner = []string{"⠁", "⠂", "⠄", "⠂"}
	// Clock that spins two hours per step
	Clock Spinner = []string{"🕐 ", "🕑 ", "🕒 ", "🕓 ", "🕔 ", "🕕 ", "🕖 ", "🕗 ", "🕘 ", "🕙 ", "🕚 "}
	// Dots that spin around a rectangle
	Dots Spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

// Progress structure used to render progress and loading indicators
type Progress struct {
	// Prompt to display before spinner or bar
	Prompt string
	// Approximate length of the total progress display, including
	// the prompt and the ..., does not include status indicator
	// at the end (e.g, the spinner, FAIL, OK, or XX%)
	DisplayLength int

	style     int
	cf        chan float64
	c         chan int
	spinsteps Spinner
	delay     time.Duration
	output    io.Writer
	wg        sync.WaitGroup
}

// NewProgressSpinner returns a new spinner with prompt <message>
// display length defaults to 30.
func NewProgressSpinner(format string, args ...interface{}) *Progress {
	return &Progress{
		style:         spinner,
		Prompt:        fmt.Sprintf(format, args...),
		DisplayLength: 30,
		output:        os.Stdout,
		spinsteps:     Wheel,
	}
}

// NewProgressBar returns a new progress bar with prompt <message>
// display length defaults to 20
func NewProgressBar(format string, args ...interface{}) *Progress {
	return &Progress{
		style:         bar,
		Prompt:        fmt.Sprintf(format, args...),
		DisplayLength: 20,
		output:        os.Stdout,
	}
}

// NewLoadingMessage creates a spinning loading indicator followed by a message.
// The loading indicator does not indicate sucess or failure and disappears when
// you call either Success() or Failure().  This is useful to show action when
// making remote calls that are expected to be short.  The delay parameter is to
// prevent flickering when the remote call finishes quickly.  If you finish your call
// and call Success() or Failure() within the delay period, the loading indicator
// will never be shown.
func NewLoadingMessage(message string, spinner Spinner, delay time.Duration) *Progress {
	return &Progress{
		style:         loading,
		Prompt:        message,
		DisplayLength: 0,
		spinsteps:     spinner,
		output:        os.Stdout,
		delay:         delay,
	}
}

// Start launches a Goroutine to render the progress bar or spinner
// and returns control to the caller for further processing.  Spinner
// will update automatically every 250ms until Success() or Fail() is
// called.  Bars will update by calling Update(<pct_complete>).  You
// must always finally call either Success() or Fail() to terminate
// the go routine.
func (p *Progress) Start() {
	p.wg.Add(1)
	switch p.style {
	case spinner:
		p.c = make(chan int)
		go renderSpinner(p, p.c)
	case bar:
		p.cf = make(chan float64, 2)
		go renderBar(p, p.cf)
		p.cf <- 0.0
	case loading:
		p.c = make(chan int)
		go renderLoading(p, p.c)
	}
}

// Success should be called on a progress bar or spinner
// after completion is successful
func (p *Progress) Success() {
	switch p.style {
	case spinner:
		p.c <- success
		close(p.c)
	case bar:
		p.cf <- -1.0
		close(p.cf)
	case loading:
		p.c <- success
		close(p.c)
	}
	p.wg.Wait()
}

// Fail should be called on a progress bar or spinner
// if a failure occurs
func (p *Progress) Fail() {
	switch p.style {
	case spinner:
		p.c <- fail
		close(p.c)
	case bar:
		p.cf <- -2.0
		close(p.cf)
	// loading only has one termination state
	case loading:
		p.c <- success
		close(p.c)
	}
	p.wg.Wait()
}

func renderSpinner(p *Progress, c chan int) {
	defer p.wg.Done()
	if p.output == nil {
		p.output = os.Stdout
	}
	promptLen := len(p.Prompt)
	dotLen := p.DisplayLength - promptLen
	if dotLen < 3 {
		dotLen = 3
	}
	for i := 0; ; i++ {
		select {
		case result := <-c:
			switch result {
			case success:
				fmt.Fprintf(p.output, "\x1b[?25h\r%s%s[%s]\n", p.Prompt, strings.Repeat(".", dotLen), "OK")
			case fail:
				fmt.Fprintf(p.output, "\x1b[?25h\r%s%s[%s]\n", p.Prompt, strings.Repeat(".", dotLen), "FAIL")
			}
			return
		default:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s%s[%s]", p.Prompt, strings.Repeat(".", dotLen), spinLookup(i, p.spinsteps))
			time.Sleep(time.Duration(250) * time.Millisecond)
		}
	}
}

func renderLoading(p *Progress, c chan int) {
	defer p.wg.Done()
	if p.output == nil {
		p.output = os.Stdout
	}

	// delay to prevent flickering
	// calling Success or Failure within delay will shortcircuit the loading indicator
	if p.delay > 0 {
		t := time.NewTicker(p.delay)
		select {
		case <-c:
			return
		case <-t.C:
			t.Stop()
		}
	}

	for i := 0; ; i++ {
		select {
		case <-c:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s\r\n", strings.Repeat(" ", len(p.spinsteps[0])+len(p.Prompt)+3))
			return
		default:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s  %s", spinLookup(i, p.spinsteps), p.Prompt)
			time.Sleep(time.Duration(250) * time.Millisecond)
		}
	}
}

func spinLookup(i int, steps []string) string {
	return steps[i%len(steps)]
}

func renderBar(p *Progress, c chan float64) {
	defer p.wg.Done()
	if p.output == nil {
		p.output = os.Stdout
	}

	for result := range c {
		eqLen := int(result * float64(p.DisplayLength))
		spLen := p.DisplayLength - eqLen
		switch {
		case result == -1.0:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s: [%s] %s", p.Prompt, strings.Repeat("=", p.DisplayLength), "100%")
			fmt.Fprintf(p.output, "\x1b[?25h\n")
			return
		case result == -2.0:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s: [%s] %s", p.Prompt, strings.Repeat("X", p.DisplayLength), "FAIL")
			fmt.Fprintf(p.output, "\x1b[?25h\n")
			return
		case result >= 0.0:
			fmt.Fprintf(p.output, "\x1b[?25l\r%s: [%s%s] %2.0f%%", p.Prompt, strings.Repeat("=", eqLen), strings.Repeat(" ", spLen), 100.0*result)
		}

	}
}

// Update the progress bar using a number [0, 1.0] to represent
// the percentage complete
func (p *Progress) Update(pct float64) {
	if pct >= 1.0 {
		pct = 1.0
	}
	p.cf <- pct
}
