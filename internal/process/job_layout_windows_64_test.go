//go:build windows && (amd64 || arm64)

package process

// Native ABI constants, independently checked against the maintained Go
// Windows definitions and the Windows SDK structures.
const (
	jobBasicBytes              = 64
	jobExtendedBytes           = 144
	jobMinimumWorkingSetOffset = 24
	jobMaximumWorkingSetOffset = 32
	jobActiveProcessOffset     = 40
	jobAffinityOffset          = 48
	jobPriorityOffset          = 56
	jobSchedulingOffset        = 60
	jobIOOffset                = 64
	jobProcessMemoryOffset     = 112
	jobMemoryOffset            = 120
	jobPeakProcessMemoryOffset = 128
	jobPeakMemoryOffset        = 136
)
