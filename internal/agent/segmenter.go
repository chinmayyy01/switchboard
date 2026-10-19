package agent

import "math"

const (
	SampleRate = 16000
	frameSize = 320
)

type Segmenter struct {
	threshold float64
	hangover  int
	minSpeech int
	maxFrames int

	pending  []int16
	inSpeech bool
	silence  int
	speech   int
	buf      []int16
}

func NewSegmenter(sampleRate int) *Segmenter {
	fps := sampleRate / frameSize
	return &Segmenter{
		threshold: 500,
		hangover:  fps * 7 / 10,
		minSpeech: fps / 8,
		maxFrames: fps * 15,
	}
}

func (s *Segmenter) Process(pcm []int16) [][]int16 {
	var done [][]int16
	s.pending = append(s.pending, pcm...)
	for len(s.pending) >= frameSize {
		frame := s.pending[:frameSize]
		s.pending = s.pending[frameSize:]
		if utt := s.processFrame(frame); utt != nil {
			done = append(done, utt)
		}
	}
	return done
}

func (s *Segmenter) processFrame(frame []int16) []int16 {
	if rms(frame) >= s.threshold {
		if !s.inSpeech {
			s.inSpeech = true
			s.buf = s.buf[:0]
			s.speech = 0
		}
		s.silence = 0
		s.speech++
		s.buf = append(s.buf, frame...)
		if len(s.buf) >= s.maxFrames*frameSize {
			return s.flush()
		}
		return nil
	}

	if !s.inSpeech {
		return nil
	}
	s.silence++
	s.buf = append(s.buf, frame...)
	if s.silence >= s.hangover {
		return s.flush()
	}
	return nil
}

func (s *Segmenter) flush() []int16 {
	var out []int16
	if s.speech >= s.minSpeech {
		out = make([]int16, len(s.buf))
		copy(out, s.buf)
	}
	s.inSpeech = false
	s.silence = 0
	s.speech = 0
	s.buf = s.buf[:0]
	return out
}

func rms(frame []int16) float64 {
	if len(frame) == 0 {
		return 0
	}
	var sum float64
	for _, v := range frame {
		f := float64(v)
		sum += f * f
	}
	return math.Sqrt(sum / float64(len(frame)))
}
