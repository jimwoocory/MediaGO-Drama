package app

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("JW_APP_FFMPEG_FIXTURE") == "1" {
		if len(os.Args) < 2 {
			os.Exit(1)
		}
		last := os.Args[len(os.Args)-1]
		if last == "pipe:1" {
			_, err := os.Stdout.Write([]byte("fragmented-mp4"))
			if err != nil {
				os.Exit(1)
			}
		} else if err := os.WriteFile(last, []byte("rendered-mp4"), 0o600); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
