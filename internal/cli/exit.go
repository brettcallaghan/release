package cli

// Status is the process exit status. The four values are the release CLI
// contract's guarantees (specs/contracts/release-cli-contract.md), and the
// contract versions them: changing what a value means is a major version.
type Status int

const (
	// StatusSuccess: the command succeeded.
	StatusSuccess Status = 0
	// StatusRefused: the command refused a stated, expected condition, such
	// as nothing to release or a policy violation. An outcome, not a fault.
	StatusRefused Status = 1
	// StatusInvalid: invalid arguments or an unacceptable configuration.
	StatusInvalid Status = 2
	// StatusFailedAfterChange: the command failed after changing the
	// repository, and reported every change it made.
	StatusFailedAfterChange Status = 3
)
