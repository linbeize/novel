<div class="header-box">
	<div class="header" style="height:78px">
	  <div class="searchbox">
	      <a href="/" class="logo">
	          <img src="{{.aOut.Logo}}" alt="{{.aOut.Title}}" style="max-width: 150px;">
	      </a>
	      <form action="{{urlfor "home.HomeController.Search"}}" method="get" class="searchform" id="searchForm" onsubmit="return checkSearch()">
	          <div class="querybox">
	              <div class="qborder">
	              	<input type="text" value="{{.Search.q}}" name="kw" id="query" class="query" autocomplete="off">
	             </div>
	          </div>
	          <div class="sbtn1"><input type="submit" value="搜索" onmouseout="this.className='btn1'" onmouseup="this.className='sbtn1'" onmousedown="this.className='btnactive'" id="searchBtn"></div>
	          <div class="hotwords">热搜书籍：
                {{range .RecKw}}
	          		<a href="{{urlfor "home.HomeController.Search" "kw" .Kw}}" target="_blank">{{.Kw}}</a>
                {{end}}
	          </div>
	      </form>
	  </div>
		<script type="text/javascript">
		var checkSearch = function() {
			if ($.trim($("#query").val()).length < 1)
				return false;
			else
				return true;
		};
		</script>

		<div class="login">
			<a href="javascript:setHome(this, location.href)">设为首页</a>
			<span class="divider divider1"></span>
			<a href="javascript:addFavorite('{{.aOut.Title}}', location.href)">收藏本站</a>
		</div>
	</div>

	<div class="mnavbox">
	  <ul class="mnav" style="width:1200px">
	      <li {{if eq .aOut.MethodName "HomeController.Index"}}class="cur"{{end}}><a href="/">首页</a></li>
	      {{range .Cates}}
	      <li class="item" id="{{.Id}}"><a href="{{urlfor "home.HomeController.Cate" "id" .Id}}">{{.Name}}</a></li>
            {{end}}
            
	      <li class="item {{if eq .aOut.MethodName "HomeController.History"}}cur{{end}}"><a href="{{urlfor "home.HomeController.History"}}" class="r-tab">历史记录</a></li>
	      <!--<li><a href="#" class="r-tab">排行</a></li>-->

	  </ul>
	</div>
	<script>
	function getQueryString(name) {
    var reg = new RegExp("(^|&)" + name + "=([^&]*)(&|$)", "i");
    var r = decodeURI(window.location.search).substr(1).match(reg);
    if (r != null)
        return unescape(r[2]);
        return null;
    }
    if (getQueryString("id") != null) {
        $(".item").removeClass("cur");
    }
    $("#"+getQueryString("id")).addClass("cur");
    </script>
</div>	
