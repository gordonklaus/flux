// Generated by Flux, not meant for human consumption.  Editing may make it unreadable by Flux.

package audioutil

import (
	"github.com/gordonklaus/flux/audio"
	"github.com/gordonklaus/portaudio"
)

func Play(x audio.Voice) () {
	var v int
	var v2 *audio.Params
	var v3 interface{}
	var v4 *portaudio.Stream
	var v5 *portaudio.Stream
	var v6 chan struct{}
	var v7 chan struct{}
	var v8 interface{}
	var v9 float64
	var v10 float64
	var v11 int
	var v12 audio.Voice
	var v13 int
	v12 = x
	v8 = x
	portaudio.Initialize()//;0
	defer portaudio.Terminate()//0;
	var v14 int
	x2 := make(chan struct{}, v14)
	v6 = x2
	v7 = x2
	const x3 = 96000
	v9 = x3
	v10 = x3
	const x4 = 64
	v13 = x4
	v = x4
	x5 := &audio.Params{SampleRate: v10, BufferSize: v13}
	v2 = x5
	v2.Set(v8)//;1
	const x6 = 1
	v11 = x6
	x7 := func (in [][]float32, out [][]float32) () {
		var v15 []float32
		var v16 [][]float32
		var v17 bool
		var v18 audio.Audio
		v16 = out
		x8, done := v12.Sing()
		v18 = x8
		v17 = done
		var v19 int
		x9 := &v16[v19]
		v15 = *x9
		for k := range v18 {
			var v20 = &v18[k]
			var v21 int
			var v22 float64
			var v23 float32
			v21 = k
			v22 = *v20
			x10 := (float32)(v22)
			v23 = x10
			v15[v21] = v23
		}
		if v17 {
			var v24 struct{}
			select {
			case v7 <- v24:
			default:
			}
		}
	}
	v3 = x7
	var v25 int
	x11, _ := portaudio.OpenDefaultStream(v25, v11, v9, v, v3)//0;
	v4 = x11
	v5 = x11
	v4.Start()//1;2
	<-v6//2;3
	v5.Stop()//3;
}
