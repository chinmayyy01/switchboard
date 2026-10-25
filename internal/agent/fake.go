package agent

import (
	"context"
	"math"
)

type Utterance struct {
	Text string
	Lang string
}

type FakeSTT struct {
	Script []Utterance
	i      int
}

func NewFakeSTT() *FakeSTT {
	return &FakeSTT{Script: []Utterance{
		{Text: "my kitchen sink is leaking badly", Lang: "en"},
		{Text: "12 Main Street", Lang: "en"},
		{Text: "tomorrow morning", Lang: "en"},
		{Text: "5551234567", Lang: "en"},
		{Text: "yes", Lang: "en"},
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

type FakeTTS struct{ SampleRate int }

func NewFakeTTS() *FakeTTS { return &FakeTTS{SampleRate: 16000} }

func (f *FakeTTS) Synthesize(_ context.Context, _ string, lang string) ([]int16, error) {
	freq := map[string]float64{"en": 440, "hi": 523, "es": 392}[lang]
	if freq == 0 {
		freq = 440
	}
	n := f.SampleRate * 300 / 1000
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = int16(3000 * math.Sin(2*math.Pi*freq*float64(i)/float64(f.SampleRate)))
	}
	return pcm, nil
}

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

type FakeBrain struct{}

func (FakeBrain) Handle(_ context.Context, turn Turn) (Outcome, error) {
	return Outcome{Reply: "You said: " + turn.UserText, State: "echo"}, nil
}
