// Generated by Flux, not meant for human consumption.  Editing may make it unreadable by Flux.

package audio

func (x *PolyphonicInstrument) Play(note Note) () {
	var v Note
	var v2 *MultiVoice
	var v3 func(_ Note) Voice
	var v4 Voice
	var v5 *PolyphonicInstrument
	var v6 *PolyphonicInstrument
	v5 = x
	v6 = x
	v = note
	x2 := &v6.MultiVoice
	v2 = *x2
	x3 := &v5.newVoice
	v3 = *x3
	x4 := v3(v)
	v4 = x4
	v2.StartVoice(v4)
}
