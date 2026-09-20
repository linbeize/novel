<script src="{{.aOut.ViewUrl}}js/bookcase.js"></script>
<style>
/*临时书架*/
.wrap{width:1200px;margin:0 auto;zoom:1;overflow:hidden;}
.wrap .bookcase{border:3px solid #88C6E5;overflow:hidden;}
.bookcase h2{padding-left:10px;font-size:14px;height:36px;line-height:36px;background:#E1ECED;border-bottom:1px solid #88C6E5;}
.bookbox {float:left;width:460px;overflow:hidden;border:1px dashed #CCC;margin:10px 10px;position:relative;}
.bookbox .num{position:absolute;top:12px;left:10px;width:22px;line-height:22px;border-radius: 4px;background:#FA744E;display:block;text-align:center;color:#eee;font-weight:bold}
.bookbox .bookinfo{padding-left:30px;}
.bookbox .delbutton{position:absolute;top:15px;right:10px;}
.bookbox .delbutton a{border:1px solid #FF4643;border-radius: 3px;padding:4px 10px;color:#FF4643;}
.bookbox .p10{padding:10px;overflow:hidden;}.bookbox div{color:#888;}
.noshow{display:none;}
.bookbox .bookimg{position:absolute;top:12px;left:10px;margin-right:10px;}
.bookbox .bookimg img{width:120px;height:150px;}
.so_list .bookinfo{padding-left:130px;height:156px;overflow:hidden;}

</style>
<div class="wrap">
	<div class="bookcase">
		<h2>临时书架 - 浏览记录</h2>
		<div class="read_book"></div><script language="javascript">loadbooker();</script>
	</div>
</div>