package config_test

import (
	"testing"

	"github.com/jkroepke/go-msi/pkg/config"
)

func TestLZXWindow(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]int{"lzx": 21, "lzx:15": 15, "lzx:21": 21, "mszip": 0, "none": 0, "lzx:x": 0, "lzx15": 0} {
		window, ok := config.LZXWindow(value)
		if window != want || ok != (want != 0) {
			t.Errorf("LZXWindow(%q) = %d, %v; want %d", value, window, ok, want)
		}
	}

	for _, value := range config.Compressions() {
		if window, ok := config.LZXWindow(value); ok && (window < 15 || window > 21) {
			t.Errorf("Compressions() lists %q, window %d", value, window)
		}
	}
}

func TestDefaultCompressionIsLZX(t *testing.T) {
	t.Parallel()

	if window, ok := config.LZXWindow(config.DefaultCompression); !ok || window != 21 {
		t.Fatalf("default %q, want LZX:21", config.DefaultCompression)
	}
}
