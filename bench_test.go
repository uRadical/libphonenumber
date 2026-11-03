package phonenumber

import "testing"

func BenchmarkParse(b *testing.B) {
	defaultSet() // build the indexes outside the timer
	for b.Loop() {
		if _, err := Parse("020 7946 0958", "GB"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseInternational(b *testing.B) {
	for b.Loop() {
		if _, err := Parse("+44 20 7946 0958", ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValid(b *testing.B) {
	n, _ := Parse("+44 20 7946 0958", "")
	for b.Loop() {
		_ = n.Valid()
	}
}

func BenchmarkFormatInternational(b *testing.B) {
	n, _ := Parse("+44 20 7946 0958", "")
	for b.Loop() {
		_ = n.Format(International)
	}
}
