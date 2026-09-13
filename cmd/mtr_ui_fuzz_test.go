package cmd

import "testing"

const fuzzMTRInputMaxBytes = 4096

func FuzzMTRInputParser(f *testing.F) {
	f.Add([]byte("p rynedgq"))
	f.Add([]byte("oOjJsS?"))
	f.Add([]byte("\x1b[A\x1b[B\x1b[C\x1b[D\x1bOA\x1bOB\x1bOC\x1bOD"))
	f.Add([]byte{0x1b, '[', '2', '0', '0', '~', 0x1b, ']', '0', ';', 'x', 0x07})
	f.Add([]byte{0x1b, '[', 'M', 0x20, 0x30, 0x30, 0x1b, 'O', 'P'})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > fuzzMTRInputMaxBytes {
			data = data[:fuzzMTRInputMaxBytes]
		}

		for _, tracking := range []bool{false, true} {
			parser := mtrInputParser{trackPaste: tracking, trackArrows: tracking}
			for _, b := range data {
				action := parser.Feed(b)
				if action < mtrActionNone || action > mtrActionRight {
					t.Fatalf("action = %d, outside known range", action)
				}
			}
			if parser.state < mtrStateGround || parser.state > mtrStateSGRMouse {
				t.Fatalf("state = %d, outside known range", parser.state)
			}
			if parser.csiN < 0 || parser.csiN > mtrParserMaxCSI+1 {
				t.Fatalf("CSI byte count = %d, want 0..%d", parser.csiN, mtrParserMaxCSI+1)
			}
		}
	})
}
