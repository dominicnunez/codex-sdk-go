//go:build windows && 386

package process

// Native ABI constants, independently checked against the maintained Go
// Windows definitions and the Windows SDK structures.
const (
	jobBasicBytes              = 48
	jobExtendedBytes           = 112
	jobMinimumWorkingSetOffset = 20
	jobMaximumWorkingSetOffset = 24
	jobActiveProcessOffset     = 28
	jobAffinityOffset          = 32
	jobPriorityOffset          = 36
	jobSchedulingOffset        = 40
	jobIOOffset                = 48
	jobProcessMemoryOffset     = 96
	jobMemoryOffset            = 100
	jobPeakProcessMemoryOffset = 104
	jobPeakMemoryOffset        = 108
)
