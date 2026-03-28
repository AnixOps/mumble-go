package stream

import "math"

// VAD detects voice activity based on audio energy.
type VAD struct {
	threshold float64
	speaking  bool
	onChange  func(bool)
}

// NewVAD creates a VAD with the given energy threshold.
func NewVAD(threshold float64, onChange func(bool)) *VAD {
	return &VAD{
		threshold: threshold,
		speaking:  false,
		onChange:  onChange,
	}
}

// Process examines PCM data (16-bit signed LE, 48kHz mono) and returns
// true if speech is detected.
func (v *VAD) Process(pcm []byte) bool {
	energy := computeRMS(pcm)
	speaking := energy >= v.threshold

	if speaking != v.speaking {
		v.speaking = speaking
		if v.onChange != nil {
			v.onChange(speaking)
		}
	}

	return speaking
}

// IsSpeaking returns the current VAD state.
func (v *VAD) IsSpeaking() bool {
	return v.speaking
}

// computeRMS computes the root mean square energy of 16-bit PCM samples.
func computeRMS(pcm []byte) float64 {
	if len(pcm) < 2 {
		return 0
	}

	n := len(pcm) / 2
	var sum float64
	for i := 0; i < n; i++ {
		sample := int16(pcm[i*2]) | (int16(pcm[i*2+1]) << 8)
		sum += float64(sample) * float64(sample)
	}
	return math.Sqrt(sum / float64(n))
}
