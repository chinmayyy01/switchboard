package agent

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/chinmay/switchboard/internal/escalate"
	"github.com/chinmay/switchboard/internal/metrics"
)

type Sink interface {
	EventJSON(v any) error
	Audio(pcm []int16) error
}

type CallSummary struct {
	Lang      string
	Turns     int
	Signal    escalate.Signal
	Escalated bool
	Reason    string
}

type Session struct {
	sel     Selector
	brain   Brain
	sink    Sink
	seg     *Segmenter
	policy  escalate.Policy
	tracker *escalate.Tracker

	history          []Message
	lang             string
	escalated        bool
	escalationReason string
	humanOwned       *atomic.Bool
}

func NewSession(sel Selector, brain Brain, sink Sink) *Session {
	return &Session{
		sel:        sel,
		brain:      brain,
		sink:       sink,
		seg:        NewSegmenter(SampleRate),
		policy:     escalate.DefaultRulePolicy(),
		tracker:    escalate.NewTracker(),
		humanOwned: &atomic.Bool{},
	}
}

func (s *Session) SetHumanOwned(v bool) { s.humanOwned.Store(v) }

func (s *Session) SetLanguage(lang string) { s.lang = lang }

func (s *Session) Summary() CallSummary {
	sig := s.tracker.Signal()
	return CallSummary{
		Lang:      s.lang,
		Turns:     sig.Turns,
		Signal:    sig,
		Escalated: s.escalated,
		Reason:    s.escalationReason,
	}
}

func (s *Session) OnAudio(ctx context.Context, pcm []int16) error {
	for _, utt := range s.seg.Process(pcm) {
		if err := s.respond(ctx, utt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) respond(ctx context.Context, pcm []int16) error {
	t0 := time.Now()

	tr, err := s.sel.Select(s.lang).STT.Transcribe(ctx, pcm, s.lang)
	if err != nil {
		return err
	}
	metrics.ObserveStage("stt", time.Since(t0))

	lang := s.lang
	if lang == "" {
		lang = tr.Lang
	}
	providers := s.sel.Select(lang)

	if err := s.sink.EventJSON(map[string]any{"type": "stt.final", "text": tr.Text, "lang": lang}); err != nil {
		return err
	}

	sig := s.tracker.Observe(tr.Text, tr.Confidence)
	if dec := s.policy.Decide(sig); dec.Escalate {
		s.escalated = true
		s.escalationReason = dec.Reason
		metrics.Escalation()
		if err := s.sink.EventJSON(map[string]any{"type": "escalation", "reason": dec.Reason}); err != nil {
			return err
		}
	}

	if s.humanOwned.Load() {
		return nil
	}

	t1 := time.Now()
	out, err := s.brain.Handle(ctx, Turn{UserText: tr.Text, History: s.history, Lang: lang})
	if err != nil {
		return err
	}
	metrics.ObserveStage("brain", time.Since(t1))

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
	t2 := time.Now()
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
	metrics.ObserveStage("tts", time.Since(t2))
	metrics.ObserveTurn(time.Since(t0))

	if err := s.sink.EventJSON(map[string]any{
		"type":     "latency",
		"stt_ms":   int64(time.Since(t0).Milliseconds()),
		"brain_ms": int64(time.Since(t1).Milliseconds()),
		"tts_ms":   int64(time.Since(t2).Milliseconds()),
		"turn_ms":  int64(time.Since(t0).Milliseconds()),
	}); err != nil {
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
