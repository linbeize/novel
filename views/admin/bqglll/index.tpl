<body class="x-iframe-body">

<style>
html{color:#666}
.set-tip{color:#999;font-size:12px;line-height:1.7;margin-top:4px;}
.set-status{display:flex;gap:24px;flex-wrap:wrap;}
.set-status .cell{min-width:150px;}
.set-status .cell .k{color:#999;font-size:12px;}
.set-status .cell .v{font-size:14px;margin-top:2px;}
</style>

<div class="x-nav">
    <span class="layui-breadcrumb">
      <a><cite>首页</cite></a>
      <a><cite>采集管理</cite></a>
      <a><cite>笔趣阁bqglll 设置</cite></a>
    </span>
</div>

<div class="x-body">

    {{if not .State.Exists}}
    <div class="layui-card" style="border-left:4px solid #FF5722;background:#FFF8F5;">
        <div class="layui-card-body">
            <p style="color:#D84315;">
                未找到 <b>bqglll</b> 采集规则，请先在「采集管理 - 采集规则」中导入该站点规则，
                否则本页设置不会生效。
            </p>
        </div>
    </div>
    {{end}}

    {{if .Poison.Paused}}
    <div class="layui-card" style="border-left:4px solid #FF5722;background:#FFF8F5;">
        <div class="layui-card-header" style="color:#D84315;font-weight:bold;">
            <i class="layui-icon layui-icon-notice"></i> 该源因检测到伪造内容已暂停采集
        </div>
        <div class="layui-card-body" style="line-height:1.9;">
            <table class="layui-table" style="margin:0;">
                <tbody>
                    <tr><th width="18%">暂停时间</th>
                        <td><strong>{{.Poison.AtText}}</strong> <span style="color:#999;">（{{.Poison.AgoText}}）</span></td></tr>
                    <tr><th>触发来源</th><td>{{.Poison.Source}}</td></tr>
                    <tr><th>判定依据</th><td>{{.Poison.Reason}}</td></tr>
                    <tr><th>内容样本</th><td style="color:#666;word-break:break-all;">{{.Poison.Sample}}</td></tr>
                </tbody>
            </table>
            <p style="margin-top:10px;color:#666;">
                系统会按下方「恢复试探间隔」定时探测站点是否恢复，
                连续 3 次拿到正常内容后自动恢复采集。
            </p>
        </div>
    </div>
    {{end}}

    <!-- 运行状态 -->
    <div class="layui-card">
        <div class="layui-card-header">运行状态</div>
        <div class="layui-card-body">
            <div class="set-status">
                <div class="cell">
                    <div class="k">采集规则</div>
                    <div class="v">{{if .State.Exists}}<span style="color:#5FB878;">已配置</span>{{else}}<span style="color:#FF5722;">未配置</span>{{end}}</div>
                </div>
                <div class="cell">
                    <div class="k">启用状态</div>
                    <div class="v">{{if eq (itoa .State.State) "1"}}<span style="color:#5FB878;">已启用</span>{{else}}<span style="color:#999;">已停用</span>{{end}}</div>
                </div>
                <div class="cell">
                    <div class="k">站点地址</div>
                    <div class="v">{{.State.Url}}</div>
                </div>
                <div class="cell">
                    <div class="k">当前状态</div>
                    <div class="v">{{if .Poison.Paused}}<span style="color:#FF5722;">已暂停（投毒）</span>{{else}}<span style="color:#5FB878;">正常</span>{{end}}</div>
                </div>
            </div>
        </div>
    </div>

    <div class="layui-card">
        <div class="layui-card-body">
            <form class="layui-form" action="">

                <!-- 采集节奏 -->
                <fieldset class="layui-elem-field layui-field-title" style="margin-top:6px;">
                    <legend>采集节奏</legend>
                </fieldset>

                {{range $i, $it := .Items}}
                {{if or (eq $it.Key "BqglllInterval") (eq $it.Key "BqglllJitter") (eq $it.Key "BqglllTimeout") (eq $it.Key "BqglllRetry")}}
                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:150px;">{{$it.Label}}</label>
                    <div class="layui-input-inline" style="width:120px;">
                        <input type="text" name="{{$it.Key}}" autocomplete="off" class="layui-input" value="{{$it.Value}}" />
                    </div>
                    <div class="layui-form-mid layui-word-aux" style="max-width:520px;white-space:normal;">
                        {{$it.Tip}}
                    </div>
                </div>
                {{end}}
                {{end}}

                <!-- 内容处理 -->
                <fieldset class="layui-elem-field layui-field-title">
                    <legend>内容处理</legend>
                </fieldset>

                {{range $i, $it := .Items}}
                {{if or (eq $it.Key "BqglllMaxDesc") (eq $it.Key "BqglllDefaultCate")}}
                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:150px;">{{$it.Label}}</label>
                    <div class="layui-input-inline" style="width:120px;">
                        <input type="text" name="{{$it.Key}}" autocomplete="off" class="layui-input" value="{{$it.Value}}" />
                    </div>
                    <div class="layui-form-mid layui-word-aux" style="max-width:520px;white-space:normal;">
                        {{$it.Tip}}
                    </div>
                </div>
                {{end}}
                {{end}}

                <!-- 投毒防护 -->
                <fieldset class="layui-elem-field layui-field-title">
                    <legend>投毒防护</legend>
                </fieldset>

                {{range $i, $it := .Items}}
                {{if eq $it.Key "BqglllPoisonCheck"}}
                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:150px;">{{$it.Label}}</label>
                    <div class="layui-input-inline">
                        <select name="{{$it.Key}}">
                            <option value="1" {{if eq $it.Value "1"}}selected{{end}}>开启</option>
                            <option value="0" {{if eq $it.Value "0"}}selected{{end}}>关闭</option>
                        </select>
                    </div>
                    <div class="layui-form-mid layui-word-aux" style="max-width:520px;white-space:normal;">
                        {{$it.Tip}}
                    </div>
                </div>
                {{end}}
                {{end}}

                {{range $i, $it := .Items}}
                {{if or (eq $it.Key "BqglllPoisonThreshold") (eq $it.Key "BqglllProbeInterval")}}
                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:150px;">{{$it.Label}}</label>
                    <div class="layui-input-inline" style="width:120px;">
                        <input type="text" name="{{$it.Key}}" autocomplete="off" class="layui-input" value="{{$it.Value}}" />
                    </div>
                    <div class="layui-form-mid layui-word-aux" style="max-width:520px;white-space:normal;">
                        {{$it.Tip}}
                    </div>
                </div>
                {{end}}
                {{end}}

                <div class="layui-form-item" style="margin-top:20px;">
                    <label class="layui-form-label" style="width:150px;"></label>
                    <div class="layui-input-block" style="margin-left:180px;">
                        <input type="submit" class="layui-btn" lay-submit="" lay-filter="bqglll" value="保存设置" />
                        <button type="button" class="layui-btn layui-btn-primary" id="btn-reset">恢复默认</button>
                    </div>
                </div>

            </form>
        </div>
    </div>

</div>

<script type="text/javascript">
    var tourl = '/admin/bqglll/index';
    var saveurl = '/admin/bqglll/save';
    var reseturl = '/admin/bqglll/reset';

    layui.use(['form', 'layer'], function () {
        var form = layui.form;
        var layer = layui.layer;

        // 提交保存
        form.on('submit(bqglll)', function (data) {
            var xhr = new XMLHttpRequest();
            xhr.open('POST', saveurl, true);
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');

            var parts = [];
            for (var k in data.field) {
                parts.push(encodeURIComponent(k) + '=' + encodeURIComponent(data.field[k]));
            }

            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                try {
                    var r = JSON.parse(xhr.responseText);
                    layer.msg(r.msg || '操作完成');
                    if (r.ret === 0) { setTimeout(function () { location.reload(); }, 800); }
                } catch (e) { layer.msg('保存失败'); }
            };
            xhr.send(parts.join('&'));

            return false;
        });

        // 恢复默认
        document.getElementById('btn-reset').onclick = function () {
            if (!confirm('确认将本页设置恢复为默认值？')) return;

            var xhr = new XMLHttpRequest();
            xhr.open('POST', reseturl, true);
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                try {
                    var r = JSON.parse(xhr.responseText);
                    layer.msg(r.msg || '操作完成');
                    if (r.ret === 0) { setTimeout(function () { location.reload(); }, 800); }
                } catch (e) { layer.msg('操作失败'); }
            };
            xhr.send('');
        };
    });
</script>

</body>
