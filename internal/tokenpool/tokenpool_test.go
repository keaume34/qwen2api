package tokenpool

import (
	"errors"
	"testing"
	"time"

	"github.com/keaume34/qwen2api/internal/config"
)

func TestPoolRoundRobin(t *testing.T) {
	p := New([]config.Token{{Value: "a"}, {Value: "b"}, {Value: "c"}}, time.Minute)
	got := []string{}
	for i := 0; i < 6; i++ {
		tk, err := p.Take()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, tk.Value)
	}
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %s want %s", i, got[i], want[i])
		}
	}
}

func TestPoolCooldown(t *testing.T) {
	p := New([]config.Token{{Value: "a"}, {Value: "b"}}, 50*time.Millisecond)
	first, _ := p.Take()
	p.MarkBad(first.Value)
	second, _ := p.Take()
	if second.Value == first.Value {
		t.Errorf("expected different token, got %s twice", first.Value)
	}
	// Both consumed once; the bad one is on cooldown, the good one should be available.
	third, err := p.Take()
	if err != nil {
		t.Fatal(err)
	}
	if third.Value == first.Value {
		t.Errorf("bad token returned before cooldown elapsed")
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := p.Take(); err != nil {
		t.Errorf("after cooldown a token should be available: %v", err)
	}
}

func TestPoolEmpty(t *testing.T) {
	p := New(nil, time.Second)
	if _, err := p.Take(); !errors.Is(err, ErrNoToken) {
		t.Errorf("got %v want ErrNoToken", err)
	}
	if p.Size() != 0 {
		t.Errorf("size = %d", p.Size())
	}
}

func TestPoolSkipsEmptyValues(t *testing.T) {
	p := New([]config.Token{{Value: ""}, {Value: "x"}, {Value: ""}}, time.Second)
	if p.Size() != 1 {
		t.Fatalf("size = %d", p.Size())
	}
	tk, err := p.Take()
	if err != nil || tk.Value != "x" {
		t.Errorf("got %+v err=%v want value=x", tk, err)
	}
}
