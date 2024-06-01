//go:build windows
// +build windows

package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// https://gioui.org/doc/architecture

func main() {
	go func() {
		w := new(app.Window)
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			l := material.H1(th, "Hello, Gio")
			l.Color = color.NRGBA{R: 127, G: 0, B: 0, A: 255}
			l.Alignment = text.Middle
			l.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

type SplitVisual struct{}

func (s SplitVisual) Layout(gtx layout.Context, left, right layout.Widget) layout.Dimensions {
	leftsize := gtx.Constraints.Min.X / 2
	rightsize := gtx.Constraints.Min.X - leftsize

	{
		gtx := gtx
		gtx.Constraints = layout.Exact(image.Pt(leftsize, gtx.Constraints.Max.Y))
		left(gtx)
	}

	{
		gtx := gtx
		gtx.Constraints = layout.Exact(image.Pt(rightsize, gtx.Constraints.Max.Y))
		trans := op.Offset(image.Pt(leftsize, 0)).Push(gtx.Ops)
		right(gtx)
		trans.Pop()
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// func exampleSplitVisual(gtx layout.Context, th *material.Theme) layout.Dimensions {
// 	return SplitVisual{}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 		return FillWithLabel(gtx, th, "Left", red)
// 	}, func(gtx layout.Context) layout.Dimensions {
// 		return FillWithLabel(gtx, th, "Right", blue)
// 	})
// }

// func FillWithLabel(gtx layout.Context, th *material.Theme, text string, backgroundColor color.NRGBA) layout.Dimensions {
// 	ColorBox(gtx, gtx.Constraints.Max, backgroundColor)
// 	return layout.Center.Layout(gtx, material.H3(th, text).Layout)
// }
