// Copyright 2018 The Ebiten Authors
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

// +build darwin

package metal

// #cgo CFLAGS: -x objective-c
// #cgo LDFLAGS: -framework AppKit
//
// #import <AppKit/AppKit.h>
//
// static void* currentWindow() {
//   return (NSWindow*)[NSApp mainWindow];
// }
import "C"

import (
	"unsafe"

	"github.com/hajimehoshi/ebiten/internal/affine"
	"github.com/hajimehoshi/ebiten/internal/graphics"
	"github.com/hajimehoshi/ebiten/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/internal/graphicsdriver/metal/ca"
	"github.com/hajimehoshi/ebiten/internal/graphicsdriver/metal/mtl"
	"github.com/hajimehoshi/ebiten/internal/graphicsdriver/metal/ns"
	"github.com/hajimehoshi/ebiten/internal/mainthread"
)

const source = `#include <metal_stdlib>

using namespace metal;

struct VertexIn {
  packed_float2 position;
  packed_float4 tex;
  packed_float4 color;
};

struct VertexOut {
  float4 position [[position]];
  float4 tex;
  float4 color;
};

vertex VertexOut VertexShader(
  uint vid [[vertex_id]],
  device VertexIn* vertices [[buffer(0)]],
  constant float2& viewport_size [[buffer(1)]]
) {
  float4x4 projectionMatrix = float4x4(
    float4(2.0 / viewport_size.x, 0, 0, 0),
    float4(0, -2.0 / viewport_size.y, 0, 0),
    float4(0, 0, 1, 0),
    float4(-1, 1, 0, 1)
  );

  VertexIn in = vertices[vid];

  VertexOut out = {
    projectionMatrix * float4(in.position, 0, 1),
    in.tex,
    in.color,
  };

  return out;
}

fragment float4 FragmentShader(VertexOut v [[stage_in]],
                               texture2d<float> texture [[texture(0)]],
                               constant float4x4& color_matrix_body [[buffer(0)]],
                               constant float4& color_matrix_translation [[buffer(1)]],
                               constant uint8_t& clear [[buffer(2)]]) {
  if (clear) {
    return 0;
  }

  constexpr sampler texture_sampler(filter::nearest);
  float4 c = texture.sample(texture_sampler, v.tex.xy);

  // Force to normalize?
  //c = clamp(c, 0, 1);
  //c.rgb = min(c.rgb, c.a);

  if (0 < c.a) {
    c.rgb /= c.a;
  }
  c = (color_matrix_body * c) + color_matrix_translation;
  c *= v.color;
  c = clamp(c, 0, 1);
  c.rgb *= c.a;
  return c;
}
`

type Driver struct {
	device    mtl.Device
	ml        ca.MetalLayer
	screenRPS mtl.RenderPipelineState
	rpss      map[graphics.CompositeMode]mtl.RenderPipelineState
	cq        mtl.CommandQueue

	vb mtl.Buffer
	ib mtl.Buffer

	src *Image
	dst *Image
}

var theDriver Driver

func Get() *Driver {
	return &theDriver
}

func (d *Driver) ensureDrawable() {

}

func (d *Driver) SetVertices(vertices []float32, indices []uint16) {
	mainthread.Run(func() error {
		// TODO: Release d.vb if d.vb is not nil
		// TODO: Reuse buffer?
		d.vb = d.device.MakeBuffer(unsafe.Pointer(&vertices[0]), unsafe.Sizeof(vertices[0])*uintptr(len(vertices)), mtl.ResourceStorageModeManaged)
		d.ib = d.device.MakeBuffer(unsafe.Pointer(&indices[0]), unsafe.Sizeof(indices[0])*uintptr(len(indices)), mtl.ResourceStorageModeManaged)
		return nil
	})
}

func (d *Driver) Flush() {
	mainthread.Run(func() error {
		// TODO: Release them
		// drawable should be updated for images
		return nil
	})
}

func (d *Driver) MaxImageSize() int {
	// TODO
	return 4096
}

func (d *Driver) NewImage(width, height int) (graphicsdriver.Image, error) {
	td := mtl.TextureDescriptor{
		PixelFormat: mtl.PixelFormatRGBA8UNorm,
		Width:       graphics.NextPowerOf2Int(width),
		Height:      graphics.NextPowerOf2Int(height),
		StorageMode: mtl.StorageModeManaged,
		Usage:       mtl.TextureUsageShaderRead | mtl.TextureUsageRenderTarget,
	}
	var t mtl.Texture
	mainthread.Run(func() error {
		t = d.device.MakeTexture(td)

		rpd := mtl.RenderPassDescriptor{}
		rpd.ColorAttachments[0].Texture = t
		rpd.ColorAttachments[0].ClearColor = mtl.ClearColor{}
		// TODO: This doesn't work??
		//rpd.ColorAttachments[0].LoadAction = mtl.LoadActionClear

		cb := d.cq.MakeCommandBuffer()
		rce := cb.MakeRenderCommandEncoder(rpd)
		rce.EndEncoding()

		cb.Commit()
		cb.WaitUntilCompleted()

		return nil
	})
	return &Image{
		driver:  d,
		width:   width,
		height:  height,
		texture: t,
	}, nil
}

