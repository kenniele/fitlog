package controlcenter

import "testing"

func TestFormatDurationRU(t *testing.T) {
	tests := []struct {
		name    string
		seconds int64
		want    string
	}{
		{name: "minutes", seconds: 1_492, want: "25 мин"},
		{name: "hours and minutes", seconds: 24_480, want: "6 ч 48 мин"},
		{name: "whole hours", seconds: 25_200, want: "7 ч 0 мин"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatDurationRU(test.seconds); got != test.want {
				t.Fatalf("formatDurationRU(%d) = %q, want %q", test.seconds, got, test.want)
			}
		})
	}
}
