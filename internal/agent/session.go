package agent

import "context"

type Sink interface {
	EventJSON(v any) error
	Audio(pcm []int16) error 
}

type Session struct {
	sel   Selector
	brain Brain
	sink  Sink
	seg   *Segmenter

	history []Message
	lang    string
}

func NewSession(sel Selector, brain Brain, sink Sink) *Session {
	return &Session{sel: sel, brain: brain, sink: sink, seg: NewSegmenter(SampleRate)}
}

func (s *Session) SetLanguage(lang string) { s.lang = lang }

func (s *Session) OnAudio(ctx context.Context, pcm []int16) error {
	for _, utt := range s.seg.Process(pcm) {
		if err := s.respond(ctx, utt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) respond(ctx context.Context, pcm []int16) error {
	tr, err := s.sel.Select(s.lang).STT.Transcribe(ctx, pcm, s.lang)
	if err != nil {
		return err
	}

	lang := s.lang
	if lang == "" {
		lang = tr.Lang
	}
	providers := s.sel.Select(lang)

	if err := s.sink.EventJSON(map[string]any{"type": "stt.final", "text": tr.Text, "lang": lang}); err != nil {
		return err
	}

	out, err := s.brain.Handle(ctx, Turn{UserText: tr.Text, History: s.history, Lang: lang})
	if err != nil {
		return err
	}

	if out.ToolCalled != nil {
		if err := s.sink.EventJSON(map[string]any{
			"type": "tool.call", "name": out.ToolCalled.Name, "args": out.ToolCalled.Args,
		}); err != nil {
			return err
		}
	}
	if out.State != "" {
		if err := s.sink.EventJSON(map[string]any{"type": "agent.state", "state": out.State}); err != nil {
			return err
		}
	}
	if err := s.sink.EventJSON(map[string]any{"type": "agent.text", "text": out.Reply}); err != nil {
		return err
	}

	if err := s.sink.EventJSON(map[string]any{"type": "agent.speaking"}); err != nil {
		return err
	}
	audio, err := providers.TTS.Synthesize(ctx, out.Reply, lang)
	if err != nil {
		return err
	}
	if err := s.sink.Audio(audio); err != nil {
		return err
	}
	if err := s.sink.EventJSON(map[string]any{"type": "agent.done"}); err != nil {
		return err
	}

	s.history = append(s.history,
		Message{Role: "user", Content: tr.Text},
		Message{Role: "assistant", Content: out.Reply},
	)

	if out.EndCall {
		return s.sink.EventJSON(map[string]any{"type": "call.end", "reason": out.EndReason})
	}
	return nil
}
