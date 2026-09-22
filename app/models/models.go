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

package models

import (
	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/orm"
	_ "github.com/go-sql-driver/mysql"
)

const (
	// 表前缀
	TABLE_PREFIX = "nov_"
)

type ArgsBase struct {
	Count   bool
	Limit   int
	Offset  int
	OrderBy string
	Fields  []string
	Keyword string
}

var (
	RoleModel       *Role
	AdminModel      *Admin
	GroupModel      *Group
	AdminLogModel   *AdminLog
	NovelModel      *Novel
	NovelLinksModel *NovelLinks
	CateModel       *Cate
	ChapterModel    *Chapter
	FeedbackModel   *Feedback
	BannerModel     *Banner
	ConfigModel     *Config
	SearchModel     *Search
	SearchLogModel  *SearchLog
	SnatchRuleModel *SnatchRule
)

// db初始化操作
func InitDB() {
	// 读取配置
	mysqlUser := beego.AppConfig.String("mysql::user")
	mysqlPass := beego.AppConfig.String("mysql::pass")
	mysqlURLs := beego.AppConfig.String("mysql::urls")
	dbName := beego.AppConfig.String("mysql::dbname")
	charSet := mysqlCharset("mysql::charset")
	dataBase := beego.AppConfig.String("mysql::database")
	driverName := beego.AppConfig.String("mysql::drivername")

	// 设置db连接
	orm.RegisterDriver(dataBase, orm.DRMySQL)
	orm.RegisterDataBase(driverName, dataBase, mysqlUser+":"+mysqlPass+"@tcp("+mysqlURLs+")/"+dbName+"?charset="+charSet)

	// 章节数据库配置
	// 读取配置
	mysqlUser = beego.AppConfig.String("mysql_chapter::user")
	mysqlPass = beego.AppConfig.String("mysql_chapter::pass")
	mysqlURLs = beego.AppConfig.String("mysql_chapter::urls")
	dbName = beego.AppConfig.String("mysql_chapter::dbname")
	charSet = mysqlCharset("mysql_chapter::charset")
	dataBase = beego.AppConfig.String("mysql_chapter::database")
	driverName = beego.AppConfig.String("mysql_chapter::drivername")

	// 设置db连接
	orm.RegisterDriver(dataBase, orm.DRMySQL)
	orm.RegisterDataBase(driverName, dataBase, mysqlUser+":"+mysqlPass+"@tcp("+mysqlURLs+")/"+dbName+"?charset="+charSet)

	// 创建表
	//orm.RunSyncdb(drivername, false, true)

	// 开发模式开启sql调试
	if beego.AppConfig.String("runmode") == "dev" {
		orm.Debug = true
	}

	InitModel()
}

func InitModel() {
	RoleModel = NewRole()
	GroupModel = NewGroup()
	AdminModel = NewAdmin()
	AdminLogModel = NewAdminLog()
	NovelModel = NewNovel()
	NovelLinksModel = NewNovelLinks()
	CateModel = NewCate()
	ChapterModel = NewChapter()
	FeedbackModel = NewFeedback()
	BannerModel = NewBanner()
	ConfigModel = NewConfig()
	SearchModel = NewSearch()
	SearchLogModel = NewSearchLog()
	SnatchRuleModel = NewSnatchRule()
}

// mysqlCharset 取连接字符集，并把 utf8 纠正为 utf8mb4
//
// 背景：配置里写 charset=utf8 时，MySQL 将其视为 utf8mb3（最多 3 字节），
// 而表结构是 utf8mb4。遇到 4 字节字符（如生僻字 𪭢 U+2AB62、emoji）时，
// 驱动会报 "Conversion from collation utf8mb3_general_ci into
// utf8mb4_0900_ai_ci impossible"，导致该章节整条入库失败。
//
// 这类生僻字在正文里并不罕见（实测即有一章因此丢失），而失败是静默的
// ——只在日志里留一行 ERROR，不易察觉。因此这里统一按 utf8mb4 连接，
// 避免依赖配置文件的写法正确。
func mysqlCharset(key string) string {
	return normalizeCharset(beego.AppConfig.String(key))
}

// normalizeCharset 把 utf8 / utf8mb3 / 空值统一为 utf8mb4
//
// 抽成独立函数便于测试。
func normalizeCharset(cs string) string {
	cs = strings.TrimSpace(cs)

	switch strings.ToLower(cs) {
	case "", "utf8", "utf8mb3":
		return "utf8mb4"
	default:
		return cs
	}
}
