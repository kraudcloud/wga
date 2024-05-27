package operator

import (
	"testing"
	"time"
)

func Fuzz_generateIndex(t *testing.F) {
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t.Fuzz(func(t *testing.T, mask uint8) {
		mask = mask % 128

		b := generateIndex(startTime, int(mask))
		if b.Cmp(bigMax(int(mask))) > 0 {
			t.Fail()
			t.Log("b", b, "max", bigMax(int(mask)), "mask", mask, "time", startTime.Unix())
		}
	})
}
