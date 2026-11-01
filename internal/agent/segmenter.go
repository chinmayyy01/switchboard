package agent

import "math"

const (
	SampleRate    = 16000
	frameSize     = 320
	prerollFrames = 15
)

type Segmenter struct {
	threshold float64
	noise     float64
	hangover  int
	minSpeech int
	maxFrames int

	pending  []int16
	preroll  [][]int16
	inSpeech bool
	silence  int
	speech   int
	buf      []int16
}

func NewSegmenter(sampleRate int) *Segmenter {
	fps := sampleRate / frameSize
	if fps < 1 {
		fps = 1
	}
	return &Segmenter{
		threshold: 280,
		noise:     60,
		hangover:  fps * 6 / 10,
		minSpeech: fps / 6,
		maxFrames: fps * 15,
		preroll:   make([][]int16, 0, prerollFrames),
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
	level := rms(frame)
	if !s.inSpeech {
		s.noise = 0.98*s.noise + 0.02*level
	}
	thr := math.Max(s.threshold, s.noise*3.2)

	if !s.inSpeech {
		if level < thr {
			s.pushPreroll(frame)
			return nil
		}
		s.inSpeech = true
		s.buf = s.buf[:0]
		for _, f := range s.preroll {
			s.buf = append(s.buf, f...)
		}
		s.speech = 0
		s.silence = 0
	}

	s.buf = append(s.buf, frame...)
	s.pushPreroll(frame)

	if level >= thr {
		s.silence = 0
		s.speech++
	} else {
		s.silence++
	}

	if s.silence >= s.hangover || len(s.buf) >= s.maxFrames*frameSize {
		return s.flush()
	}
	return nil
}

func (s *Segmenter) pushPreroll(frame []int16) {
	cp := make([]int16, len(frame))
	copy(cp, frame)
	s.preroll = append(s.preroll, cp)
	if len(s.preroll) > prerollFrames {
		s.preroll = s.preroll[len(s.preroll)-prerollFrames:]
	}
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

func (s *Segmenter) Reset() {
	s.pending = nil
	s.preroll = s.preroll[:0]
	s.inSpeech = false
	s.silence = 0
	s.speech = 0
	s.buf = s.buf[:0]
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
