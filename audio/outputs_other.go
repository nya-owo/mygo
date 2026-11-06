//go:build !(windows && (amd64 || arm64))

package audio

// listOutputs has none to list: sound plays through the system's default.
func listOutputs() ([]Output, error) { return nil, errUnsupported }
