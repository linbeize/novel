<body class="x-iframe-body">

<style>
html{color:#666}
.fs-head{display:flex;align-items:center;gap:12px;flex-wrap:wrap;}
.fs-head .kw{font-size:15px;color:#333;}
.fs-head .kw b{color:#1E9FFF;}
.fs-stat{color:#999;font-size:12px;}
.fs-site{margin-top:14px;border:1px solid #e6e6e6;border-radius:4px;overflow:hidden;}
.fs-site .bar{background:#f7f8fa;padding:9px 14px;border-bottom:1px solid #e6e6e6;
              display:flex;align-items:center;justify-content:space-between;}
.fs-site .bar .nm{font-weight:bold;color:#333;}
.fs-site .bar .nm i{color:#5FB878;margin-right:5px;}
.fs-site .bar .cnt{color:#999;font-size:12px;}
.fs-item{display:flex;gap:14px;padding:14px;border-bottom:1px solid #f2f2f2;}
.fs-item:last-child{border-bottom:none;}
.fs-item .cv{width:80px;flex:0 0 80px;}
.fs-item .cv img{width:80px;height:106px;object-fit:cover;border-radius:3px;
                 background:#f5f5f5;border:1px solid #eee;}
.fs-item .cv .noimg{width:80px;height:106px;border-radius:3px;background:#f5f5f5;
                    border:1px solid #eee;display:flex;align-items:center;justify-content:center;
                    color:#ccc;font-size:12px;}
.fs-item .info{flex:1;min-width:0;}
.fs-item .nm{font-size:15px;color:#1E9FFF;margin-bottom:5px;}
.fs-item .nm a{color:#1E9FFF;}
.fs-item .meta{color:#999;font-size:12px;line-height:1.9;}
.fs-item .meta span{margin-right:14px;}
.fs-item .desc{color:#888;font-size:12px;line-height:1.7;margin-top:5px;
               display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;}
.fs-item .act{padding-top:6px;}
.fs-badge{display:inline-block;padding:1px 7px;border-radius:2px;font-size:12px;
          margin-left:6px;}
.fs-badge.yes{background:#e8f7ee;color:#38b26b;}
.fs-empty{text-align:center;color:#999;padding:50px 0;}
.fs-empty .ico{font-size:44px;color:#ddd;line-height:1;}
.fs-empty .t{margin-top:12px;font-size:14px;}
.fs-empty .s{margin-top:8px;font-size:12px;color:#bbb;line-height:1.9;}
</style>

<div class="x-nav">
    <span class="layui-breadcrumb">
      <a><cite>首页</cite></a>
      <a><cite>小说管理</cite></a>
      <a><cite>搜索采集站</cite></a>
    </span>
</div>

<div class="x-body">

    <div class="layui-card">
        <div class="layui-card-body">
            <form class="layui-form" action="" onsubmit="return false;">
                <div class="layui-form-item" style="margin-bottom:0;">
                    <div class="layui-input-inline" style="width:340px;">
                        <input type="text" id="kw" class="layui-input"
                               placeholder="输入书名或作者，回车搜索" value="{{.Kw}}" autofocus />
                    </div>
                    <button class="layui-btn" id="btn-search">
                        <i class="layui-icon layui-icon-search"></i> 搜索
                    </button>
                    <div class="layui-form-mid layui-word-aux">
                        同时搜索所有已启用的采集站，结果按站点分组
                    </div>
                </div>
            </form>
        </div>
    </div>

    {{if .Kw}}
    <div class="layui-card">
        <div class="layui-card-body">
            <div class="fs-head">
                <span class="kw">搜索「<b>{{.Kw}}</b>」</span>
                <span class="fs-stat">
                    共 {{.NovsCount}} 条结果，来自 {{.SiteCount}} 个站点，耗时 {{.Elapsed}}
                </span>
            </div>
        </div>
    </div>

    {{if gt .NovsCount 0}}
    {{range $gi, $g := .Groups}}
    <div class="fs-site">
        <div class="bar">
            <span class="nm"><i class="layui-icon layui-icon-ok-circle"></i>{{$g.Site}}</span>
            <span class="cnt">{{$g.Count}} 条</span>
        </div>

        {{range $ii, $it := $g.Items}}
        <div class="fs-item">
            <div class="cv">
                {{if $it.Nov.Cover}}
                <img src="{{$it.Nov.Cover}}" onerror="this.style.display='none';this.nextElementSibling.style.display='flex';" />
                <div class="noimg" style="display:none;">无封面</div>
                {{else}}
                <div class="noimg">无封面</div>
                {{end}}
            </div>

            <div class="info">
                <div class="nm">
                    {{$it.Nov.Name}}
                    {{if index $.Existing $it.Nov.Name}}
                    <span class="fs-badge yes">已收录</span>
                    {{end}}
                </div>
                <div class="meta">
                    <span>作者：{{$it.Nov.Author}}</span>
                    {{if $it.Nov.ChapterTitle}}<span>最新：{{$it.Nov.ChapterTitle}}</span>{{end}}
                </div>
                {{if $it.Nov.Desc}}
                <div class="desc">{{$it.Nov.Desc}}</div>
                {{end}}
            </div>

            <div class="act">
                <button class="layui-btn layui-btn-sm"
                        onclick="snatch('{{$it.Url}}', '{{$it.Nov.Name}}', this)">
                    采集
                </button>
                <a href="{{$it.Url}}" target="_blank" class="layui-btn layui-btn-sm layui-btn-primary">
                    查看源站
                </a>
            </div>
        </div>
        {{end}}
    </div>
    {{end}}

    {{else}}
    <div class="layui-card">
        <div class="layui-card-body">
            <div class="fs-empty">
                <div class="ico"><i class="layui-icon layui-icon-face-cry"></i></div>
                <div class="t">没有找到「{{.Kw}}」</div>
                <div class="s">
                    可换用书名关键词重试（如只输入前几个字）<br/>
                    若多个站点都无结果，可能是这些站点的搜索已失效，可到「采集规则」中检查
                </div>
            </div>
        </div>
    </div>
    {{end}}
    {{else}}
    <div class="layui-card">
        <div class="layui-card-body">
            <div class="fs-empty">
                <div class="ico"><i class="layui-icon layui-icon-search"></i></div>
                <div class="t">输入书名或作者开始搜索</div>
                <div class="s">
                    会同时查询所有已启用的采集站，并按站点分组展示<br/>
                    可直接点「采集」把书加入站内，无需到采集规则里配置
                </div>
            </div>
        </div>
    </div>
    {{end}}

</div>

<script>
    layui.use(['layer', 'form'], function () {
        var layer = layui.layer;

        // 搜索
        document.getElementById('btn-search').onclick = doSearch;
        document.getElementById('kw').onkeydown = function (e) {
            if (e.keyCode === 13) doSearch();
        };

        function doSearch() {
            var kw = document.getElementById('kw').value.trim();
            if (!kw) { layer.msg('请输入书名或作者'); return; }

            // 本页是在弹层 iframe 中打开的，用 location.href 跳转即可
            // （只影响 iframe 自身，不会影响后台主页面）
            layer.load(2, {shade: 0.1});
            location.href = '{{urlfor "admin.NovelController.FindSnatchs"}}?kw=' +
                            encodeURIComponent(kw);
        }
    });

    // 采集（添加采集点）
    function snatch(url, name, btn) {
        layui.use(['layer'], function () {
            var layer = layui.layer;

            var idx = layer.confirm('确认采集《' + name + '》？', {
                btn: ['确认', '取消']
            }, function () {
                layer.close(idx);
                btn.disabled = true;
                btn.innerHTML = '采集中…';

                var xhr = new XMLHttpRequest();
                xhr.open('POST', '{{urlfor "admin.NovelController.AddSnatch"}}', true);
                xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
                // AddSnatch 用 IsAjax() 判断走接口还是渲染页面，
                // 其依据是 X-Requested-With 头，必须显式设置
                xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
                xhr.onreadystatechange = function () {
                    if (xhr.readyState !== 4) return;
                    btn.disabled = false;
                    btn.innerHTML = '采集';

                    try {
                        var r = JSON.parse(xhr.responseText);
                        if (r.ret === 0) {
                            layer.msg('已加入采集队列', {icon: 1});
                        } else {
                            layer.alert(r.msg || '采集失败', {icon: 2});
                        }
                    } catch (e) {
                        layer.alert('操作失败：响应格式异常', {icon: 2});
                    }
                };
                xhr.send('url=' + encodeURIComponent(url));
            }, function () {});
        });
    }
</script>

</body>
