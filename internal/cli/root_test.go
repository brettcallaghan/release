package cli

import (
	"bytes"
	"strings"
	"testing"
)

func run(args ...string) (stdout, stderr string, code int) {
	var out, errb bytes.Buffer
	code = Execute(args, &out, &errb)
	return out.String(), errb.String(), code
}

// The contract's exit vocabulary is versioned, so its values are asserted
// here rather than only implied by the snapshot cases.
func TestStatusValuesMatchTheContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  Status
		want int
	}{
		{"success", StatusSuccess, 0},
		{"refused", StatusRefused, 1},
		{"invalid", StatusInvalid, 2},
		{"failed after change", StatusFailedAfterChange, 3},
	} {
		if int(tc.got) != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestBareInvocationPrintsUsageToStderrAndExitsInvalid(t *testing.T) {
	stdout, stderr, code := run()
	if code != int(StatusInvalid) {
		t.Errorf("exit = %d, want %d", code, StatusInvalid)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr lacks usage:\n%s", stderr)
	}
}

func TestUnknownFlagExitsInvalid(t *testing.T) {
	_, stderr, code := run("--no-such-flag")
	if code != int(StatusInvalid) {
		t.Errorf("exit = %d, want %d", code, StatusInvalid)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Errorf("stderr does not name the problem:\n%s", stderr)
	}
}

func TestUnexpectedArgumentExitsInvalid(t *testing.T) {
	if _, _, code := run("no-such-command"); code != int(StatusInvalid) {
		t.Errorf("exit = %d, want %d", code, StatusInvalid)
	}
}

func TestVersionFlagsPrintToStdoutAndSucceed(t *testing.T) {
	for _, flag := range []string{"--version", "-V"} {
		stdout, stderr, code := run(flag)
		if code != int(StatusSuccess) {
			t.Errorf("%s: exit = %d, want 0", flag, code)
		}
		if !strings.HasPrefix(stdout, "release ") {
			t.Errorf("%s: stdout = %q, want it to start with %q", flag, stdout, "release ")
		}
		if stderr != "" {
			t.Errorf("%s: stderr must stay empty, got %q", flag, stderr)
		}
	}
}

// -v belongs to --verbose in the contract, so cobra's default -v for
// --version must not be present.
func TestVersionShorthandIsCapitalV(t *testing.T) {
	root := newRootCmd(&streams{})
	if f := root.Flags().ShorthandLookup("v"); f != nil {
		t.Errorf("-v is bound to --%s; the contract reserves it for --verbose", f.Name)
	}
}