func (d *Driver) NewScreenFramebufferImage(width, height int) (graphicsdriver.Image, error) {
	mainthread.Run(func() error {
		d.ml.SetDrawableSize(width, height)
		return nil
	})
	return &Image{
		driver: d,
		width:  width,
		height: height,
		screen: true,
	}, nil
}

func (d *Driver) Reset() error {
	if err := mainthread.Run(func() error {
		// TODO: Release existing rpss
		if d.rpss == nil {
			d.rpss = map[graphics.CompositeMode]mtl.RenderPipelineState{}
		}

		var err error
		d.device, err = mtl.CreateSystemDefaultDevice()
		if err != nil {
			return err
		}

		d.ml = ca.MakeMetalLayer()
		d.ml.SetDevice(d.device)
		// https://developer.apple.com/documentation/quartzcore/cametallayer/1478155-pixelformat
		//
		// The pixel format for a Metal layer must be MTLPixelFormatBGRA8Unorm,
		// MTLPixelFormatBGRA8Unorm_sRGB, MTLPixelFormatRGBA16Float, MTLPixelFormatBGRA10_XR, or
		// MTLPixelFormatBGRA10_XR_sRGB.
		d.ml.SetPixelFormat(mtl.PixelFormatBGRA8UNorm)
		d.ml.SetMaximumDrawableCount(3)
		d.ml.SetDisplaySyncEnabled(true)

		lib, err := d.device.MakeLibrary(source, mtl.CompileOptions{})
		if err != nil {
			return err
		}
		vs, err := lib.MakeFunction("VertexShader")
		if err != nil {
			return err
		}
		fs, err := lib.MakeFunction("FragmentShader")
		if err != nil {
			return err
		}
		rpld := mtl.RenderPipelineDescriptor{
			VertexFunction:   vs,
			FragmentFunction: fs,
		}
		rpld.ColorAttachments[0].PixelFormat = d.ml.PixelFormat()
		rpld.ColorAttachments[0].BlendingEnabled = true
		rpld.ColorAttachments[0].DestinationAlphaBlendFactor = mtl.BlendFactorZero
		rpld.ColorAttachments[0].DestinationRGBBlendFactor = mtl.BlendFactorZero
		rpld.ColorAttachments[0].SourceAlphaBlendFactor = mtl.BlendFactorOne
		rpld.ColorAttachments[0].SourceRGBBlendFactor = mtl.BlendFactorOne
		rps, err := d.device.MakeRenderPipelineState(rpld)
		if err != nil {
			return err
		}
		d.screenRPS = rps

		conv := func(c graphics.Operation) mtl.BlendFactor {
			switch c {
			case graphics.Zero:
				return mtl.BlendFactorZero
			case graphics.One:
				return mtl.BlendFactorOne
			case graphics.SrcAlpha:
				return mtl.BlendFactorSourceAlpha
			case graphics.DstAlpha:
				return mtl.BlendFactorDestinationAlpha
			case graphics.OneMinusSrcAlpha:
				return mtl.BlendFactorOneMinusSourceAlpha
			case graphics.OneMinusDstAlpha:
				return mtl.BlendFactorOneMinusDestinationAlpha
			default:
				panic("not reached")
			}
		}

		for c := graphics.CompositeModeSourceOver; c <= graphics.CompositeModeMax; c++ {
			rpld := mtl.RenderPipelineDescriptor{
				VertexFunction:   vs,
				FragmentFunction: fs,
			}
			rpld.ColorAttachments[0].PixelFormat = mtl.PixelFormatRGBA8UNorm
			rpld.ColorAttachments[0].BlendingEnabled = true

			src, dst := c.Operations()
			rpld.ColorAttachments[0].DestinationAlphaBlendFactor = conv(dst)
			rpld.ColorAttachments[0].DestinationRGBBlendFactor = conv(dst)
			rpld.ColorAttachments[0].SourceAlphaBlendFactor = conv(src)
			rpld.ColorAttachments[0].SourceRGBBlendFactor = conv(src)
			rps, err := d.device.MakeRenderPipelineState(rpld)
			if err != nil {
				return err
			}
			d.rpss[c] = rps
		}

		d.cq = d.device.MakeCommandQueue()
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (d *Driver) Draw(indexLen int, indexOffset int, mode graphics.CompositeMode, colorM *affine.ColorM, filter graphics.Filter) error {
	if err := mainthread.Run(func() error {
		// content view can be changed anytime...
		cocoaWindow := ns.NewWindow(unsafe.Pointer(C.currentWindow()))
		cocoaWindow.ContentView().SetLayer(d.ml)
		cocoaWindow.ContentView().SetWantsLayer(true)

		var drawable ca.MetalDrawable
		if d.dst.screen {
			var err error
			drawable, err = d.ml.NextDrawable()
			if err != nil {
				return err
			}
		}

		rpd := mtl.RenderPassDescriptor{}
		if d.dst.screen {
			rpd.ColorAttachments[0].LoadAction = mtl.LoadActionDontCare
			rpd.ColorAttachments[0].StoreAction = mtl.StoreActionStore
		} else {
			rpd.ColorAttachments[0].LoadAction = mtl.LoadActionLoad
			rpd.ColorAttachments[0].StoreAction = mtl.StoreActionStore
		}
		var t mtl.Texture
		if d.dst.screen {
			t = drawable.Texture()
		} else {
			t = d.dst.texture
		}
		rpd.ColorAttachments[0].Texture = t

		w, h := d.dst.viewportSize()

		cb := d.cq.MakeCommandBuffer()
		rce := cb.MakeRenderCommandEncoder(rpd)
		if d.dst.screen {
			rce.SetRenderPipelineState(d.screenRPS)
		} else {
			rce.SetRenderPipelineState(d.rpss[mode])
		}
		rce.SetViewport(mtl.Viewport{0, 0, float64(w), float64(h), -1, 1})
		rce.SetVertexBuffer(d.vb, 0, 0)
		viewportSize := [...]float32{float32(w), float32(h)}
		rce.SetVertexBytes(unsafe.Pointer(&viewportSize[0]), unsafe.Sizeof(viewportSize), 1)
		esBody, esTranslate := colorM.UnsafeElements()
		rce.SetFragmentBytes(unsafe.Pointer(&esBody[0]), unsafe.Sizeof(esBody[0])*uintptr(len(esBody)), 0)
		rce.SetFragmentBytes(unsafe.Pointer(&esTranslate[0]), unsafe.Sizeof(esTranslate[0])*uintptr(len(esTranslate)), 1)
		clear := byte(0)
		if d.src == nil {
			clear = 1
		}
		rce.SetFragmentBytes(unsafe.Pointer(&clear), 1, 2)
		if d.src != nil {
			rce.SetFragmentTexture(d.src.texture, 0)
		} else {
			rce.SetFragmentTexture(mtl.Texture{}, 0)
		}
		rce.DrawIndexedPrimitives(mtl.PrimitiveTypeTriangle, indexLen, mtl.IndexTypeUInt16, d.ib, indexOffset*2)
		rce.EndEncoding()

		if d.dst.screen {
			cb.PresentDrawable(drawable)
		}

		cb.Commit()
		cb.WaitUntilCompleted()

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (d *Driver) ResetSource() {
	d.src = nil
}

type Image struct {
	driver  *Driver
	width   int
	height  int
	screen  bool
	texture mtl.Texture
}

func (i *Image) viewportSize() (int, int) {
	if i.screen {
		return i.width, i.height
	}
	return graphics.NextPowerOf2Int(i.width), graphics.NextPowerOf2Int(i.height)
}

func (i *Image) Dispose() {
}

func (i *Image) IsInvalidated() bool {
	// TODO: Does Metal cause context lost?
	// https://developer.apple.com/documentation/metal/mtlresource/1515898-setpurgeablestate
	// https://developer.apple.com/documentation/metal/mtldevicenotificationhandler
	return false
}

func (i *Image) Pixels() ([]byte, error) {
	b := make([]byte, 4*i.width*i.height)
	mainthread.Run(func() error {
		i.texture.GetBytes(&b[0], uintptr(4*i.width), mtl.Region{
			Size: mtl.Size{i.width, i.height, 1},
		}, 0)
		return nil
	})
	return b, nil
}

func (i *Image) SetAsDestination() {
	i.driver.dst = i
}

func (i *Image) SetAsSource() {
	i.driver.src = i
}

func (i *Image) ReplacePixels(pixels []byte, x, y, width, height int) {
	mainthread.Run(func() error {
		i.texture.ReplaceRegion(mtl.Region{
			Origin: mtl.Origin{x, y, 0},
			Size:   mtl.Size{width, height, 1},
		}, 0, unsafe.Pointer(&pixels[0]), 4*width)
		return nil
	})
}
