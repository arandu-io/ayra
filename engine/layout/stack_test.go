package layout_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
)

func BenchmarkStack(b *testing.B) {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}
	b.ReportAllocs()

	for b.Loop() {
		gtx.Ops.Reset()

		layout.Stack{}.Layout(gtx,
			layout.Expanded(emptyWidget{
				Size: image.Point{X: 60, Y: 60},
			}.Layout),
			layout.Stacked(emptyWidget{
				Size: image.Point{X: 30, Y: 30},
			}.Layout),
		)
	}
}

func BenchmarkBackground(b *testing.B) {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}
	b.ReportAllocs()

	for b.Loop() {
		gtx.Ops.Reset()

		layout.Background{}.Layout(gtx,
			emptyWidget{
				Size: image.Point{X: 60, Y: 60},
			}.Layout,
			emptyWidget{
				Size: image.Point{X: 30, Y: 30},
			}.Layout,
		)
	}
}

type emptyWidget struct {
	Size image.Point
}

func (w emptyWidget) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: w.Size}
}
