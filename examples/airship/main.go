// Copyright 2016 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build example

package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const (
	screenWidth  = 320
	screenHeight = 240
	maxAngle     = 256
	maxLean      = 16
)

var (
	skyColor  = color.RGBA{0x66, 0xcc, 0xff, 0xff}
	thePlayer = &player{
		x16:   16 * 100,
		y16:   16 * 200,
		angle: maxAngle * 3 / 4,
	}
	gophersImage           *ebiten.Image
	repeatedGophersImage   *ebiten.Image
	groundImage            *ebiten.Image
	perspectiveGroundImage *ebiten.Image
	fogImage               *ebiten.Image
)

func init() {
	var err error
	gophersImage, _, err = ebitenutil.NewImageFromFile("_resources/images/gophers.jpg", ebiten.FilterDefault)
	if err != nil {
		panic(err)
	}
	groundImage, _ = ebiten.NewImage(screenWidth*2, screenHeight*2/3+50, ebiten.FilterDefault)
	perspectiveGroundImage, _ = ebiten.NewImage(screenWidth*2, screenHeight, ebiten.FilterDefault)

	const repeat = 5
	w, h := gophersImage.Size()
	repeatedGophersImage, _ = ebiten.NewImage(w*repeat, h*repeat, ebiten.FilterDefault)
	for j := 0; j < repeat; j++ {
		for i := 0; i < repeat; i++ {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(w*i), float64(h*j))
			repeatedGophersImage.DrawImage(gophersImage, op)
		}
	}

	const fogHeight = 8
	w, _ = perspectiveGroundImage.Size()
	fogRGBA := image.NewRGBA(image.Rect(0, 0, w, fogHeight))
	for j := 0; j < fogHeight; j++ {
		a := uint32(float64(fogHeight-1-j) * 0xff / (fogHeight - 1))
		clr := skyColor
		r, g, b, oa := uint32(clr.R), uint32(clr.G), uint32(clr.B), uint32(clr.A)
		clr.R = uint8(r * a / oa)
		clr.G = uint8(g * a / oa)
		clr.B = uint8(b * a / oa)
		clr.A = uint8(a)
		for i := 0; i < w; i++ {
			fogRGBA.SetRGBA(i, j, clr)
		}
	}
	fogImage, _ = ebiten.NewImageFromImage(fogRGBA, ebiten.FilterDefault)
}

type player struct {
	x16   int
	y16   int
	angle int
	lean  int
}

func round(x float64) float64 {
	return math.Floor(x + 0.5)
}

func (p *player) MoveForward() {
	w, h := gophersImage.Size()
	mx := w * 16
	my := h * 16
	s, c := math.Sincos(float64(p.angle) * 2 * math.Pi / maxAngle)
	p.x16 += int(round(16*c) * 2)
	p.y16 += int(round(16*s) * 2)
	for mx <= p.x16 {
		p.x16 -= mx
	}
	for my <= p.y16 {
		p.y16 -= my
	}
	for p.x16 < 0 {
		p.x16 += mx
	}
	for p.y16 < 0 {
		p.y16 += my
	}
}

func (p *player) RotateRight() {
	p.angle++
	if maxAngle <= p.angle {
		p.angle -= maxAngle
	}
	p.lean++
	if maxLean < p.lean {
		p.lean = maxLean
	}
}

func (p *player) RotateLeft() {
	p.angle--
	if p.angle < 0 {
		p.angle += maxAngle
	}
	p.lean--
	if p.lean < -maxLean {
		p.lean = -maxLean
	}
}

func (p *player) Stabilize() {
	if 0 < p.lean {
		p.lean--
	}
	if p.lean < 0 {
		p.lean++
	}
}

func (p *player) Position() (int, int) {
	return p.x16, p.y16
}

func (p *player) Angle() int {
	return p.angle
}

func updateGroundImage(ground *ebiten.Image) {
	ground.Clear()

	x16, y16 := thePlayer.Position()
	a := thePlayer.Angle()
	gw, gh := ground.Size()
	w, h := gophersImage.Size()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(-x16)/16, float64(-y16)/16)
	op.GeoM.Translate(float64(-w*2), float64(-h*2))
	op.GeoM.Rotate(float64(-a)*2*math.Pi/maxAngle + math.Pi*3.0/2.0)
	op.GeoM.Translate(float64(gw)/2, float64(gh)-32)
	ground.DrawImage(repeatedGophersImage, op)
}

func drawGroundImage(screen *ebiten.Image, ground *ebiten.Image) {
	perspectiveGroundImage.Clear()
	gw, _ := ground.Size()
	pw, ph := perspectiveGroundImage.Size()
	for j := 0; j < ph; j++ {
		// z is in [1.5, 0.01]
		rate := float64(j) / float64(ph)
		z := (1-rate)*1.5 + rate*0.01
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(pw)/2, 0)
		op.GeoM.Scale(1/z, 4) // 4 is an arbitrary number not to make empty lines.
		op.GeoM.Translate(float64(pw)/2, float64(j)/z)

		src := image.Rect(0, j, gw, j+1)
		op.SourceRect = &src
		perspectiveGroundImage.DrawImage(ground, op)
	}

	perspectiveGroundImage.DrawImage(fogImage, nil)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(pw)/2, 0)
	op.GeoM.Rotate(-1 * float64(thePlayer.lean) / maxLean * math.Pi / 8)
	op.GeoM.Translate(float64(screenWidth)/2, screenHeight/3)
	screen.DrawImage(perspectiveGroundImage, op)
}

func update(screen *ebiten.Image) error {
	if ebiten.IsKeyPressed(ebiten.KeySpace) {
		thePlayer.MoveForward()
	}
	rotated := false
	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		thePlayer.RotateRight()
		rotated = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		thePlayer.RotateLeft()
		rotated = true
	}
	if !rotated {
		thePlayer.Stabilize()
	}

	if ebiten.IsRunningSlowly() {
		return nil
	}

	screen.Fill(skyColor)
	updateGroundImage(groundImage)
	drawGroundImage(screen, groundImage)
	tutrial := "Space: Move forward\nLeft/Right: Rotate"
	msg := fmt.Sprintf("FPS: %0.2f\n%s", ebiten.CurrentFPS(), tutrial)
	ebitenutil.DebugPrint(screen, msg)
	return nil
}

func main() {
	if err := ebiten.Run(update, screenWidth, screenHeight, 2, "Air Ship (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}
