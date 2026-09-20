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

package utils

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	"github.com/astaxie/beego"
)

func RegisterFuncMap() {
	// 注册模板函数
	beego.AddFuncMap("datetime", GetDateFormat)
	beego.AddFuncMap("itoa", IntToString)
	beego.AddFuncMap("substr_no_html", SubstrNoHtml)
	beego.AddFuncMap("add", Add)
	beego.AddFuncMap("css", Css)
	beego.AddFuncMap("num_format", NumFormat)

	// 页面底部性能统计：在模板渲染到该处时输出本次请求耗时
	beego.AddFuncMap("perf", PerfReport)

	// JS 字符串安全输出：用于把书名、章节名等写入 <script> 中的字符串字面量
	beego.AddFuncMap("js", JsQuote)
}

// JsQuote 将字符串安全地输出为 JavaScript 字符串字面量（含首尾引号）
//
// 场景：模板里需要把书名、章节标题等拼进 <script> 的字符串中，
// 例如浏览历史记录 lastread.set('书名', ...)。
// 若书名本身含有单引号（如《's 的秘密》）或换行，直接拼接会截断
// 字符串、破坏整段脚本。
//
// 这里用 json.Marshal 生成转义后的字面量：它会正确处理引号、反斜杠、
// 换行及 < > & 等字符（后者会被转义为 \u003c 等，可避免 </script> 注入）。
// 返回内容自带双引号，因此模板中不要再手动加引号。
func JsQuote(v interface{}) template.JS {
	if v == nil {
		return template.JS(`""`)
	}

	var s string
	switch t := v.(type) {
	case string:
		s = t
	case []byte:
		s = string(t)
	default:
		s = fmt.Sprint(t)
	}

	b, err := json.Marshal(s)
	if err != nil {
		return template.JS(`""`)
	}

	return template.JS(b)
}

// 供模板调用：输出当前请求的性能统计文本
//
// 由于是在模板渲染过程中取值，此时的耗时已包含数据库查询与
// 渲染到该行之前的模版执行时间，最接近「服务端处理耗时」。
func PerfReport() string {
	return CurrentPerf().Report()
}

// 数字转换字符串
func IntToString(in interface{}) string {
	if in == nil || in == 0 {
		return ""
	}

	s := fmt.Sprint(in)

	if s == "0" {
		return ""
	}

	return s
}

// 去除html标签后截取
func SubstrNoHtml(s string, start, length int) string {
	s = beego.Htmlunquote(s)
	s = beego.HTML2str(s)

	s = strings.Replace(s, "\n", "", -1)

	s = beego.Substr(s, start, length)

	return s
}

// 加法计算
func Add(n, m int) int {
	return n + m
}

// 数字格式化计算
func NumFormat(num uint32) string {
	str := fmt.Sprintf("%d", num)
	if num > 10000 {
		n := float64(num) / float64(10000)
		str = fmt.Sprintf("%.2f万", n)
	}

	return str
}

// 转换css输出
func Css(css string) template.CSS {
	return template.CSS(css)
}
