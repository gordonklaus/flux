// Generated by Flux, not meant for human consumption.  Editing may make it unreadable by Flux.

package audio

func (x *SineBeats) AddBeat(amp float64, sineFreq float64, beatFreq float64, beatWidth float64) () {
	var v *[]*SineBeat
	var v2 []*SineBeat
	var v3 *SineBeats
	var v4 float64
	var v5 float64
	var v6 *SineBeat
	var v7 float64
	var v8 float64
	var v9 []*SineBeat
	v5 = beatFreq
	v8 = beatWidth
	v3 = x
	v4 = amp
	v7 = sineFreq
	x2 := &v3.Beats
	v9 = *x2
	v = x2
	x3 := NewSineBeat(v4, v7, v5, v8)
	v6 = x3
	x4 := append(v9, v6)
	v2 = x4
	*v = v2
}