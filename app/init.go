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
	// 会话配置兜底
	//
	// beego 默认会话有效期仅 1 小时、且用内存存储（服务重启即全部失效），
	// 后台管理会表现为「每次都得重新登录」。conf/app.conf 中已显式配置，
	// 但该文件不随代码分发（各环境不同），故此处再做一层兜底：
	// 仅当配置文件未设置时才生效，已配置则以配置文件为准。
	applySessionDefaults()

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

// applySessionDefaults 会话配置兜底
//
// 只在配置文件未显式设置时生效，便于不同环境按需覆盖。
func applySessionDefaults() {
	const week = 7 * 24 * 3600

	s := &beego.BConfig.WebConfig.Session

	// 默认 1 小时过短，后台管理场景下频繁掉登录
	if s.SessionGCMaxLifetime <= 3600 {
		s.SessionGCMaxLifetime = week
	}

	// 0 表示浏览器会话级 Cookie，关掉浏览器即失效
	if s.SessionCookieLifeTime <= 0 {
		s.SessionCookieLifeTime = week
	}

	// 内存存储会在服务重启后丢失全部会话；改用文件存储
	if s.SessionProvider == "" || s.SessionProvider == "memory" {
		s.SessionProvider = "file"
		if s.SessionProviderConfig == "" {
			s.SessionProviderConfig = "./data/session"
		}
	}
}
