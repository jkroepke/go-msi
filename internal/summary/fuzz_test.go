package summary_test

import (
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/summary"
)

// FuzzUnmarshal feeds corrupted property sets to the reader.
func FuzzUnmarshal(f *testing.F) {
	for _, codePage := range []int{1252, 1251} {
		raw, err := summary.Marshal([]summary.Property{
			{ID: summary.PIDCodepage, Value: codePage},
			{ID: summary.PIDTitle, Value: "Installation Database"},
			{ID: summary.PIDCreated, Value: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
			{ID: summary.PIDPageCount, Value: 500},
		}, codePage)
		if err != nil {
			f.Fatal(err)
		}

		f.Add(raw)
	}

	f.Fuzz(func(_ *testing.T, raw []byte) {
		_, _ = summary.Unmarshal(raw)
	})
}

// FuzzRoundTrip marshals fuzzed strings and reads them back unchanged.
func FuzzRoundTrip(f *testing.F) {
	f.Add("Title", "Author ö", int64(0), 1252)
	f.Add("", "Ж", int64(1<<40), 1251)
	f.Add("x", "y", int64(10000000000), 1252)
	f.Add("x", "y", int64(-11644473600), 1252)

	f.Fuzz(func(t *testing.T, title, author string, unix int64, codePage int) {
		when := time.Unix(unix, 0).UTC()
		props := []summary.Property{
			{ID: summary.PIDCodepage, Value: codePage},
			{ID: summary.PIDTitle, Value: title},
			{ID: summary.PIDAuthor, Value: author},
			{ID: summary.PIDCreated, Value: when},
		}

		raw, err := summary.Marshal(props, codePage)
		if err != nil {
			return
		}

		out, err := summary.Unmarshal(raw)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}

		if got := summary.Get(out, summary.PIDTitle); title != "" && got != title {
			t.Fatalf("title %q, wrote %q", got, title)
		}

		if got := summary.Get(out, summary.PIDAuthor); author != "" && got != author {
			t.Fatalf("author %q, wrote %q", got, author)
		}

		if got, ok := summary.Get(out, summary.PIDCreated).(time.Time); !ok || !got.Equal(when) {
			t.Fatalf("created %v, wrote %v", got, when)
		}
	})
}

// TestFileTimeRange is the FuzzRoundTrip finding: FileTime used UnixNano,
// which overflows silently outside 1678 to 2262.
func TestFileTimeRange(t *testing.T) {
	t.Parallel()

	for _, when := range []time.Time{
		time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1650, 6, 1, 0, 0, 0, 100, time.UTC),
		time.Date(2286, 11, 20, 17, 46, 40, 0, time.UTC),
		time.Date(30000, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if got := summary.FromFileTime(summary.FileTime(when)); !got.Equal(when) {
			t.Errorf("%v comes back as %v", when, got)
		}
	}

	if got := summary.FileTime(time.Date(1600, 12, 31, 23, 59, 59, 0, time.UTC)); got != 0 {
		t.Errorf("a time before 1601 gives %d, want 0", got)
	}

	if _, err := summary.Marshal([]summary.Property{{ID: summary.PIDCreated, Value: time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC)}}, 1252); err == nil {
		t.Error("Marshal accepted a time before 1601")
	}
}
