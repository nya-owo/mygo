package audio

import (
	"os"
	"testing"
)

// The outputs the system has, listed by name, one of them the default:
// run with MYGO_AUDIO_LIVE set on a machine with sound.
func TestOutputsLive(t *testing.T) {
	if os.Getenv("MYGO_AUDIO_LIVE") == "" {
		t.Skip("MYGO_AUDIO_LIVE unset")
	}
	outs, err := Outputs()
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, o := range outs {
		t.Logf("%q default=%v id=%s", o.Name, o.Default, o.ID)
		if o.Default {
			defaults++
		}
	}
	if len(outs) > 0 && defaults != 1 {
		t.Errorf("%d defaults among %d outputs", defaults, len(outs))
	}
}
