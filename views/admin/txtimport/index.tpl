<body class="x-iframe-body">

<style>
html{color:#666}
.ti-tip{color:#999;font-size:12px;line-height:1.7;margin-top:4px;}
.ti-drop{border:2px dashed #d2d2d2;border-radius:6px;padding:30px 20px;text-align:center;
         cursor:pointer;transition:all .2s;background:#fafafa;}
.ti-drop:hover,.ti-drop.over{border-color:#1E9FFF;background:#f5fbff;}
.ti-drop .ico{font-size:38px;color:#c2c2c2;line-height:1;}
.ti-drop .t1{margin-top:10px;font-size:14px;color:#333;}
.ti-drop .t2{margin-top:6px;font-size:12px;color:#999;}
.ti-file{display:none;}
.ti-result{margin-top:16px;display:none;}
.ti-prev{margin-top:10px;max-height:300px;overflow:auto;background:#fafafa;
         border:1px solid #eee;border-radius:4px;padding:10px;}
.ti-prev .row{padding:6px 4px;border-bottom:1px dashed #eee;font-size:13px;}
.ti-prev .row:last-child{border-bottom:none;}
.ti-prev .no{color:#999;margin-right:6px;}
.ti-prev .tt{color:#1E9FFF;}
.ti-prev .bd{color:#888;font-size:12px;margin-top:2px;}
</style>

<div class="x-nav">
    <span class="layui-breadcrumb">
      <a><cite>首页</cite></a>
      <a><cite>小说管理</cite></a>
      <a><cite>TXT 导入</cite></a>
    </span>
</div>

<div class="x-body">

    <div class="layui-card">
        <div class="layui-card-header">选择文件</div>
        <div class="layui-card-body">

            <div class="ti-drop" id="drop">
                <div class="ico"><i class="layui-icon layui-icon-upload-drag"></i></div>
                <div class="t1" id="dropText">点击选择 txt 文件，或拖拽到此处</div>
                <div class="t2">支持 UTF-8 / GBK 编码，单个文件不超过 50MB</div>
            </div>
            <input type="file" id="txtfile" class="ti-file" accept=".txt" />

        </div>
    </div>

    <div class="layui-card">
        <div class="layui-card-header">书籍信息</div>
        <div class="layui-card-body">
            <form class="layui-form" action="">

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;">书名 <span style="color:red;">*</span></label>
                    <div class="layui-input-inline" style="width:320px;">
                        <input type="text" id="name" name="name" autocomplete="off"
                               class="layui-input" placeholder="留空则从文件名推断" />
                    </div>
                    <div class="layui-form-mid layui-word-aux">留空会自动从文件名提取</div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;">作者</label>
                    <div class="layui-input-inline" style="width:320px;">
                        <input type="text" id="author" name="author" autocomplete="off"
                               class="layui-input" placeholder="留空则从文件名推断，仍无则记为佚名" />
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;">分类 <span style="color:red;">*</span></label>
                    <div class="layui-input-inline" style="width:200px;">
                        <select id="cate_id" name="cate_id">
                            {{range $i, $c := .Cates}}
                            <option value="{{$c.Id}}" {{if eq $c.Id 13}}selected{{end}}>{{$c.Name}}</option>
                            {{end}}
                        </select>
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;">简介</label>
                    <div class="layui-input-block" style="margin-left:140px;">
                        <textarea id="desc" name="desc" class="layui-textarea"
                                  style="max-width:620px;min-height:70px;"
                                  placeholder="可留空"></textarea>
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;">同名处理</label>
                    <div class="layui-input-block" style="margin-left:140px;">
                        <input type="checkbox" id="overwrite" name="overwrite" lay-skin="primary"
                               title="覆盖已有章节" />
                        <div class="ti-tip" style="margin-left:0;">
                            站内已有同名书时，默认拒绝导入。勾选此项会<b style="color:#FF5722;">先删除该书全部章节再导入</b>，请谨慎操作。
                        </div>
                    </div>
                </div>

                <div class="layui-form-item">
                    <label class="layui-form-label" style="width:110px;"></label>
                    <div class="layui-input-block" style="margin-left:140px;">
                        <button type="button" class="layui-btn" id="btn-parse">解析预览</button>
                        <button type="button" class="layui-btn layui-btn-normal" id="btn-import">确认导入</button>
                    </div>
                </div>

            </form>
        </div>
    </div>

    <!-- 解析结果 -->
    <div class="layui-card ti-result" id="card-result">
        <div class="layui-card-header" id="resultTitle">解析结果</div>
        <div class="layui-card-body">
            <div id="resultBody"></div>
            <div class="ti-prev" id="preview"></div>
        </div>
    </div>

</div>

<script type="text/javascript">
    var parseUrl  = '/admin/txtimport/parse';
    var importUrl = '/admin/txtimport/upload';

    // 选中的文件
    var picked = null;

    layui.use(['form', 'layer'], function () {
        var layer = layui.layer;
        var form = layui.form;

        var drop = document.getElementById('drop');
        var input = document.getElementById('txtfile');

        // 点击选择
        drop.onclick = function () { input.click(); };

        // 拖拽
        drop.ondragover = function (e) { e.preventDefault(); drop.classList.add('over'); };
        drop.ondragleave = function () { drop.classList.remove('over'); };
        drop.ondrop = function (e) {
            e.preventDefault();
            drop.classList.remove('over');
            if (e.dataTransfer.files.length > 0) {
                setFile(e.dataTransfer.files[0]);
            }
        };

        input.onchange = function () {
            if (input.files.length > 0) setFile(input.files[0]);
        };

        function setFile(f) {
            if (!/\.txt$/i.test(f.name)) {
                layer.msg('仅支持 txt 文件');
                return;
            }
            if (f.size > 50 * 1024 * 1024) {
                layer.msg('文件过大，请控制在 50MB 以内');
                return;
            }
            picked = f;
            document.getElementById('dropText').innerHTML =
                '已选择：<b style="color:#1E9FFF;">' + f.name + '</b>' +
                ' <span style="color:#999;">(' + (f.size / 1024 / 1024).toFixed(1) + ' MB)</span>';
        }

        /* ---------- 解析预览 ---------- */
        document.getElementById('btn-parse').onclick = function () {
            if (!picked) { layer.msg('请先选择 txt 文件'); return; }

            var fd = new FormData();
            fd.append('txtfile', picked);

            var load = layer.load(2);
            var xhr = new XMLHttpRequest();
            xhr.open('POST', parseUrl, true);

            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                layer.close(load);

                try {
                    var r = JSON.parse(xhr.responseText);
                    if (r.ret !== 0) { layer.msg(r.msg || '解析失败'); return; }
                    showParse(r.data);
                } catch (e) {
                    layer.msg('解析失败：响应格式异常');
                }
            };
            xhr.send(fd);
        };

        function showParse(d) {
            document.getElementById('resultTitle').innerHTML =
                '解析结果：共 <b style="color:#1E9FFF;">' + d.chapters + '</b> 章，' +
                '约 <b style="color:#1E9FFF;">' + d.text_num + '</b> 字';

            // 自动填书名/作者
            var nameEl = document.getElementById('name');
            var authEl = document.getElementById('author');
            if (!nameEl.value && d.guess_name) nameEl.value = d.guess_name;
            if (!authEl.value && d.guess_auth) authEl.value = d.guess_auth;

            var html = '';
            if (d.chapters === 0) {
                html = '<div style="color:#FF5722;">未识别到章节标题，请确认文件含「第X章」等标题行。</div>';
            } else {
                for (var i = 0; i < d.preview.length; i++) {
                    var p = d.preview[i];
                    html += '<div class="row">' +
                            '<div><span class="no">' + p.no + '.</span>' +
                            '<span class="tt">' + esc(p.title) + '</span>' +
                            '<span class="no" style="margin-left:8px;">' + p.len + ' 字</span></div>' +
                            '<div class="bd">' + esc(p.body) + '…</div>' +
                            '</div>';
                }
                if (d.chapters > d.preview.length) {
                    html += '<div style="color:#999;padding:8px 4px;">…… 其余 ' +
                            (d.chapters - d.preview.length) + ' 章未预览</div>';
                }
            }

            document.getElementById('preview').innerHTML = html;
            document.getElementById('card-result').style.display = 'block';
            layer.msg('解析成功，请确认章节切分是否正确');
        }

        /* ---------- 执行导入 ---------- */
        document.getElementById('btn-import').onclick = function () {
            if (!picked) { layer.msg('请先选择 txt 文件'); return; }

            var name = document.getElementById('name').value.trim();
            if (!name) { layer.msg('请填写书名（或先点解析预览自动填充）'); return; }

            var overwrite = document.getElementById('overwrite').checked ? '1' : '0';
            if (overwrite === '1') {
                if (!confirm('已勾选「覆盖已有章节」：\n若站内存在同名书，其全部章节将被删除后重新导入。\n确认继续？')) {
                    return;
                }
            }

            var fd = new FormData();
            fd.append('txtfile', picked);
            fd.append('name', name);
            fd.append('author', document.getElementById('author').value.trim());
            fd.append('cate_id', document.getElementById('cate_id').value);
            fd.append('desc', document.getElementById('desc').value.trim());
            fd.append('overwrite', overwrite);

            var load = layer.load(3);
            var btn = document.getElementById('btn-import');
            btn.disabled = true;

            var xhr = new XMLHttpRequest();
            xhr.open('POST', importUrl, true);

            // 大文件解析需要时间，放宽超时
            xhr.timeout = 10 * 60 * 1000;

            xhr.onreadystatechange = function () {
                if (xhr.readyState !== 4) return;
                layer.close(load);
                btn.disabled = false;

                try {
                    var r = JSON.parse(xhr.responseText);
                    if (r.ret !== 0) {
                        layer.alert(r.msg || '导入失败', {icon: 2});
                        return;
                    }
                    var d = r.data;
                    document.getElementById('resultTitle').innerHTML = '导入完成';
                    document.getElementById('preview').innerHTML =
                        '<div style="line-height:2;">' +
                        '<b>' + esc(d.novel_name) + '</b>　' + esc(d.author) + '<br/>' +
                        '分类：' + esc(d.cate_name) + '<br/>' +
                        '导入章节：<b style="color:#5FB878;">' + d.chapters + '</b> 章<br/>' +
                        '总字数：' + d.text_num + '<br/>' +
                        '耗时：' + d.duration + '<br/>' +
                        (d.is_new ? '新建书籍记录' : '覆盖已有书籍') +
                        '</div>';
                    document.getElementById('card-result').style.display = 'block';
                    layer.msg('导入成功');
                } catch (e) {
                    layer.alert('导入失败：响应格式异常', {icon: 2});
                }
            };

            xhr.ontimeout = function () {
                layer.close(load);
                btn.disabled = false;
                layer.alert('导入超时。文件较大时可稍后查看后台小说列表确认结果。', {icon: 0});
            };

            xhr.send(fd);
        };
    });

    function esc(s) {
        if (s == null) return '';
        return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
</script>

</body>
