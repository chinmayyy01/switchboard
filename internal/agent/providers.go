package agent

import "context"

type STT interface {
	Transcribe(ctx context.Context, pcm []int16, lang string) (Transcript, error)
}

type LLM interface {
	Respond(ctx context.Context, convo []Message, tools []Tool) (Reply, error)
}

type TTS interface {
	Synthesize(ctx context.Context, text, lang string) ([]int16, error)
}

type Brain interface {
	Handle(ctx context.Context, turn Turn) (Outcome, error)
}

type Providers struct {
	STT STT
	TTS TTS
	LLM LLM
}

type Selector interface {
	Select(lang string) Providers
}
