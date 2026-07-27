package utils

import (
	"fmt"
	"strings"
	"time"
)

var monthsID = []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func FormatTanggalID(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), monthsID[int(t.Month())], t.Year())
}

func FormatTanggalJamID(t time.Time) string {
	return fmt.Sprintf("%s, %02d:%02d WIB", FormatTanggalID(t), t.Hour(), t.Minute())
}

func FormatRupiah(amount float64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	whole := int64(amount)
	s := fmt.Sprintf("%d", whole)
	parts := []string{}
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := "Rp " + strings.Join(parts, ".")
	if negative {
		out = "-" + out
	}
	return out
}
