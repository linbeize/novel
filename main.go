// Copyright 2017 Vckai Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"path/filepath"
	"runtime"

	"github.com/astaxie/beego"

	_ "github.com/vckai/novel/app"
)

func main() {
	// 获取当前文件所在目录，推到项目目录（按你的实际结构调整）
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)

	// 假设 views/ 在 main.go 同级目录下
	beego.SetViewsPath(filepath.Join(baseDir, "views"))
	beego.SetStaticPath("/public", filepath.Join(baseDir, "static"))

	// 开启 gzip 压缩。
	//
	// 说明：beego 的 enablegzip 只存在于结构体字段，并不会从 app.conf
	// 读取（该版本未做解析），因此必须在代码里设置开关。
	// 压缩阈值与级别仍可通过 app.conf 的 gzipMinLength / gzipCompressLevel 调整。
	//
	// 这是性能优化里收益最大的一处：页面以 HTML 文本为主，
	// 章节阅读页未压缩时可达数百 KB，开启后通常可降到 1/4 左右。
	beego.BConfig.EnableGzip = true

	//beego.BConfig.Listen.EnableAdmin = true
	//beego.BConfig.Listen.AdminPort = 8091
	beego.Run()
}
