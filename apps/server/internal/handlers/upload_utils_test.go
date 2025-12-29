package handlers

import (
	"testing"
)

func TestParseSizeToBytes(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"1024", 1024, false},
		{"1KB", 1024, false},
		{"1.5KB", 1536, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.25GB", int64(1.25 * 1024 * 1024 * 1024), false},
		{"12XB", 0, true},
	}

	for _, c := range cases {
		got, err := parseSizeToBytes(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("parseSizeToBytes(%q) expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseSizeToBytes(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("parseSizeToBytes(%q) got %d, want %d", c.in, got, c.want)
		}
	}
}

func TestIsAllowedType(t *testing.T) {
	// wildcard allows everything
	if !isAllowedType("a.xyz", "application/xyz", []string{"*"}) {
		t.Fatalf("wildcard '*' should allow any type")
	}
	if !isAllowedType("a.xyz", "application/xyz", []string{"*/*"}) {
		t.Fatalf("wildcard '*/*' should allow any type")
	}

	// extension match
	if !isAllowedType("doc.PDF", "application/pdf", []string{".pdf"}) {
		t.Fatalf(".pdf should be allowed by extension (case-insensitive)")
	}
	if isAllowedType("doc.txt", "text/plain", []string{".pdf"}) {
		t.Fatalf(".txt should not be allowed when only .pdf is configured")
	}

	// mime exact match
	if !isAllowedType("img.bin", "image/png", []string{"image/png"}) {
		t.Fatalf("image/png should be allowed by exact mime match")
	}

	// mime prefix match
	if !isAllowedType("picture.jpeg", "image/jpeg", []string{"image/*"}) {
		t.Fatalf("image/* should allow image/jpeg")
	}
	if isAllowedType("data.json", "application/json", []string{"image/*"}) {
		t.Fatalf("image/* should not allow application/json")
	}
}
