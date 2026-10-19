package agent

import (
	"context"
	"math"
)

// Fakes let the whole Phase 1 loop run with no models installed, at $0, and
// make the pipeline testable. Swap for real providers one line at a time.

// Utterance is one canned line of "speech" with the language the fake STT
// should report — i.e. the fake's language identification.
type Utterance struct {
	Text string
	Lang string
}

// FakeSTT walks through a canned script, reporting each line's language. The
// script is a home-services call (plumbing/HVAC), our Phase 1 vertical, and it
// deliberately switches language to exercise the router.
type FakeSTT struct {
	Script []Utterance
	i      int
}

func NewFakeSTT() *FakeSTT {
	return &FakeSTT{Script: []Utterance{
		{Text: "my kitchen sink is leaking badly, I need someone today", Lang: "en"},
		{Text: "कल सुबह आ सकते हो?", Lang: "hi"},
		{Text: "yes please book that slot", Lang: "en"},
	}}
}

func (f *FakeSTT) Transcribe(_ context.Context, _ []int16, _ string) (Transcript, error) {
	u := Utterance{Text: "…", Lang: "en"}
	if len(f.Script) > 0 {
		u = f.Script[f.i%len(f.Script)]
		f.i++
	}
	return Transcript{Text: u.Text, Lang: u.Lang, Confidence: 1}, nil
}

// FakeTTS returns a short beep so you can *hear* the returned PCM. The pitch
// varies by language, so you can tell the language reached the audio stage.
type FakeTTS struct{ SampleRate int }

func NewFakeTTS() *FakeTTS { return &FakeTTS{SampleRate: 16000} }

func (f *FakeTTS) Synthesize(_ context.Context, _ string, lang string) ([]int16, error) {
	freq := map[string]float64{"en": 440, "hi": 523, "es": 392}[lang]
	if freq == 0 {
		freq = 440
	}
	n := f.SampleRate * 300 / 1000 // 300 ms
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = int16(3000 * math.Sin(2*math.Pi*freq*float64(i)/float64(f.SampleRate)))
	}
	return pcm, nil
}

// FakeLLM echoes the last user message.
type FakeLLM struct{}

func (FakeLLM) Respond(_ context.Context, convo []Message, _ []Tool) (Reply, error) {
	return Reply{Text: "echo: " + lastUser(convo)}, nil
}

func lastUser(convo []Message) string {
	for i := len(convo) - 1; i >= 0; i-- {
		if convo[i].Role == "user" {
			return convo[i].Content
		}
	}
	return ""
}

// FakeSelector returns one stable set of fakes for any language. Real code would
// return language-specific models (Whisper/Piper vs Sarvam) here. It holds the
// instances so their per-session state (the script index) survives across turns.
type FakeSelector struct {
	stt STT
	tts TTS
	llm LLM
}

func NewFakeSelector() *FakeSelector {
	return &FakeSelector{stt: NewFakeSTT(), tts: NewFakeTTS(), llm: FakeLLM{}}
}

func (f *FakeSelector) Select(_ string) Providers {
	return Providers{STT: f.stt, TTS: f.tts, LLM: f.llm}
}

// FakeBrain is the placeholder for the booking state machine. Replace me.
type FakeBrain struct{}

func (FakeBrain) Handle(_ context.Context, turn Turn) (Outcome, error) {
	return Outcome{Reply: "You said: " + turn.UserText, State: "echo"}, nil
}
