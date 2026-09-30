package gitlog

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	raw := "\x1eaaa111\x1faaa\x1f2026-09-01T00:30:00+03:00\x1fMoses Odeke\x1fMO@x.com\x1fp1\x1ffeat(auth): add OTP\n\npidmis_auth/otp.py\npidmis_auth/views.py\n" +
		"\x1ebbb222\x1fbbb\x1f2026-08-31T10:00:00+03:00\x1fizaiah-m\x1fi@x.com\x1fp1 p2\x1fMerge pull request #1\n\n"
	cs, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 commits, got %d", len(cs))
	}
	if cs[0].Subject != "feat(auth): add OTP" || len(cs[0].Files) != 2 || cs[0].AuthorEmail != "mo@x.com" || cs[0].Merge {
		t.Fatalf("bad first commit: %+v", cs[0])
	}
	if !cs[1].Merge || len(cs[1].Files) != 0 {
		t.Fatalf("bad merge commit: %+v", cs[1])
	}
}

func TestInRangeUsesAuthorCalendarDay(t *testing.T) {
	since, _ := time.Parse("2006-01-02", "2026-09-01")
	until, _ := time.Parse("2006-01-02", "2026-09-30")
	// 00:30 on the 1st in UTC+3 is still Aug 31 in UTC, but counts for September.
	d, _ := time.Parse(time.RFC3339, "2026-09-01T00:30:00+03:00")
	if !InRange(d, since, until) {
		t.Fatal("commit on the 1st (author zone) should be in range")
	}
	d, _ = time.Parse(time.RFC3339, "2026-10-01T00:10:00+03:00")
	if InRange(d, since, until) {
		t.Fatal("commit on Oct 1 should be out of range")
	}
}
