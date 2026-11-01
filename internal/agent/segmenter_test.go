package agent

import "testing"

func frame(amp int16) []int16 {
	f := make([]int16, frameSize)
	for i := range f {
		f[i] = amp
	}
	return f
}

func feed(seg *Segmenter, f []int16, n int) [][]int16 {
	var out [][]int16
	for i := 0; i < n; i++ {
		out = append(out, seg.Process(f)...)
	}
	return out
}

func TestSegmenterKeepsPreroll(t *testing.T) {
	seg := NewSegmenter(SampleRate)

	feed(seg, frame(0), 30)
	utts := feed(seg, frame(2000), 50)
	utts = append(utts, feed(seg, frame(0), 40)...)

	if len(utts) != 1 {
		t.Fatalf("want 1 utterance, got %d", len(utts))
	}
	utt := utts[0]
	if len(utt) < 60*frameSize {
		t.Fatalf("utterance too short: %d samples", len(utt))
	}
	if utt[0] != 0 {
		t.Fatalf("expected preroll silence at start, got %d", utt[0])
	}
}

func TestSegmenterDropsBlip(t *testing.T) {
	seg := NewSegmenter(SampleRate)
	feed(seg, frame(0), 10)
	utts := feed(seg, frame(2000), 2)
	utts = append(utts, feed(seg, frame(0), 40)...)
	if len(utts) != 0 {
		t.Fatalf("blip should be dropped, got %d utterances", len(utts))
	}
}

func TestSegmenterReset(t *testing.T) {
	seg := NewSegmenter(SampleRate)
	feed(seg, frame(2000), 20)
	seg.Reset()
	utts := feed(seg, frame(0), 40)
	if len(utts) != 0 {
		t.Fatalf("reset should clear buffered speech, got %d", len(utts))
	}
}

func TestSplitSentences(t *testing.T) {
	got := splitSentences("Hi there. How are you? I can help!")
	if len(got) != 3 {
		t.Fatalf("want 3 sentences, got %d: %#v", len(got), got)
	}
}

func TestHasWords(t *testing.T) {
	if hasWords(".") {
		t.Fatal("punctuation should not count as words")
	}
	if !hasWords("ok") {
		t.Fatal("letters should count as words")
	}
}

func TestCleanForSpeech(t *testing.T) {
	if got := cleanForSpeech("**Hi** _there_"); got != "Hi there" {
		t.Fatalf("got %q", got)
	}
}

func TestStartsWithAck(t *testing.T) {
	if !startsWithAck("Okay, what's the address?") {
		t.Fatal("expected acknowledgment")
	}
	if !startsWithAck("Got it, leak repair.") {
		t.Fatal("expected acknowledgment")
	}
	if startsWithAck("I can help with that.") {
		t.Fatal("did not expect acknowledgment")
	}
}
