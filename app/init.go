package app

import (
	"strings"
	"time"

	"github.com/astaxie/beego"
	"github.com/beego/i18n"

	"github.com/vckai/novel/app/controllers"
	"github.com/vckai/novel/app/models"
	_ "github.com/vckai/novel/app/routers"
	"github.com/vckai/novel/app/services"
	"github.com/vckai/novel/app/utils"
	"github.com/vckai/novel/app/utils/log"
)

const (
	VERSION = "0.0.6"
)

var (
	RunTime = time.Now()
)

func init() {
	// 初始化db
	models.InitDB()

	// 初始化语言选项
	initLang()

	// 服务初始化配置
	services.Init()

	beego.AddFuncMap("urlfor", controllers.URLFor)

	// 注册模板函数
	utils.RegisterFuncMap()

	// 接入 ORM 日志用于 SQL 统计。
	//
	// 注意：本站配置存于数据库 nov_config 表，须用 services.ConfigService 读取，
	// 而不是 beego.AppConfig（后者只读 conf/*.conf 文件）。
	//
	// 默认关闭的原因：beego 的 orm.Debug 会为每条 SQL 做完整字符串格式化
	// （拼出 SQL 与全部参数），生产环境开启反而拖慢页面。
	// 需要排查「某页查了多少次库」时，在后台把 PerfSQLCount 设为 1 并重启即可。
	if services.ConfigService.String("PerfSQLCount") == "1" {
		utils.EnableSQLCount(true)
		utils.HookORMLog()
	}

	// 设置版本号
	beego.AppConfig.Set("version", VERSION)
}

// 初始化语言选项
func initLang() {
	// Initialized language type list.
	langs := strings.Split(beego.AppConfig.String("lang::types"), "|")

	for _, lang := range langs {
		if err := i18n.SetMessage(lang, "lang/"+"locale_"+lang+".ini"); err != nil {
			log.Error("Fail to set message file: " + err.Error())
			return
		}
	}

	beego.AddFuncMap("i18n", i18n.Tr)
}
