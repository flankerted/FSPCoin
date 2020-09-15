package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const (
	rpcPort  = "8545"
	httpPort = "8080"
)

type ResErr struct {
	Code    int
	Message string
}

type RespondErr struct {
	JsonRPC string
	Id      string
	Error   ResErr
}

type RPCCommand struct {
	description string
	parameters  []string
	typeNameIn  []string
	baseOut     int
}

var RPCCommandsCN = map[string]RPCCommand{
	"personal_newAccount":     {"产生新地址", []string{"要设置的密码，示例：1234"}, []string{"string"}, 0},
	"eth_accounts":            {"查看已有地址", []string{}, []string{}, 0},
	"elephant_exportKS":       {"导出密钥", []string{"导出的密钥，示例：0x1234567890abcde1234567890abcde1234567890", "要设置的密码，示例：1234"}, []string{"string", "string"}, 0},
	"eleminer_setEtherbase":   {"设置挖矿地址", []string{"要设置的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 0},
	"elephant_getBalance":     {"获取CTT币余额", []string{"要设置的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 10},
	"eleminer_start":          {"开始挖矿", []string{"要用于挖矿的线程数量，示例：1"}, []string{"int"}, 0},
	"eleminer_stop":           {"停止挖矿", []string{}, []string{}, 0},
	"storage_verify":          {"校验空间", []string{"密码，示例：1234（校验通过获得2个CTT）", "空间单元的ID，示例：1"}, []string{"string", "int"}, 0},
	"storage_getChunkIdList":  {"查看已提供空间", []string{}, []string{}, 0},
	"storage_readRawData":     {"校验数据同步", []string{"空间单元的ID，示例：1", "分片ID，示例：1", "起始位置，示例：0", "长度，示例：3"}, []string{"int", "int", "int", "int"}, 0},
	"blizcs_getRentSize":      {"查看已租用空间", []string{"文件名，示例：2"}, []string{"int"}, 0},
	"blizcs_claim":            {"提供空间", []string{}, []string{}, 0},
	"blizcs_createObject":     {"申请空间对象", []string{"文件名，示例：2", "文件大小，示例：1", "密码，示例：1234"}, []string{"int", "int", "string"}, 0},
	"blizcs_addObjectStorage": {"扩展对象空间", []string{"文件名，示例：2", "文件大小，示例：1", "密码，示例：1234"}, []string{"int", "int", "string"}, 0},
	"blizcs_write":            {"向对象写入字符", []string{"文件名，示例：2", "起始位置，示例：0", "待写字符串，示例：abc"}, []string{"int", "int", "string"}, 0},
	"blizcs_read":             {"读取对象空间", []string{"文件名，示例：2", "起始位置，示例：0", "待读长度，示例：3"}, []string{"int", "int", "int"}, 0},
	"blizcs_writeFile":        {"上传文件", []string{"文件名，示例：2", "起始位置，示例：0", "要上传的文件"}, []string{"int", "int", "string"}, 0},
	"blizcs_readFile":         {"读空间写入文件", []string{"文件名，示例：2", "读取起始位置，示例：0", "读取长度，示例：3", "保存文件名，示例：file.txt"}, []string{"int", "int", "int", "string"}, 0},
	"blizcs_getObjectsInfo":   {"查看已有对象", []string{"密码，示例：1234"}, []string{"string"}, 0},
}

var RPCCommandsEN = map[string]RPCCommand{
	"personal_newAccount":     {"Create a new address", []string{"To set the password, such as: 1234"}, []string{"string"}, 0},
	"eth_accounts":            {"Check addresses", []string{}, []string{}, 0},
	"elephant_exportKS":       {"Export keystore", []string{"The key to export, such as: 0x1234567890abcde1234567890abcde1234567890", "The password, such as: 1234"}, []string{"string", "string"}, 0},
	"eleminer_setEtherbase":   {"Set mining address", []string{"Address needed, such as: 0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 0},
	"elephant_getBalance":     {"Show balance", []string{"Address needed, such as: 0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 10},
	"eleminer_start":          {"Start mining", []string{"Number of threads used for mining, such as: 1"}, []string{"int"}, 0},
	"eleminer_stop":           {"Stop mining", []string{}, []string{}, 0},
	"storage_verify":          {"Verify space", []string{"Password, such as: 1234", "ID of space unit, such as: 1"}, []string{"string", "int"}, 0},
	"storage_getChunkIdList":  {"Show space provided", []string{}, []string{}, 0},
	"storage_readRawData":     {"Verify existing data", []string{"ID of space unit, such as: 1", "ID of space shard, such as: 1", "Starting place, such as: 0", "Length, such as: 3"}, []string{"int", "int", "int", "int"}, 0},
	"blizcs_getRentSize":      {"Show total space", []string{"Object name, such as: 2"}, []string{"int"}, 0},
	"blizcs_claim":            {"Provide space", []string{}, []string{}, 0},
	"blizcs_createObject":     {"Apply for space", []string{"Object name, such as: 2", "Object size, such as: 1", "Password, such as: 1234"}, []string{"int", "int", "string"}, 0},
	"blizcs_addObjectStorage": {"Expand space", []string{"Object name, such as: 2", "Object size, such as: 1", "Password, such as: 1234"}, []string{"int", "int", "string"}, 0},
	"blizcs_write":            {"Write strings", []string{"Object name such as: 2", "Starting place, such as: 0", "Strings needed to write, such as: abc"}, []string{"int", "int", "string"}, 0},
	"blizcs_read":             {"Read strings", []string{"Object name such as: 2", "Starting place, such as: 0", "Length of string, such as: 3"}, []string{"int", "int", "int"}, 0},
	"blizcs_writeFile":        {"Upload a file", []string{"Object name such as: 2", "Starting place, such as: 0", "File needed to upload"}, []string{"int", "int", "string"}, 0},
	"blizcs_readFile":         {"Download as a file", []string{"Object name, such as: 2", "Starting place, such as: 0", "Length needed to read, such as: 3", "File name of the contents, suhc as: file.txt"}, []string{"int", "int", "int", "string"}, 0},
	"blizcs_getObjectsInfo":   {"Show space objects", []string{"Password, such as: 1234"}, []string{"string"}, 0},
}

var (
	g_Result       string
	g_Accounts     []string
	g_DownloadFile string

	g_Title        string
	g_Subtitle1    string
	g_Subtitle2    string
	g_Subtitle3    string
	g_Subtitle4    string
	g_Subtitle5    string
	g_Subtitle6    string
	g_Subtitle7    string
	g_FunctionShow string
	g_CancelBtnCap string

	g_ButtonIDDscp = make(map[string]string)
	g_RPCCommands  map[string]RPCCommand

	g_Lang = flag.String("lang", "cn", "Select the language you want")
)

func init() {
	flag.Parse()
	if *g_Lang == "en" {
		g_Title = "GCTT Demo"
		g_Subtitle1 = "Wallet"
		g_Subtitle2 = "Miner"
		g_Subtitle3 = "Space provider"
		g_Subtitle4 = "Application for Space"
		g_Subtitle5 = "Show Space"
		g_Subtitle6 = "Write and read"
		g_Subtitle7 = "Status"
		g_CancelBtnCap = `btn.innerHTML = "Cancel";`
		g_FunctionShow =
			`function show(hintsArray) {
	if  (isArray(hintsArray)) {
		var lengthHints = hintsArray.length;
		var str = "<br><div><div style='border:dotted 1px blue'><table cellspacing='2'>";
		if (lengthHints == 0) {
			document.getElementById("submitDemo").submit();
			return;
		} else {
			str += "<tr><td>" + "Parameter 1: " + hintsArray[0] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter1' value=''/></td>";
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "Parameter 2: " + hintsArray[1] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter2' value=''/></td>";
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "Parameter 3: " + hintsArray[2] + "</td>";
			if (hintsArray[2] == "File needed to upload") {
				str += "<tr><td>" + "<input value='Select file' type='button' id='selectFile' onclick='upload()' /></td>";
				str += "<tr><td>" + "<input type='text' style='width: 400px;' id='fileUploaded3' readonly='true' value=''/></td>";
				str += "<tr><td>" + "<input type='text' style='width: 400px; display:none;' id='parameter3' value=''/></td>";
			} else {
				str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter3' value=''/></td>";
			}
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "Parameter 4: " + hintsArray[3] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter4' value=''/></td>";
		}
		str += "</tr></table></div></div>";
		showWindow('Operate Parameters', str, 800, 300, true, ['Confirm', funOK]);
	}
}`

		g_RPCCommands = RPCCommandsEN
	} else {
		g_Title = "测试Demo"
		g_Subtitle1 = "钱包操作"
		g_Subtitle2 = "矿工相关操作"
		g_Subtitle3 = "硬盘提供者相关操作"
		g_Subtitle4 = "空间对象管理"
		g_Subtitle5 = "空间对象查看"
		g_Subtitle6 = "空间对象读写"
		g_Subtitle7 = "状态及返回信息"
		g_CancelBtnCap = `btn.innerHTML = "取消";`
		g_FunctionShow =
			`function show(hintsArray) {
	if  (isArray(hintsArray)) {
		var lengthHints = hintsArray.length;
		var str = "<br><div><div style='border:dotted 1px blue'><table cellspacing='2'>";
		if (lengthHints == 0) {
			document.getElementById("submitDemo").submit();
			return;
		} else {
			str += "<tr><td>" + "参数一：" + hintsArray[0] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter1' value=''/></td>";
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "参数二：" + hintsArray[1] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter2' value=''/></td>";
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "参数三：" + hintsArray[2] + "</td>";
			if (hintsArray[2] == "要上传的文件") {
				str += "<tr><td>" + "<input value='选择文件' type='button' id='selectFile' onclick='upload()' /></td>";
				str += "<tr><td>" + "<input type='text' style='width: 400px;' id='fileUploaded3' readonly='true' value=''/></td>";
				str += "<tr><td>" + "<input type='text' style='width: 400px; display:none;' id='parameter3' value=''/></td>";
			} else {
				str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter3' value=''/></td>";
			}
			lengthHints -= 1;
		}
		if (lengthHints > 0) {
			str += "<tr><td>" + "参数四：" + hintsArray[3] + "</td>";
			str += "<tr><td>" + "<input type='text' style='width: 400px;' id='parameter4' value=''/></td>";
		}
		str += "</tr></table></div></div>";
		showWindow('操作相关参数', str, 800, 300, true, ['确定', funOK]);
	}
}`

		g_RPCCommands = RPCCommandsCN
	}

	g_ButtonIDDscp["personal_newAccount"] = "id=\"personal_newAccount\" value=\"" + g_RPCCommands["personal_newAccount"].description
	g_ButtonIDDscp["eth_accounts"] = "id=\"eth_accounts\" value=\"" + g_RPCCommands["eth_accounts"].description
	g_ButtonIDDscp["elephant_exportKS"] = "id=\"elephant_exportKS\" value=\"" + g_RPCCommands["elephant_exportKS"].description
	g_ButtonIDDscp["eleminer_setEtherbase"] = "id=\"eleminer_setEtherbase\" value=\"" + g_RPCCommands["eleminer_setEtherbase"].description
	g_ButtonIDDscp["elephant_getBalance"] = "id=\"elephant_getBalance\" value=\"" + g_RPCCommands["elephant_getBalance"].description
	g_ButtonIDDscp["eleminer_start"] = "id=\"eleminer_start\" value=\"" + g_RPCCommands["eleminer_start"].description
	g_ButtonIDDscp["eleminer_stop"] = "id=\"eleminer_stop\" value=\"" + g_RPCCommands["eleminer_stop"].description
	g_ButtonIDDscp["storage_verify"] = "id=\"storage_verify\" value=\"" + g_RPCCommands["storage_verify"].description
	g_ButtonIDDscp["storage_getChunkIdList"] = "id=\"storage_getChunkIdList\" value=\"" + g_RPCCommands["storage_getChunkIdList"].description
	g_ButtonIDDscp["storage_readRawData"] = "id=\"storage_readRawData\" value=\"" + g_RPCCommands["storage_readRawData"].description
	g_ButtonIDDscp["blizcs_getRentSize"] = "id=\"blizcs_getRentSize\" value=\"" + g_RPCCommands["blizcs_getRentSize"].description
	g_ButtonIDDscp["blizcs_claim"] = "id=\"blizcs_claim\" value=\"" + g_RPCCommands["blizcs_claim"].description
	g_ButtonIDDscp["blizcs_createObject"] = "id=\"blizcs_createObject\" value=\"" + g_RPCCommands["blizcs_createObject"].description
	g_ButtonIDDscp["blizcs_addObjectStorage"] = "id=\"blizcs_addObjectStorage\" value=\"" + g_RPCCommands["blizcs_addObjectStorage"].description
	g_ButtonIDDscp["blizcs_write"] = "id=\"blizcs_write\" value=\"" + g_RPCCommands["blizcs_write"].description
	g_ButtonIDDscp["blizcs_read"] = "id=\"blizcs_read\" value=\"" + g_RPCCommands["blizcs_read"].description
	g_ButtonIDDscp["blizcs_writeFile"] = "id=\"blizcs_writeFile\" value=\"" + g_RPCCommands["blizcs_writeFile"].description
	g_ButtonIDDscp["blizcs_readFile"] = "id=\"blizcs_readFile\" value=\"" + g_RPCCommands["blizcs_readFile"].description
	g_ButtonIDDscp["blizcs_getObjectsInfo"] = "id=\"blizcs_getObjectsInfo\" value=\"" + g_RPCCommands["blizcs_getObjectsInfo"].description
}

func generateHints() string {
	var strParamsHints = "\n\tswitch (btn.id) {\n"
	for k, v := range g_RPCCommands {
		strParamsHints += "\tcase \"" + k + "\":"
		for i, hint := range v.parameters {
			strParamsHints += "\n\t\thints[" + strconv.FormatInt(int64(i), 10) + "] = \"" + hint + "\";"
		}
		strParamsHints += "\n\t\tbreak;\n"
	}
	strParamsHints += "\t}\n\t"
	return strParamsHints
}

func getResult() string {
	return g_Result
}

func demoHandler(w http.ResponseWriter, r *http.Request) {
	var (
		demoStr1 = `<html>
<head>
<meta charset="utf-8">
<title>Demo</title>
<script type="text/javascript">
function setValue(btn) {
	document.getElementById("typeId").value = btn.id;
	var hints = new Array();` + generateHints() +
			`show(hints);
}
//判断是否为数组
function isArray(v) {
	return v && typeof v.length == 'number' && typeof v.splice == 'function';
}
//创建元素
function createEle(tagName) {
	return document.createElement(tagName);
}
//在body中添加子元素
function appChild(eleName) {
	return document.body.appendChild(eleName);
}
//从body中移除子元素
function remChild(eleName) {
	return document.body.removeChild(eleName);
}	
//弹出窗口标题、html、宽度、高度、是否为模式对话框(true,false)，以及
//按钮（关闭按钮为默认，格式为['按钮1',fun1,'按钮2',fun2]数组形式，前面为按钮值，后面为按钮onclick事件）
function showWindow(title, html, width, height, modal, buttons) {
	//避免窗体过小
	if (width < 300) {
    	width = 300;
	}
	if (height < 200) {
    	height = 200;
	}
	//声明mask的宽度和高度（也即整个屏幕的宽度和高度）
	var w, h;
	//可见区域宽度和高度
	var cw = document.body.clientWidth;
	var ch = document.body.clientHeight;
	//正文的宽度和高度 
	var sw = document.body.scrollWidth;
	var sh = document.body.scrollHeight;
	//可见区域顶部距离body顶部和左边距离
	var st = document.body.scrollTop;
	var sl = document.body.scrollLeft;
	w = cw > sw ? cw: sw;
	h = ch > sh ? ch: sh;
	//避免窗体过大
	if (width > w) {
		width = w;
	}
	if (height > h) {
		height = h;
	}
	//如果modal为true，即模式对话框的话，就要创建一透明的掩膜
	if (modal) {
		var mask = createEle('div');
		mask.style.cssText = "position:absolute;left:0;top:0px;background:#fff;filter:Alpha(Opacity=30);opacity:0.5;z-index:100;width:" + w + "px;height:" + h + "px;";
		appChild(mask);
	}
	//这是主窗体
	var win = createEle('div');
	win.style.cssText = "position:absolute;left:" + (sl + cw / 2 - width / 2) + "px;top:" + (st + ch / 2 - height / 2) + "px;background:#f0f0f0;z-index:101;width:" + width + "px;height:" + height + "px;border:solid 2px #afccfe;";
	//标题栏
	var tBar = createEle('div');
	//afccfe,dce8ff,2b2f79
	tBar.style.cssText = "margin:0;width:" + width + "px;height:25px;background:url(top-bottom.png);cursor:move;";
	//标题栏中的文字部分
	var titleCon = createEle('div');
	titleCon.style.cssText = "float:left;margin:3px;";
	titleCon.innerHTML = title; //firefox不支持innerText，所以这里用innerHTML
	tBar.appendChild(titleCon);
	//标题栏中的“关闭按钮”
	var closeCon = createEle('div');
	closeCon.style.cssText = "float:right;width:20px;margin:3px;cursor:pointer;"; //cursor:hand在firefox中不可用
	closeCon.innerHTML = "×";
	tBar.appendChild(closeCon);
	win.appendChild(tBar);
	//窗体的内容部分，CSS中的overflow使得当内容大小超过此范围时，会出现滚动条
	var htmlCon = createEle('div');
	htmlCon.style.cssText = "text-align:center;width:" + width + "px;height:" + (height - 50) + "px;overflow:auto;";
	htmlCon.innerHTML = html;
	win.appendChild(htmlCon);
	//窗体底部的按钮部分
	var btnCon = createEle('div');
	btnCon.style.cssText = "width:" + width + "px;height:25px;text-height:20px;background:url(top-bottom.png);text-align:center;";
	//如果参数buttons为数组的话，就会创建自定义按钮
	if (isArray(buttons)) {
		var length = buttons.length;
		if (length > 0) {
			if (length % 2 == 0) {
				for (var i = 0; i < length; i = i + 2) {
					//按钮数组
					var btn = createEle('button');
					btn.innerHTML = buttons[i]; //firefox不支持value属性，所以这里用innerHTML
					btn.onclick = buttons[i + 1];
					btnCon.appendChild(btn);
					//用户填充按钮之间的空白
					var nbsp = createEle('label');
					nbsp.innerHTML = "&nbsp&nbsp";
					btnCon.appendChild(nbsp);
				}
			}
		}
	}
	//这是默认的取消按钮
	var btn = createEle('button');
` + g_CancelBtnCap + `
	btnCon.appendChild(btn);
	win.appendChild(btnCon);
	appChild(win);
	//获取浏览器事件的方法，兼容ie和firefox
	function getEvent() {
		return window.event || arguments.callee.caller.arguments[0];
	}
	//顶部的标题栏和底部的按钮栏中的“关闭按钮”的关闭事件
	btn.onclick = closeCon.onclick = function() {
		if (document.getElementById("selectFile") != null) {
			document.getElementById("selectFile").disabled = false;
		}
		remChild(win);
		if (mask) {
			remChild(mask);
		}
		document.getElementById("typeId").value = "";
		document.getElementById("submitDemo").submit();
	}
}
`
		demoStr2 = `
function upload() {
	document.getElementById("fileUpload").click();
}
function fileSelected() {
	var obj = document.getElementById("fileUpload");
	var len = obj.files.length;
	for (var i = 0; i < len; i++) {
		document.getElementById("fileUploaded3").value = obj.files[i].name;
		document.getElementById("parameter3").value = obj.files[i].name;
	}
	if (document.getElementById("selectFile") != null) {
		document.getElementById("selectFile").disabled = true;
	}
}
function setParams() {
    if (document.getElementById("parameter1") != null) {
		document.getElementById("param1Id").value = document.getElementById("parameter1").value
	}
	if (document.getElementById("parameter2") != null) {
		document.getElementById("param2Id").value = document.getElementById("parameter2").value
	}
	if (document.getElementById("parameter3") != null) {
		document.getElementById("param3Id").value = document.getElementById("parameter3").value
	}
	if (document.getElementById("parameter4") != null) {
		document.getElementById("param4Id").value = document.getElementById("parameter4").value
	}
}
function funOK() {
    setParams();
	if (document.getElementById("selectFile") != null) {
		document.getElementById("selectFile").disabled = false;
	}
	document.getElementById("submitDemo").submit();
}
window.onload = function () {
	var e = document.getElementById("resultArea");
	e.scrollTop = e.scrollHeight;
}
</script>
</head>
<body>
<div class="text" style="text-align:center; font-size:30px">` + g_Title + `</div>
<div class="text" style="text-align:center;">
<form method="POST" action="/demo" id="submitDemo" enctype="multipart/form-data">
<input type="hidden" id="typeId" name="type" value=" "/>
<input type="hidden" name="param1" id="param1Id" value=""/>
<input type="hidden" name="param2" id="param2Id" value=""/>
<input type="hidden" name="param3" id="param3Id" value=""/>
<input type="hidden" name="param4" id="param4Id" value=""/>
<input name="fileToUpload" id="fileUpload" onchange="fileSelected();" type="file" style="display:none;"/>
<br>
<br><table border="1" align=center cellpadding="8"><tr><td align=center>
<br>
<br>
<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle1 + `</div>
<br>
<div class="text" style="text-align:center;">` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["personal_newAccount"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["eth_accounts"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_exportKS"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_getBalance"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			`<br>
<br>
<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle2 + `</div>
<br>
<div class="text" style="text-align:center;">` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["eleminer_setEtherbase"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["eleminer_start"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["eleminer_stop"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			`<br>
<br>
<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle3 + `</div>
<br>` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_claim"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["storage_getChunkIdList"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["storage_verify"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>"
		demoStr3 = `<br>
<br>
<br></td>

`
		// `<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle4 + `</div><br>` +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_createObject"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_addObjectStorage"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<br>" +
		// `<br><div class="text" style="text-align:center; font-size:25px">` + g_Subtitle5 + `</div><br>` +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_getObjectsInfo"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_getRentSize"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["storage_readRawData"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<br>" +
		// `<br><div class="text" style="text-align:center; font-size:25px">` + g_Subtitle6 + `</div><br>` +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_write"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_read"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_writeFile"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
		// "\n<input type=\"button\"" + g_Butt onIDDscp["blizcs_readFile"] + "\" style=\"width: 180px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>"
		demoStr4 = ` </tr></table><br>
<br>
<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle7 + `</div>
<br>
<textarea id="resultArea" style="width: 600px; height: 200px; font-size:18px" readonly="true">`
		demoStr5 = `</textarea>
</form>
</div>
</body>
</html>`
	)
	if r.Method == "GET" {
		io.WriteString(w, demoStr1+g_FunctionShow+demoStr2+demoStr3+demoStr4+getResult()+demoStr5)

		//if g_DownloadFile != "" {
		//	w.Header().Set("Content-Disposition", "attachment; filename="+g_DownloadFile)
		//	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		//	if err := transferFile("./"+g_DownloadFile, w); err != nil {
		//		fmt.Println("Error: transfer file error\n")
		//		g_Result = g_Result + "Error:\ntransfer file error\n"
		//	}
		//}
		//g_DownloadFile = ""
	}

	if r.Method == "POST" {
		postType := r.FormValue("type")
		defer http.Redirect(w, r, "/demo", http.StatusFound)
		if postType == "" {
			return
		}
		params := getParams(r)
		post := generatePost(postType, params)
		var jsonStr = []byte(post)
		fmt.Println("request: ", postType)

		// Deal with the file uploaded
		f, h, err := r.FormFile("fileToUpload")
		if postType == "blizcs_writeFile" {
			if err != nil {
				showResult("Error, no file selected", 0, g_RPCCommands[postType].baseOut)
				return
			} else {
				defer f.Close()
				fmt.Println("The file to upload is:", h.Filename)

				filePath := filepath.Join("./", h.Filename)
				t, err := os.Create(filePath)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer t.Close()

				if _, err := io.Copy(t, f); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		if postType == "blizcs_readFile" {
			if len(params) == 4 {
				g_DownloadFile = params[3]
			}
		}

		req, err := http.NewRequest("POST", "http://127.0.0.1:"+rpcPort, bytes.NewBuffer(jsonStr))
		if err != nil {
			fmt.Println(err, "\n")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err, "\n")
			return
		}
		defer resp.Body.Close()

		//fmt.Println("response Status: \n", resp.Status)
		//fmt.Println("response Headers:", resp.Header)
		body, _ := ioutil.ReadAll(resp.Body)
		resBody := string(body)
		//fmt.Println("response Body:\n", string(body))
		var resErr RespondErr
		var respUnmarshal = false
		g_Result = ""
		if err1 := json.Unmarshal([]byte(resBody), &resErr); err1 != nil {
			respUnmarshal = true
		} else {
			if resErr.Error.Code == 0 {
				respUnmarshal = true
			} else {
				printErr(resErr.Error.Message)
				g_Result = g_Result + "Error:\n" + resErr.Error.Message + "\n\n"
			}
		}

		var respBody interface{}
		if respUnmarshal {
			if err2 := json.Unmarshal([]byte(resBody), &respBody); err2 != nil {
				fmt.Println(err2, "\n")
			} else {
				res, _ := parseResp(respBody, postType)
				showResult(res, 0, g_RPCCommands[postType].baseOut)
			}
		}

		if g_DownloadFile != "" {
			w.Header().Set("Content-Disposition", "attachment; filename="+g_DownloadFile)
			w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
			if err := transferFile("./"+g_DownloadFile, w); err != nil {
				fmt.Println("Error: transfer file error\n")
				g_Result = g_Result + "Error:\ntransfer file error\n"
			}
		}
		g_DownloadFile = ""
	}
}

func getParams(r *http.Request) []string {
	var params []string
	strName := []string{"param1", "param2", "param3", "param4"}
	for i := 0; i < len(strName); i += 1 {
		if val := r.FormValue(strName[i]); val != "" {
			params = append(params, r.FormValue(strName[i]))
		} else {
			break
		}
	}

	return params
}

func generatePost(postType string, params []string) string {
	var postRPCparams = ""
	for i, v := range params {
		if len(v) != 0 {
			v2, ok := g_RPCCommands[postType]
			if !ok {
				log.Fatal("RPCCommands", "postType", postType)
				continue
			}
			if len(v2.typeNameIn) <= i {
				log.Fatal("RPCCommands", "len(v2.typeNameIn)", len(v2.typeNameIn), "i", i)
				continue
			}
			quote := ""
			if g_RPCCommands[postType].typeNameIn[i] == "string" {
				quote = "\""
			} else if g_RPCCommands[postType].typeNameIn[i] == "int" {
				quote = ""
			}
			if i != 0 {
				postRPCparams = postRPCparams + "," + quote + v + quote
			} else {
				postRPCparams = postRPCparams + quote + v + quote
			}
		}
	}

	postRPC := make(map[string]string, len(g_RPCCommands))
	for k := range g_RPCCommands {
		postRPC[k] = "{" + "\"jsonrpc\":\"" + "2.0" + "\",\"method\":\"" + k +
			"\",\"params\":[" + postRPCparams + "]" + ",\"id\":\"" + "68" + "\"}"
	}

	return postRPC[postType]
}

func printErr(err string) {
	fmt.Println("Error: ", err, "\n")
}

func parseResp(input interface{}, typeInput string) (interface{}, bool) {
	respStrs := input.(map[string]interface{})

	var result interface{}
	for k, v := range respStrs {
		isRet := false
		if k == "result" {
			result = v
			isRet = true
		}
		switch vv := v.(type) {
		case string:
			if isRet {
				fmt.Println(k, ":", vv)
			}
		case int:
			if isRet {
				fmt.Println(k, ":", vv)
			}
		case []interface{}:
			if isRet {
				fmt.Println(k, ":")
			}
			for i, u := range vv {
				if isRet {
					fmt.Println(i+1, ":", u)
				}
			}
		case bool:
			if isRet {
				fmt.Println(k, ":", vv)
			}
		default:
			fmt.Println(k, ": unknown type/null \n")
			return result, false
		}
	}
	fmt.Println("\n")

	saveResult(result, typeInput)

	return result, true
}

func showResult(ret interface{}, index int, base int) {

	if index > 0 {
		g_Result = g_Result + strconv.FormatInt(int64(index), 10) + " : "
	} else {
		g_Result = g_Result + "\n"
	}
	switch vv := ret.(type) {
	case string:
		if base != 0 {
			val, _ := new(big.Int).SetString(vv, 0)
			balance := val.String()
			const weiPos = 18
			if len(balance) >= weiPos {
				tmp := []byte(balance)
				offset := len(balance) - weiPos
				balance = string(tmp[:offset]) + "." + string(tmp[offset:])
			}
			g_Result = g_Result + balance + "\n"
		} else {
			g_Result = g_Result + vv + "\n"
		}
	case int:
		g_Result = g_Result + strconv.FormatInt(int64(vv), base) + "\n"
	case []interface{}:
		for i, u := range vv {
			showResult(u, i+1, base)
		}
	case bool:
		if vv {
			g_Result = g_Result + "true" + "\n"
		} else {
			g_Result = g_Result + "false" + "\n"
		}
	default:
		g_Result = g_Result + "unknown type/null" + "\n"
	}
	if index == 0 {
		g_Result = g_Result + "\n"
	}
}

func saveResult(ret interface{}, typeInput string) {
	switch typeInput {
	case "personal_newAccount":
		g_Accounts = append(g_Accounts, ret.(string))
	case "eth_accounts":
		val := ret.([]interface{})
		var accounts []string
		for _, u := range val {
			accounts = append(accounts, u.(string))
		}
		g_Accounts = accounts

	default:
		return
	}
}

func transferFile(filePath string, w http.ResponseWriter) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(w, file)

	return nil
}

func init() {
	reqType := "eth_accounts"
	var jsonStr = []byte(generatePost(reqType, []string{}))

	req, err := http.NewRequest("POST", "http://127.0.0.1:"+rpcPort, bytes.NewBuffer(jsonStr))
	if err != nil {
		fmt.Println(err, "\n")
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err, "\n")
		return
	}
	defer resp.Body.Close()

	//fmt.Println("response Status: \n", resp.Status)
	//fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	resBody := string(body)
	//fmt.Println("response Body:\n", string(body))

	var respBody interface{}
	if err2 := json.Unmarshal([]byte(resBody), &respBody); err2 != nil {
		fmt.Println(err2, "\n")
	} else {
		parseResp(respBody, reqType)
	}
}

func main() {
	http.HandleFunc("/demo", demoHandler)
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/files/", http.StripPrefix("/files/", fs))
	err := http.ListenAndServe(":"+httpPort, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}
