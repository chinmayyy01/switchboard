package agent

import (
	"context"
	"testing"
)

type recSink struct {
	events []map[string]any
	audio  int
}

func (r *recSink) EventJSON(v any) error {
	if m, ok := v.(map[string]any); ok {
		r.events = append(r.events, m)
	}
	return nil
}

func (r *recSink) Audio(pcm []int16) error {
	r.audio += len(pcm)
	return nil
}

func TestSessionDetectsLanguageAndRunsPipeline(t *testing.T) {
	ctx := context.Background()
	sink := &recSink{}
	s := NewSession(NewFakeSelector(), FakeBrain{}, sink)

	loud := make([]int16, frameSize)
	for i := range loud {
		loud[i] = 6000
	}
	silent := make([]int16, frameSize)

	for i := 0; i < 25; i++ {
		if err := s.OnAudio(ctx, loud); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 60; i++ {
		if err := s.OnAudio(ctx, silent); err != nil {
			t.Fatal(err)
		}
	}

	if sink.audio == 0 {
		t.Fatal("expected TTS audio, got none")
	}

	var sawFinal bool
	for _, e := range sink.events {
		if e["type"] == "stt.final" {
			sawFinal = true
			if e["lang"] != "en" {
				t.Errorf("expected detected lang en, got %v", e["lang"])
			}
			t.Logf("detected: text=%q lang=%v", e["text"], e["lang"])
		}
	}
	if !sawFinal {
		t.Fatal("missing stt.final")
	}
}
