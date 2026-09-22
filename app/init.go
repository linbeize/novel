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

	// 控制台/日志文件的输出级别。
	//
	// beego 默认输出到 Debug 级，而采集过程会逐条记录「获取章节 xxx 使用时间」
	// 这类明细，长时间运行时日志量很大（systemd journal 会迅速膨胀）。
	// 故这里默认只在 dev 或显式配置 logLevel=debug 时才输出 Debug；
	// 注意内存缓冲不受影响——后台「运行日志」页面始终能看到 Debug 记录。
	if beego.BConfig.RunMode != "dev" {
		if services.ConfigService.String("LogLevel") != "debug" {
			beego.SetLevel(beego.LevelInformational)
		}
	}

	// 内存缓冲同样受日志级别控制。
	//
	// 上面只关掉了控制台输出；后台「运行日志」页读的是内存缓冲，
	// 若不一并过滤，页面仍会被 DEBUG 明细（多为页面上的 javascript:/
	// 站外链接这类预期情况）刷屏。此处注入判定函数，按 LogLevel 决定。
	log.SetDebugProvider(func() bool {
		return services.ConfigService.String("LogLevel") == "debug"
	})

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
