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

// +build js

package endian

import (
	"github.com/gopherjs/gopherjs/js"
)

func init() {
	a := js.Global.Get("ArrayBuffer").New(4)
	a8 := js.Global.Get("Uint8Array").New(a)
	a32 := js.Global.Get("Uint32Array").New(a)
	a32.SetIndex(0, 1)
	switch a8.Index(0).Int() {
	case 1:
		isLittleEndian = true
	case 0:
		isLittleEndian = false
	default:
		panic("not reach")
	}
}