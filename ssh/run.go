package ssh

import (
	"errors"
	"strings"
)

// RunStep is one command the Android app types into the login shell after it
// connects, stored as a repeated "run=" token in the host's #sshc comment:
//
//	Host box #sshc run=cd%20/srv/app run=-htop
//
// The value is percent-escaped like start= (escapeToken); a leading '-' marks
// a step that is switched off and kept for later (a command that itself
// starts with '-' has it escaped as %2D). Steps run in the order written.
//
// Desktop sshc has no shell to type into (ssh runs in your terminal), so it
// only keeps and shows them. The app runs them only for hosts whose startup
// is the plain login shell.
type RunStep struct {
	Cmd string
	Off bool
}

// valid reports whether s can be stored and typed: one non-empty line.
func (s RunStep) valid() bool {
	return strings.TrimSpace(s.Cmd) != "" && !strings.ContainsAny(s.Cmd, "\r\n")
}

// token renders s as a "run=" metadata token.
func (s RunStep) token() string {
	val := escapeToken(strings.TrimSpace(s.Cmd))
	if strings.HasPrefix(val, "-") {
		val = "%2D" + val[1:]
	}
	if s.Off {
		val = "-" + val
	}
	return "run=" + val
}

// parseRunStep reads a run= token value; ok is false for one that isn't
// valid, which the caller keeps verbatim instead.
func parseRunStep(val string) (RunStep, bool) {
	var s RunStep
	if strings.HasPrefix(val, "-") {
		s.Off = true
		val = val[1:]
	}
	s.Cmd = strings.TrimSpace(unescapeToken(val))
	return s, s.valid()
}

// ValidateRunStep checks a command before it's saved: one non-empty line.
func ValidateRunStep(cmd string) error {
	if !(RunStep{Cmd: cmd}).valid() {
		if strings.TrimSpace(cmd) == "" {
			return errEmptyRun
		}
		return errMultilineRun
	}
	return nil
}

var (
	errEmptyRun     = errors.New("after-login command is empty")
	errMultilineRun = errors.New("after-login command must be one line")
)
