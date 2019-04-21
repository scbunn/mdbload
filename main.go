// Copyright © 2019 Stephen Bunn
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"github.com/scbunn/mdbload/cmd"
)

var (
	// BuildInfo should get set at build time
	// -ldflags "-X main.VERSION=v1.0.0" \
	// -ldflags "-X main.GIT_SHA=0a000" \
	// -ldflags "-X main.BUILD_DATE=20190101.00000"
	VERSION    = "v0.0.1"
	GIT_SHA    = "00000"
	BUILD_DATE = "19720101.0000"
)

func main() {
	cmd.Execute(VERSION, GIT_SHA, BUILD_DATE)
}
