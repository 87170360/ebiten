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

// +build android ios

package opengl

import (
	"golang.org/x/mobile/gl"
)

func (d *Driver) DoWork(chError <-chan error, chDone <-chan struct{}) error {
	return d.context.doWork(chError, chDone)
}

func (d *Driver) Init() {
	d.context.gl, d.context.worker = gl.NewContext()
}

func (d *Driver) InitWithContext(context gl.Context) {
	d.context.gl = context
}
