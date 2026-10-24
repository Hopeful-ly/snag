package platform

import "testing"

func TestDrivesAreTheirLetters(t *testing.T) {
	if SameDrive(`C:\Users\me\AppData\Local\Snag\work`, `D:\Films`) {
		t.Error("C: and D: are one drive")
	}
	if !SameDrive(`C:\Users\me\AppData`, `c:\Downloads`) {
		t.Error("C: and c: are two drives")
	}
}
