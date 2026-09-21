package services

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/astaxie/beego"

	"github.com/vckai/novel/app/models"
)

/*
联网测试的公共初始化

部分测试需要真实的数据库与服务实例（如采集服务依赖 ProxyService、
NovelService 等）。此处在测试进程内完成一次初始化：

  - 切换到项目根目录，使 beego 能读到 conf/app.conf；
  - 载入配置、初始化数据库连接与各服务单例。

注意：这些测试会真实读取数据库（只读为主），请勿在测试中写入业务数据。
*/

var initOnce sync.Once

// 初始化应用依赖，可重复调用（仅首次生效）
func initApp() {
	initOnce.Do(func() {
		if wd, err := os.Getwd(); err == nil && strings.HasSuffix(wd, "/app/services") {
			_ = os.Chdir("../..")
		}
		beego.BConfig.RunMode = "prod"
		beego.LoadAppConfig("ini", "conf/app.conf")
		models.InitDB()
		Init()
	})
}

// 联网测试开关：默认跳过，避免拖慢常规测试与触发站点限流。
// 显式开启：RUN_SNATCH_TEST=1 go test ./app/services/ -run xxx -v
func requireNetwork(t *testing.T) {
	t.Helper()

	initApp()

	if os.Getenv("RUN_SNATCH_TEST") != "1" {
		t.Skip("联网测试默认跳过，设置 RUN_SNATCH_TEST=1 开启")
	}
}
