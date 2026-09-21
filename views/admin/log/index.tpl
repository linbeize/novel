<body class="x-iframe-body">
	<div class="x-nav">
		<span class="layui-breadcrumb">
			<a><cite>首页</cite></a>
			<a><cite>系统管理</cite></a>
			<a><cite>运行日志</cite></a>
		</span>
	</div>
	<div class="x-body">
		<div class="layui-card">
			<xblock>
				<div class="layui-inline">
					<label class="layui-form-label" style="width:auto;padding-right:8px;">级别</label>
					<div class="layui-input-inline" style="width:130px;">
						<select id="level" lay-filter="level">
							<option value="ALL" selected>全部</option>
							<option value="DEBUG">DEBUG 及以上</option>
							<option value="INFO">INFO 及以上</option>
							<option value="WARN">WARN 及以上</option>
							<option value="ERROR">仅 ERROR</option>
						</select>
					</div>
				</div>
				<div class="layui-inline">
					<button class="layui-btn layui-btn-primary" id="btn-refresh"><i class="layui-icon">&#xe669;</i>刷新</button>
					<button class="layui-btn layui-btn-normal" id="btn-auto">自动刷新：开</button>
					<button class="layui-btn layui-btn-danger" id="btn-clear">清空</button>
				</div>
				<span class="x-right" style="line-height:40px">
					共 <span id="count">0</span> 条
					<i class="layui-icon layui-icon-help" style="cursor:help;color:#999;"
					   title="此处只保留最近 2000 条日志，重启后清空。完整历史请用 journalctl -u novel 查看。"></i>
				</span>
			</xblock>

			<div id="log-box" style="background:#1e1e1e;color:#d4d4d4;font-family:Consolas,Monaco,'Courier New',monospace;
			     font-size:13px;line-height:1.7;padding:12px;height:640px;overflow:auto;border-radius:4px;">
			</div>
		</div>
	</div>

	<script>
		var lastSeq = 0;          // 已拉取到的最新序号
		var autoTimer = null;     // 自动刷新定时器
		var autoOn = true;

		// 级别配色
		var levelColor = {
			'DEBUG': '#8a8a8a',
			'INFO':  '#4ec9b0',
			'NOTICE':'#569cd6',
			'WARN':  '#dcdcaa',
			'ERROR': '#f48771'
		};

		function esc(s) {
			return String(s == null ? '' : s)
				.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
		}

		function render(entries, reset) {
			var box = document.getElementById('log-box');

			if (reset) {
				box.innerHTML = '';
			}

			if (entries.length === 0 && reset) {
				box.innerHTML = '<div style="color:#777;">（暂无日志）</div>';
				document.getElementById('count').innerText = '0';
				return;
			}

			var html = '';
			for (var i = 0; i < entries.length; i++) {
				var e = entries[i];
				var color = levelColor[e.level] || '#d4d4d4';
				html += '<div style="white-space:pre-wrap;word-break:break-all;">'
					+ '<span style="color:#666;">' + esc(e.time) + '</span> '
					+ '<span style="color:' + color + ';">[' + esc(e.level) + ']</span> '
					+ esc(e.message)
					+ '</div>';
			}

			// 追加模式：直接插入，避免整块重绘导致滚动跳动
			if (reset) {
				box.innerHTML = html;
			} else {
				box.insertAdjacentHTML('beforeend', html);
			}

			document.getElementById('count').innerText = box.children.length;

			// 自动滚到底部（若用户没往上翻）
			var nearBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 80;
			if (reset || nearBottom) {
				box.scrollTop = box.scrollHeight;
			}
		}

		// reset=true 重新拉取全部；false 时只拉增量
		//
		// 注意：本函数内的 $ 依赖调用方传入（后台的 $ 需由 layui.jquery 赋值），
		// 故不使用全局 $.getJSON，而用 XMLHttpRequest，
		// 避免在 layui.use 之外引用未定义的 $。
		function load(reset) {
			var level = document.getElementById('level').value;
			var url = '{{urlfor "admin.LogController.List"}}?level=' + encodeURIComponent(level)
				+ '&limit=500&afterSeq=' + (reset ? 0 : lastSeq);

			var xhr = new XMLHttpRequest();
			xhr.open('GET', url, true);
			xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
			xhr.onreadystatechange = function() {
				if (xhr.readyState !== 4) {
					return;
				}
				if (xhr.status !== 200) {
					return;
				}

				var res;
				try {
					res = JSON.parse(xhr.responseText);
				} catch (e) {
					return;
				}

				if (res.ret !== 0) {
					return;
				}

				var data = res.data || {};
				var entries = data.entries || [];

				if (reset) {
					render(entries, true);
				} else if (entries.length > 0) {
					render(entries, false);
				}

				// 推进游标：即使本次无新日志也要同步，避免重复拉取
				var seq = parseInt(data.seq, 10);
				if (!isNaN(seq) && seq > lastSeq) {
					lastSeq = seq;
				}
			};
			xhr.send();
		}

		layui.use(['form', 'layer'], function() {
			var form = layui.form, layer = layui.layer;
			$ = layui.jquery;

			// 切换级别后重新拉取
			form.on('select(level)', function() {
				lastSeq = 0;
				load(true);
			});

			document.getElementById('btn-refresh').onclick = function(e) {
				e.preventDefault();
				lastSeq = 0;
				load(true);
			};

			document.getElementById('btn-auto').onclick = function(e) {
				e.preventDefault();
				autoOn = !autoOn;
				this.innerText = '自动刷新：' + (autoOn ? '开' : '关');
				if (autoOn) {
					startAuto();
				} else {
					clearInterval(autoTimer);
				}
			};

			document.getElementById('btn-clear').onclick = function(e) {
				e.preventDefault();
				layer.confirm('确定清空当前日志显示吗？（不影响 journalctl 的历史日志）', function(idx) {
					$.post('{{urlfor "admin.LogController.Clear"}}', function() {
						lastSeq = 0;
						load(true);
						layer.close(idx);
					});
				});
			};

			function startAuto() {
				clearInterval(autoTimer);
				// 每 3 秒拉一次增量
				autoTimer = setInterval(function() {
					if (autoOn) {
						load(false);
					}
				}, 3000);
			}

			// 首次加载
			load(true);
			startAuto();
		});
	</script>
</body>
