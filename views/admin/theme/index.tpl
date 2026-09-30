<body class="x-iframe-body">

<style>
html{color:#666}
.th-tip{background:#f7f8fa;border-left:3px solid #1E9FFF;padding:10px 14px;
        line-height:1.9;font-size:13px;color:#666;border-radius:3px;margin-bottom:14px;}
.th-tip b{color:#333;}
.th-tip .warn{color:#FF5722;}

.th-grid{display:flex;flex-wrap:wrap;gap:14px;}
.th-card{width:260px;border:1px solid #e6e6e6;border-radius:5px;overflow:hidden;
         background:#fff;transition:box-shadow .2s;}
.th-card:hover{box-shadow:0 2px 10px rgba(0,0,0,.08);}
.th-card.inuse{border-color:#5FB878;box-shadow:0 0 0 1px #5FB878 inset;}

.th-card .hd{padding:10px 14px;background:#fafafa;border-bottom:1px solid #eee;
             display:flex;align-items:center;justify-content:space-between;}
.th-card .hd .nm{font-weight:bold;color:#333;font-size:14px;}
.th-card .hd .cur{font-size:12px;color:#5FB878;}
.th-card .bd{padding:12px 14px;font-size:12px;color:#888;line-height:1.9;}
.th-card .bd .row{display:flex;justify-content:space-between;}
.th-card .bd .no{color:#FF5722;}
.th-card .ft{padding:10px 14px;border-top:1px solid #f2f2f2;background:#fcfcfc;}

.th-btn{display:inline-block;padding:5px 12px;font-size:12px;border-radius:3px;
        cursor:pointer;border:1px solid #e6e6e6;background:#fff;color:#666;
        margin-right:6px;transition:all .15s;}
.th-btn:hover{border-color:#1E9FFF;color:#1E9FFF;}
.th-btn.primary{background:#1E9FFF;border-color:#1E9FFF;color:#fff;}
.th-btn.primary:hover{opacity:.9;color:#fff;}
.th-btn.danger:hover{border-color:#FF5722;color:#FF5722;}
.th-btn[disabled]{opacity:.45;cursor:not-allowed;}
</style>

<div class="x-nav">
    <span class="layui-breadcrumb">
      <a><cite>首页</cite></a>
      <a><cite>系统设置</cite></a>
      <a><cite>主题管理</cite></a>
    </span>
</div>

<div class="x-body">

    <div class="th-tip">
        <b>当前使用</b>　PC 端：<b style="color:#1E9FFF;">{{.PCTheme}}</b>　
        移动端：<b style="color:#1E9FFF;">{{.MTheme}}</b><br/>
        <span class="warn">注意</span>：主题的<b>切换</b>立即生效；
        <b>新建或删除</b>主题后模板需重新载入内存，<b>须重启服务</b>才会出现或消失。
    </div>

    <div class="layui-card">
        <div class="layui-card-header">
            可用主题
            <span style="color:#999;font-size:12px;font-weight:normal;margin-left:8px;">
                共 {{.ThemeCount}} 套，其中 {{.CompleteCount}} 套含完整模板（PC + 移动端）
            </span>
        </div>
        <div class="layui-card-body">
            <div class="th-grid">
                {{range $i, $t := .Themes}}
                <div class="th-card {{if or $t.IsPCTheme $t.IsMTheme}}inuse{{end}}">
                    <div class="hd">
                        <span class="nm">{{$t.Name}}</span>
                        {{if or $t.IsPCTheme $t.IsMTheme}}
                        <span class="cur">
                            {{if $t.IsPCTheme}}PC 使用中{{end}}
                            {{if and $t.IsPCTheme $t.IsMTheme}} / {{end}}
                            {{if $t.IsMTheme}}移动端使用中{{end}}
                        </span>
                        {{end}}
                    </div>
                    <div class="bd">
                        <div class="row">
                            <span>PC 模板</span>
                            <span class="{{if not $t.HasPC}}no{{end}}">
                                {{if $t.HasPC}}{{$t.PCFiles}} 个{{else}}缺失{{end}}
                            </span>
                        </div>
                        <div class="row">
                            <span>移动端模板</span>
                            <span class="{{if not $t.HasM}}no{{end}}">
                                {{if $t.HasM}}{{$t.MFiles}} 个{{else}}缺失{{end}}
                            </span>
                        </div>
                    </div>
                    <div class="ft">
                        <span class="th-btn {{if $t.IsPCTheme}}primary{{end}}"
                              {{if or (not $t.HasPC) $t.IsPCTheme}}disabled{{end}}
                              onclick="switchTheme('pc', '{{$t.Name}}')">
                            {{if $t.IsPCTheme}}PC 使用中{{else}}用于 PC{{end}}
                        </span>
                        <span class="th-btn {{if $t.IsMTheme}}primary{{end}}"
                              {{if or (not $t.HasM) $t.IsMTheme}}disabled{{end}}
                              onclick="switchTheme('m', '{{$t.Name}}')">
                            {{if $t.IsMTheme}}移动端使用中{{else}}用于移动端{{end}}
                        </span>
                        {{if ne $t.Name "default"}}
                        <span class="th-btn danger" style="margin-left:6px;"
                              onclick="delTheme('{{$t.Name}}')">删除</span>
                        {{end}}
                    </div>
                </div>
                {{end}}
            </div>
        </div>
    </div>

    <div class="layui-card">
        <div class="layui-card-header">新建主题</div>
        <div class="layui-card-body">
            <form class="layui-form" action="">
                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:130px;">主题名</label>
                    <div class="layui-input-inline" style="width:240px;">
                        <input type="text" id="newName" autocomplete="off" class="layui-input"
                               placeholder="如 mytheme（英文/数字/-/_）" />
                    </div>
                    <div class="layui-form-mid layui-word-aux">
                        只能包含字母、数字、下划线、短横线
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:130px;">复制自</label>
                    <div class="layui-input-inline" style="width:240px;">
                        <select id="newFrom">
                            {{range $i, $t := .Themes}}
                            {{if $t.HasPC}}
                            <option value="{{$t.Name}}">{{$t.Name}}</option>
                            {{end}}
                            {{end}}
                        </select>
                    </div>
                    <div class="layui-form-mid layui-word-aux">
                        会把该主题的全部模板复制一份，并自动改好内部的引用路径
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:130px;"></label>
                    <div class="layui-input-block" style="margin-left:160px;">
                        <button class="layui-btn" id="btn-create">创建主题</button>
                        <span style="color:#999;font-size:12px;margin-left:10px;">
                            创建后需重启服务才能在列表中正常使用
                        </span>
                    </div>
                </div>
            </form>
        </div>
    </div>

    <div class="layui-card">
        <div class="layui-card-header">如何修改样式</div>
        <div class="layui-card-body" style="font-size:13px;line-height:2;color:#666;">
            新建主题后，模板文件位于：<br/>
            <code style="background:#f5f5f5;padding:2px 8px;border-radius:3px;">
                views/home/&lt;主题名&gt;/　　views/m/&lt;主题名&gt;/
            </code>
            <br/>
            修改其中的 <code>.tpl</code> 与引用的 CSS 即可调整外观。
            公共片段在同名的 <code>common/</code> 目录下（页头、页脚、分页等）。<br/>
            <span style="color:#FF5722;">提示</span>：
            模板里形如 <code>{{"{{"}}template "home/主题名/common/header.tpl" .{{"}}"}}</code>
            的引用必须与实际目录名一致，否则前台会报找不到模板。
            用「创建主题」生成的文件已自动处理这一点。
        </div>
    </div>

</div>

<script>
    layui.use(['form', 'layer'], function () {
        var layer = layui.layer;
        var form = layui.form;

        // 切换主题
        window.switchTheme = function (which, name) {
            var side = (which === 'pc') ? 'PC 端' : '移动端';
            layer.confirm('确认把' + side + '切换到主题「' + name + '」？', {
                btn: ['确认', '取消']
            }, function () {
                layer.closeAll('dialog');
                post('/admin/theme/switch', 'which=' + which + '&name=' + encodeURIComponent(name));
            });
        };

        // 删除主题
        window.delTheme = function (name) {
            layer.confirm('确认删除主题「' + name + '」？<br/>' +
                          '<span style="color:#FF5722;">其模板文件将被永久删除。</span>',
                {btn: ['确认删除', '取消']}, function () {
                    layer.closeAll('dialog');
                    post('/admin/theme/delete', 'name=' + encodeURIComponent(name));
                });
        };

        // 创建主题
        document.getElementById('btn-create').onclick = function () {
            var name = document.getElementById('newName').value.trim();
            var from = document.getElementById('newFrom').value;

            if (!name) { layer.msg('请输入主题名'); return; }
            if (!/^[A-Za-z0-9_-]+$/.test(name)) {
                layer.msg('主题名只能包含字母、数字、下划线、短横线');
                return;
            }

            var btn = this;
            btn.disabled = true;

            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/admin/theme/create', true);
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                btn.disabled = false;
                try {
                    var r = JSON.parse(xhr.responseText);
                    if (r.ret !== 0) { layer.alert(r.msg, {icon: 2}); return; }
                    layer.alert(r.msg, {icon: 1}, function () { location.reload(); });
                } catch (e) { layer.alert('操作失败', {icon: 2}); }
            };
            xhr.send('name=' + encodeURIComponent(name) + '&from=' + encodeURIComponent(from));
        };

        function post(url, body) {
            var xhr = new XMLHttpRequest();
            xhr.open('POST', url, true);
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                try {
                    var r = JSON.parse(xhr.responseText);
                    if (r.ret !== 0) { layer.alert(r.msg, {icon: 2}); return; }
                    layer.msg(r.msg || '操作成功', {icon: 1});
                    setTimeout(function () { location.reload(); }, 1200);
                } catch (e) { layer.alert('操作失败', {icon: 2}); }
            };
            xhr.send(body);
        }
    });
</script>

</body>
