package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/chinmay/switchboard/internal/escalate"
	"github.com/chinmay/switchboard/internal/guard"
	"github.com/chinmay/switchboard/internal/metrics"
	"github.com/chinmay/switchboard/internal/obs"
	"go.opentelemetry.io/otel/attribute"
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
	turns            int
	emptyTurns       int
	firstAudio       atomic.Int64
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

func (s *Session) Interrupt() { s.seg.Reset() }

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
	if s.humanOwned.Load() {
		return nil
	}
	s.firstAudio.Store(0)

	ctx, span := obs.Tracer().Start(ctx, "turn")
	defer span.End()

	t0 := time.Now()

	sttCtx, sttSpan := obs.Tracer().Start(ctx, "stt")
	tr, err := s.sel.Select(s.lang).STT.Transcribe(sttCtx, pcm, s.lang)
	if err != nil {
		sttSpan.RecordError(err)
		sttSpan.End()
		return err
	}
	sttSpan.End()
	sttDone := time.Now()
	metrics.ObserveStage("stt", sttDone.Sub(t0))

	lang := s.lang
	if lang == "" {
		lang = tr.Lang
	}
	if lang == "" {
		lang = "en"
	}
	span.SetAttributes(attribute.String("lang", lang))

	if err := s.sink.EventJSON(map[string]any{"type": "stt.final", "text": tr.Text, "lang": lang}); err != nil {
		return err
	}

	if !hasWords(tr.Text) {
		s.emptyTurns++
		if s.emptyTurns >= 2 {
			s.escalated = true
			s.escalationReason = "no_input"
			metrics.Escalation()
			if err := s.sink.EventJSON(map[string]any{"type": "escalation", "reason": "no_input"}); err != nil {
				return err
			}
			return s.speak(ctx, "I'm having trouble hearing you. Let me connect you with a teammate.", lang)
		}
		return s.speak(ctx, "Sorry, I didn't catch that. Could you say that again?", lang)
	}
	s.emptyTurns = 0

	if guard.LooksLikeInjection(tr.Text) {
		s.escalated = true
		s.escalationReason = "prompt_injection"
		metrics.Escalation()
		if err := s.sink.EventJSON(map[string]any{"type": "guardrail", "kind": "prompt_injection"}); err != nil {
			return err
		}
		return s.sink.EventJSON(map[string]any{"type": "escalation", "reason": "prompt_injection"})
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

	t1 := time.Now()
	s.turns++
	filler := s.startFiller(ctx, lang)
	fillerReady := make(chan struct{})
	go func() {
		defer close(fillerReady)
		audio, ok := <-filler
		if !ok || len(audio) == 0 {
			return
		}
		if err := s.sink.EventJSON(map[string]any{"type": "agent.speaking"}); err != nil {
			return
		}
		s.markAudio()
		s.sink.Audio(audio)
	}()

	spoke := false
	var ttsStream time.Duration
	onSentence := func(sentence string) error {
		if !spoke {
			select {
			case <-fillerReady:
			case <-time.After(300 * time.Millisecond):
			}
			s.seg.Reset()
			if err := s.sink.EventJSON(map[string]any{"type": "agent.speaking"}); err != nil {
				return err
			}
			spoke = true
		}

		ttsCtx, ttsSpan := obs.Tracer().Start(ctx, "tts")
		start := time.Now()
		audio, err := s.sel.Select(lang).TTS.Synthesize(ttsCtx, sentence, lang)
		ttsStream += time.Since(start)
		if err != nil {
			ttsSpan.RecordError(err)
			ttsSpan.End()
			return err
		}
		ttsSpan.End()
		if len(audio) == 0 {
			return nil
		}
		s.markAudio()
		return s.sink.Audio(audio)
	}

	brainCtx, brainSpan := obs.Tracer().Start(ctx, "brain")
	out, err := s.brain.Handle(brainCtx, Turn{
		UserText:   tr.Text,
		History:    s.history,
		Lang:       lang,
		OnSentence: onSentence,
	})
	if err != nil {
		brainSpan.RecordError(err)
		brainSpan.End()
		return err
	}
	brainSpan.End()
	brainDone := time.Now()
	metrics.ObserveStage("brain", brainDone.Sub(t1))

	if out.Escalate && !s.escalated {
		s.escalated = true
		reason := out.EndReason
		if reason == "" {
			reason = "policy"
		}
		s.escalationReason = reason
		metrics.Escalation()
		if err := s.sink.EventJSON(map[string]any{"type": "escalation", "reason": reason}); err != nil {
			return err
		}
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
	if len(out.Citations) > 0 {
		if err := s.sink.EventJSON(map[string]any{"type": "citation", "docs": out.Citations}); err != nil {
			return err
		}
	}

	reply := out.Reply
	if !hasWords(reply) {
		reply = "Sorry, I didn't catch that. Could you say that again?"
	}

	if !out.Streamed {
		select {
		case <-fillerReady:
		case <-time.After(300 * time.Millisecond):
		}
	}

	var ttsMs int64
	if out.Streamed {
		if err := s.sink.EventJSON(map[string]any{"type": "agent.text", "text": reply}); err != nil {
			return err
		}
		if err := s.sink.EventJSON(map[string]any{"type": "agent.done"}); err != nil {
			return err
		}
		metrics.ObserveStage("tts", ttsStream)
		ttsMs = ttsStream.Milliseconds()
	} else {
		t2 := time.Now()
		if err := s.speak(ctx, reply, lang); err != nil {
			return err
		}
		metrics.ObserveStage("tts", time.Since(t2))
		ttsMs = time.Since(t2).Milliseconds()
	}

	metrics.ObserveTurn(time.Since(t0))
	span.SetAttributes(attribute.String("state", out.State), attribute.Bool("escalated", out.Escalate))

	firstAudioMs := int64(-1)
	if t := s.firstAudio.Load(); t != 0 {
		firstAudioMs = (t - t0.UnixNano()) / int64(time.Millisecond)
		metrics.ObserveStage("first_audio", time.Duration(firstAudioMs)*time.Millisecond)
	}

	if err := s.sink.EventJSON(map[string]any{
		"type":           "latency",
		"stt_ms":         sttDone.Sub(t0).Milliseconds(),
		"brain_ms":       brainDone.Sub(t1).Milliseconds(),
		"tts_ms":         ttsMs,
		"first_audio_ms": firstAudioMs,
		"turn_ms":        time.Since(t0).Milliseconds(),
	}); err != nil {
		return err
	}

	s.history = append(s.history,
		Message{Role: "user", Content: tr.Text},
		Message{Role: "assistant", Content: reply},
	)

	if out.EndCall {
		return s.sink.EventJSON(map[string]any{"type": "call.end", "reason": out.EndReason})
	}
	return nil
}

func (s *Session) speak(ctx context.Context, text, lang string) error {
	text = cleanForSpeech(text)
	if text == "" {
		return nil
	}

	s.seg.Reset()
	if err := s.sink.EventJSON(map[string]any{"type": "agent.text", "text": text}); err != nil {
		return err
	}
	if err := s.sink.EventJSON(map[string]any{"type": "agent.speaking"}); err != nil {
		return err
	}

	for _, part := range splitSentences(text) {
		ttsCtx, ttsSpan := obs.Tracer().Start(ctx, "tts")
		audio, err := s.sel.Select(lang).TTS.Synthesize(ttsCtx, part, lang)
		if err != nil {
			ttsSpan.RecordError(err)
			ttsSpan.End()
			s.sink.EventJSON(map[string]any{"type": "agent.done"})
			return err
		}
		ttsSpan.End()
		if len(audio) == 0 {
			continue
		}
		s.markAudio()
		if err := s.sink.Audio(audio); err != nil {
			return err
		}
	}

	return s.sink.EventJSON(map[string]any{"type": "agent.done"})
}

func splitSentences(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '.', '!', '?', '\n':
			if part := strings.TrimSpace(text[start : i+1]); part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		out = append(out, tail)
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func hasWords(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func cleanForSpeech(s string) string {
	s = strings.NewReplacer("*", "", "#", "", "`", "", "_", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

var fillerTexts = []string{"Mm-hmm.", "Okay.", "Right."}

var ackPrefixes = []string{
	"okay", "ok ", "got it", "sure", "right", "mm-hmm", "hmm",
	"yes", "yeah", "great", "alright", "thanks", "no problem",
	"of course", "good",
}

func startsWithAck(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	for _, p := range ackPrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

func (s *Session) markAudio() {
	if s.firstAudio.Load() == 0 {
		s.firstAudio.CompareAndSwap(0, time.Now().UnixNano())
	}
}

func (s *Session) startFiller(ctx context.Context, lang string) <-chan []int16 {
	ch := make(chan []int16, 1)
	if s.turns%2 == 0 {
		close(ch)
		return ch
	}

	text := fillerTexts[s.turns%len(fillerTexts)]
	go func() {
		defer close(ch)
		ttsCtx, span := obs.Tracer().Start(ctx, "tts.filler")
		audio, err := s.sel.Select(lang).TTS.Synthesize(ttsCtx, text, lang)
		if err != nil {
			span.RecordError(err)
			span.End()
			return
		}
		span.End()
		if len(audio) > 0 {
			ch <- audio
		}
	}()
	return ch
}
