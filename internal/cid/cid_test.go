package cid

import "testing"

func TestSumDeterministic(t *testing.T) {
	a := Sum([]byte("hello"))
	b := Sum([]byte("hello"))
	c := Sum([]byte("world"))
	if a != b {
		t.Fatalf("same bytes produced different cids: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("different bytes produced same cid: %q", a)
	}
	if a == "" || a[0] != 'b' {
		t.Fatalf("unexpected cid %q", a)
	}
}
