<link href="{{.mOut.ViewUrl}}css/detail.css?v=aa6bf34e" rel="stylesheet" type="text/css">
<header class="hd-bar">
	<a href="javascript:history.go(-1);" class="search-back" id="historyBack"></a>
	<h1>{{.Title}}</h1>
	<a href="/m/" class="search-home"></a>
</header>
<script src="{{.mOut.ViewUrl}}js/bookcase.js"></script>
<style>
/*临时书架*/
.bookname a {
    color: #252525;
    
}
.wrap .bookcase{overflow:hidden;}
.bookbox {margin:10px 10px 0px;padding:10px;line-height:22px;overflow:hidden;border:1px solid #d7d7d7;border-radius:6px;position:relative;}
.bookbox .num{position:absolute;top:12px;left:10px;width:22px;line-height:22px;border-radius: 4px;background:#FA744E;display:block;text-align:center;color:#eee;font-weight:bold}
.bookbox .bookinfo{padding-left:30px;}
.bookbox .delbutton{position:absolute;top:15px;right:10px;}
.bookbox .delbutton a{border:1px solid #FF4643;border-radius: 3px;padding:4px 10px;color:#FF4643;}
.bookbox div{color:#888;}
.noshow{display:none;}
.bookbox .bookimg{position:absolute;top:12px;left:10px;margin-right:10px;}
.bookbox .bookimg img{width:80px;height:100px;}
.so_list .bookinfo{padding-left:90px;height:106px;overflow:hidden;}
</style>
<div class="wrap">
	<div class="block bookcase">
		<div class="read_book"></div><script language="javascript">loadbooker();</script>
	</div>
</div>