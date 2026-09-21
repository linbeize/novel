<body class="x-iframe-body">
	<div class="x-nav">
		<span class="layui-breadcrumb">
		  <a><cite>首页</cite></a>
		</span>
	</div>

	<div class="x-body">
        {{if .Pause.Paused}}
        <!-- 采集因投毒而暂停：显示时间与原因，便于人工确认后恢复 -->
        <div class="layui-card" style="border-left:4px solid #FF5722;background:#FFF8F5;">
            <div class="layui-card-header" style="color:#D84315;font-weight:bold;">
                <i class="layui-icon layui-icon-notice"></i> 采集已自动暂停
            </div>
            <div class="layui-card-body" style="line-height:1.9;">
                <p style="color:#D84315;margin-bottom:8px;">
                    原因：检测到采集站返回伪造内容（标题正常但正文为随机软文），已停止采集以防污染书库。
                </p>
                <table class="layui-table" style="margin:0;">
                    <tbody>
                        <tr>
                            <th width="20%">暂停时间</th>
                            <td><strong>{{.Pause.AtText}}</strong> <span style="color:#999;">（{{.Pause.AgoText}}）</span></td>
                        </tr>
                        <tr>
                            <th>触发来源</th>
                            <td>{{.Pause.Source}}</td>
                        </tr>
                        <tr>
                            <th>判定依据</th>
                            <td>{{.Pause.Reason}}</td>
                        </tr>
                        <tr>
                            <th>内容样本</th>
                            <td style="color:#666;word-break:break-all;">{{.Pause.Sample}}</td>
                        </tr>
                    </tbody>
                </table>
                <p style="margin-top:10px;color:#666;">
                    确认站点已恢复正常后，可到「系统设置」重新开启「自动采集」，或点击
                    <a href="javascript:;" onclick="resumeSnatch()" style="color:#1E9FFF;">此处恢复</a>。
                </p>
            </div>
        </div>
        <script type="text/javascript">
        function resumeSnatch(){
            if(!confirm('确认采集站已恢复正常，并重新开启采集？')) return;
            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/admin/home/resume', true);
            xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            xhr.onreadystatechange = function(){
                if(xhr.readyState !== 4) return;
                try {
                    var r = JSON.parse(xhr.responseText);
                    alert(r.msg || '已恢复');
                    if(r.ret === 0){ location.reload(); }
                } catch(e){ alert('操作失败'); }
            };
            xhr.send('');
        }
        </script>
        {{end}}
        <div class="layui-card">
            <blockquote class="layui-elem-quote">
                欢迎使用{{.aOut.Title}}！<span class="f-14">v{{.aOut.Version}}</span>
            </blockquote>
            <table class="layui-table">
                <thead>
                    <tr>
                        <th colspan="2" scope="col">登录信息</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <th width="30%">登录次数</th>
                        <td><span>{{.LoginVisit}}</span></td>
                    </tr>
                    <tr>
                        <td>上次登录IP</td>
                        <td>{{.LastLoginIp}}</td>
                    </tr>
                    <tr>
                        <td>上次登录时间</td>
                        <td>{{datetime .LastLoginedAt "2006-01-02 15:04"}}</td>
                    </tr>
                </tbody>
            </table>
            <!--
            <fieldset class="layui-elem-field layui-field-title site-title">
              <legend><a name="default">信息统计</a></legend>
            </fieldset>
            <table class="layui-table">
                <thead>
                    <tr>
                        <th>统计</th>
                        <th>资讯库</th>
                        <th>图片库</th>
                        <th>产品库</th>
                        <th>用户</th>
                        <th>管理员</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>总数</td>
                        <td>92</td>
                        <td>9</td>
                        <td>0</td>
                        <td>8</td>
                        <td>20</td>
                    </tr>
                    <tr>
                        <td>今日</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                    </tr>
                    <tr>
                        <td>昨日</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                    </tr>
                    <tr>
                        <td>本周</td>
                        <td>2</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                    </tr>
                    <tr>
                        <td>本月</td>
                        <td>2</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                        <td>0</td>
                    </tr>
                </tbody>
            </table>
            -->
            <fieldset class="layui-elem-field layui-field-title site-title">
              <legend><a name="default">系统信息</a></legend>
            </fieldset>
            <table class="layui-table">
                <thead>
                    <tr>
                        <th colspan="2" scope="col">服务器信息</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <th width="30%">应用名称</th>
                        <td><span id="lbServerName">{{.AppName}}</span></td>
                    </tr>
                    <tr>
                        <td>服务器IP地址</td>
                        <td>{{.Ip}}</td>
                    </tr>
                    <tr>
                        <td>服务器端口 </td>
                        <td>{{.Port}}</td>
                    </tr>
                    <tr>
                        <td>Goos</td>
                        <td>{{.Goos}}</td>
                    </tr>
                    <tr>
                        <td>Go Version</td>
                        <td>{{.Version}}</td>
                    </tr>
                    <tr>
                        <td>服务器当前时间 </td>
                        <td>{{.NowTime}}</td>
                    </tr>
                    <tr>
                        <td>服务器上次启动到现在已运行 </td>
                        <td>{{.UpTime}}分钟</td>
                    </tr>
                    <tr>
                        <td>CPU 总数 </td>
                        <td>{{.Cpus}}</td>
                    </tr>
                    <tr>
                        <td>CPU 类型 </td>
                        <td>{{.CPUModelName}}</td>
                    </tr>
                    <tr>
                        <td>虚拟内存 </td>
                        <td>{{.Memory}}MB</td>
                    </tr>
                    <tr>
                        <td>当前程序占用内存 </td>
                        <td>{{.Mem}}M</td>
                    </tr>
                    <tr>
                        <td>当前Session数量 </td>
                        <td>8</td>
                    </tr>
                    <tr>
                        <td>当前SessionID </td>
                        <td>{{.SessionID}}</td>
                    </tr>
                </tbody>
            </table>
        </div>
    </div>
	<div class="layui-footer footer footer-demo">
		<div class="layui-main">
			<p>
                Copyright ©2017 <a href="https://github.com/vckai/novel" target="_blank">vckai/novel</a> v{{.aOut.Version}} All Rights Reserved.
			</p>
		</div>
	</div>
</body>
