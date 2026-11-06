package phonenumber

import "testing"

func checkInput(t *testing.T, f *AsYouTypeFormatter, r rune, want string) {
	t.Helper()
	if got := f.InputDigit(r); got != want {
		t.Errorf("InputDigit(%q) = %q, want %q", r, got, want)
	}
}

func checkInputRemember(t *testing.T, f *AsYouTypeFormatter, r rune, want string) {
	t.Helper()
	if got := f.InputDigitAndRememberPosition(r); got != want {
		t.Errorf("InputDigitAndRememberPosition(%q) = %q, want %q", r, got, want)
	}
}

func checkRemembered(t *testing.T, f *AsYouTypeFormatter, want int) {
	t.Helper()
	if got := f.RememberedPosition(); got != want {
		t.Errorf("RememberedPosition() = %d, want %d", got, want)
	}
}

func checkNationalPrefix(t *testing.T, f *AsYouTypeFormatter, want string) {
	t.Helper()
	if got := f.extractedNationalPrefix; got != want {
		t.Errorf("extracted national prefix = %q, want %q", got, want)
	}
}
