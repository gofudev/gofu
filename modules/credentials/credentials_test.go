package credentials

import (
	"testing"
)

func TestGet(t *testing.T) {
	t.Run("set returns value", func(t *testing.T) {
		t.Setenv("TEST_SECRET", "hello")
		v, err := Get("TEST_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "hello" {
			t.Errorf("got %q, want %q", v, "hello")
		}
	})

	t.Run("unset returns error", func(t *testing.T) {
		t.Setenv("TEST_SECRET", "")
		_, err := Get("TEST_SECRET_UNSET_XYZ123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		t.Setenv("TEST_SECRET_EMPTY", "")
		_, err := Get("TEST_SECRET_EMPTY")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
