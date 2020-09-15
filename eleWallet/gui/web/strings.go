// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

package webgui

import "strconv"

var (
	g_LangIsEN     = true
	g_Title        = "GCTT Demo"
	g_Subtitle1    = "Wallet"
	g_Subtitle2    = "Miner"
	g_Subtitle3    = "Space provider"
	g_Subtitle4    = "Application for Space"
	g_Subtitle5    = "Show Space"
	g_Subtitle6    = "Write and read"
	g_Subtitle7    = "Status"
	g_CancelBtnCap = `btn.innerHTML = "Cancel";`
	g_FunctionShow = ""
	g_ButtonIDDscp = make(map[string]string)
)

type RPCCommand struct {
	Description string
	Hints       []string
	typeNameIn  []string
	baseOut     int
}

var RPCCommandsCN = map[string]RPCCommand{
	"personal_newAccount":     {"产生新地址", []string{"要设置的密码，示例：1234"}, []string{"string"}, 0},
	"guiback_importKey":       {"导入密钥", []string{"要导入的密钥JSON内容，16进制字符串格式", "密码，示例：1234"}, []string{"string", "string"}, 0},
	"eth_accounts":            {"查看已有地址", []string{}, []string{}, 0},
	"elephant_getBalance":     {"查询CTT币余额", []string{"要查询的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 10},
	"elephant_getSignedCs":    {"获取已签约CS", []string{"要查询的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 0},
	"elephant_signCs":         {"签约CS", []string{"要签约的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string", "string"}, 0},
	"elephant_cancelCs":       {"解约CS", []string{"要解约的地址，示例：0x1234567890abcde1234567890abcde1234567890"}, []string{"string", "string"}, 0},
	"webgui_setPasswd":        {"输入账户密码", []string{"要输入的密码，示例：1234"}, []string{"string"}, 0},
	"blizcs_getRentSize":      {"查看已租用空间", []string{"文件名，示例：2"}, []string{"uint64"}, 0},
	"blizcs_createObject":     {"申请空间对象", []string{"空间分区ID，示例：2", "文件大小，示例：1", "密码，示例：1234"}, []string{"uint64", "uint64", "string"}, 0},
	"blizcs_addObjectStorage": {"扩展对象空间", []string{"空间分区ID，示例：2", "文件大小，示例：1", "密码，示例：1234"}, []string{"uint64", "uint64", "string"}, 0},
	"blizcs_write":            {"向对象写入字符", []string{"空间分区ID，示例：2", "起始位置，示例：0", "待写字符串，示例：abc", "密码，示例：1234"}, []string{"uint64", "uint64", "string", "string"}, 0},
	"blizcs_read":             {"读取对象空间", []string{"空间分区ID，示例：2", "起始位置，示例：0", "待读长度，示例：3", "密码，示例：1234"}, []string{"uint64", "uint64", "uint64", "string"}, 0},
	"blizcs_writeFile":        {"上传文件", []string{"空间分区ID，示例：2", "起始位置，示例：0", "要上传的文件", "密码，示例：1234"}, []string{"uint64", "uint64", "string", "string"}, 0},
	"blizcs_readFile":         {"读空间写入文件", []string{"空间分区ID，示例：2", "读取起始位置，示例：0", "读取长度，示例：3", "保存文件名，示例：file.txt"}, []string{"uint64", "uint64", "uint64", "string"}, 0},
	"blizcs_getObjectsInfo":   {"查看已有对象", []string{"密码，示例：1234"}, []string{"string"}, 0},
}

var RPCCommandsEN = map[string]RPCCommand{
	"personal_newAccount":     {"Create a new address", []string{"To set the password, such as: 1234"}, []string{"string"}, 0},
	"guiback_importKey":       {"Import keystore", []string{"The key string to import", "Password, such as: 1234"}, []string{"string", "string"}, 0},
	"eth_accounts":            {"Check addresses", []string{}, []string{}, 0},
	"elephant_getBalance":     {"Show balance", []string{"Address needed, such as: 0x1234567890abcde1234567890abcde1234567890"}, []string{"string"}, 10},
	"elephant_getSignedCs":    {"Show signed CS", []string{}, []string{}, 0},
	"elephant_signCs":         {"Sign CS", []string{"Address needed, such as: 0x1234567890abcde1234567890abcde1234567890", "Start and end time, format: YYYY-MM-DD hh:mm:ss~YYYY-MM-DD hh:mm:ss", "Flow(unit: Byte)", "Password, such as: 1234"}, []string{"string", "string", "uint64", "string"}, 0},
	"elephant_cancelCs":       {"Cancel CS", []string{"Address needed, such as: 0x1234567890abcde1234567890abcde1234567890", "Password, such as: 1234"}, []string{"string", "string"}, 0},
	"webgui_setPasswd":        {"Enter the password", []string{"To input the password, such as: 1234"}, []string{"string"}, 0},
	"blizcs_getRentSize":      {"Show total space", []string{"Object name, such as: 2"}, []string{"uint64"}, 0},
	"blizcs_createObject":     {"Apply for space", []string{"Object name, such as: 2", "Object size, such as: 1", "Password, such as: 1234"}, []string{"uint64", "uint64", "string"}, 0},
	"blizcs_addObjectStorage": {"Expand space", []string{"Object name, such as: 2", "Object size, such as: 1", "Password, such as: 1234"}, []string{"uint64", "uint64", "string"}, 0},
	"elephant_checkTx":        {"Check tx", []string{"tx hash"}, []string{"string"}, 0},
	"elephant_checkReceipt":   {"Check receipt", []string{"tx hash"}, []string{"string"}, 0},
	"blizcs_write":            {"Write strings", []string{"Object name such as: 2", "Starting place, such as: 0", "Strings needed to write, such as: abc", "To set the password, such as: 1234"}, []string{"uint64", "uint64", "string", "string"}, 0},
	"blizcs_read":             {"Read strings", []string{"Object name such as: 2", "Starting place, such as: 0", "Length of string, such as: 3", "To set the password, such as: 1234"}, []string{"uint64", "uint64", "uint64", "string"}, 0},
	"blizcs_writeFile":        {"Upload a file", []string{"Object name such as: 2", "Starting place, such as: 0", "File needed to upload", "To set the password, such as: 1234"}, []string{"uint64", "uint64", "string", "string"}, 0},
	"blizcs_readFile":         {"Download as a file", []string{"Object name, such as: 2", "Starting place, such as: 0", "Length needed to read, such as: 3", "File name of the contents", "Parameter 5: To set the password, such as: 1234"}, []string{"uint64", "uint64", "uint64", "string", "string"}, 0},
	"blizcs_getObjectsInfo":   {"Show space objects", []string{"Password, such as: 1234"}, []string{"string"}, 0},
	"elephant_walletTx":       {"Wallet Tx", []string{"Tx type: 0-sign time, 1-sign flow", "Tx content: such as to=? passwd=? data=?, if data is clock, subdata needed"}, []string{"uint32", "string"}, 0},
}

func initStrings() {
	if g_LangIsEN {
		g_Title = "GCTT Client for Test"
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
	} else {
		g_Title = "客户端测试界面网页版"
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
	}
	g_ButtonIDDscp["personal_newAccount"] = "id=\"personal_newAccount\" value=\"" + getCmds()["personal_newAccount"].Description
	g_ButtonIDDscp["guiback_importKey"] = "id=\"guiback_importKey\" value=\"" + getCmds()["guiback_importKey"].Description
	g_ButtonIDDscp["eth_accounts"] = "id=\"eth_accounts\" value=\"" + getCmds()["eth_accounts"].Description
	g_ButtonIDDscp["elephant_getBalance"] = "id=\"elephant_getBalance\" value=\"" + getCmds()["elephant_getBalance"].Description
	g_ButtonIDDscp["elephant_getSignedCs"] = "id=\"elephant_getSignedCs\" value=\"" + getCmds()["elephant_getSignedCs"].Description
	g_ButtonIDDscp["elephant_signCs"] = "id=\"elephant_signCs\" value=\"" + getCmds()["elephant_signCs"].Description
	g_ButtonIDDscp["elephant_cancelCs"] = "id=\"elephant_cancelCs\" value=\"" + getCmds()["elephant_cancelCs"].Description
	g_ButtonIDDscp["webgui_setPasswd"] = "id=\"webgui_setPasswd\" value=\"" + getCmds()["webgui_setPasswd"].Description
	g_ButtonIDDscp["elephant_sendLightTx"] = "id=\"elephant_sendLightTx\" value=\"" + getCmds()["elephant_sendLightTx"].Description
	g_ButtonIDDscp["blizcs_getRentSize"] = "id=\"blizcs_getRentSize\" value=\"" + getCmds()["blizcs_getRentSize"].Description
	g_ButtonIDDscp["blizcs_createObject"] = "id=\"blizcs_createObject\" value=\"" + getCmds()["blizcs_createObject"].Description
	g_ButtonIDDscp["blizcs_addObjectStorage"] = "id=\"blizcs_addObjectStorage\" value=\"" + getCmds()["blizcs_addObjectStorage"].Description
	g_ButtonIDDscp["elephant_checkTx"] = "id=\"elephant_checkTx\" value=\"" + getCmds()["elephant_checkTx"].Description
	g_ButtonIDDscp["elephant_checkReceipt"] = "id=\"elephant_checkReceipt\" value=\"" + getCmds()["elephant_checkReceipt"].Description
	g_ButtonIDDscp["elephant_walletTx"] = "id=\"elephant_walletTx\" value=\"" + getCmds()["elephant_walletTx"].Description
	g_ButtonIDDscp["blizcs_write"] = "id=\"blizcs_write\" value=\"" + getCmds()["blizcs_write"].Description
	g_ButtonIDDscp["blizcs_read"] = "id=\"blizcs_read\" value=\"" + getCmds()["blizcs_read"].Description
	g_ButtonIDDscp["blizcs_writeFile"] = "id=\"blizcs_writeFile\" value=\"" + getCmds()["blizcs_writeFile"].Description
	g_ButtonIDDscp["blizcs_readFile"] = "id=\"blizcs_readFile\" value=\"" + getCmds()["blizcs_readFile"].Description
	g_ButtonIDDscp["blizcs_getObjectsInfo"] = "id=\"blizcs_getObjectsInfo\" value=\"" + getCmds()["blizcs_getObjectsInfo"].Description
}

func getCmds() map[string]RPCCommand {
	cmds := RPCCommandsCN
	if g_LangIsEN {
		cmds = RPCCommandsEN
	}
	return cmds
}

func generateHints() string {
	var strParamsHints = "\n\tswitch (btn.id) {\n"
	for k, v := range getCmds() {
		strParamsHints += "\tcase \"" + k + "\":"
		for i, hint := range v.Hints {
			strParamsHints += "\n\t\thints[" + strconv.FormatInt(int64(i), 10) + "] = \"" + hint + "\";"
		}
		strParamsHints += "\n\t\tbreak;\n"
	}
	strParamsHints += "\t}\n\t"
	return strParamsHints
}

func getWebShowString() (string, string) {
	var (
		demoStr1 = `<html>
<head>
<meta charset="utf-8">
<title>Client Test GUI</title>
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
<form method="POST" action="/eleWallet" id="submitDemo" enctype="multipart/form-data">
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
			"\n<input type=\"button\"" + g_ButtonIDDscp["personal_newAccount"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["guiback_importKey"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["eth_accounts"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_getBalance"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_getSignedCs"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_signCs"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_cancelCs"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["webgui_setPasswd"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			// "\n<input type=\"button\"" + g_ButtonIDDscp["elephant_sendLightTx"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			`<br><br>`
		demoStr3 = `<div class="text" style="text-align:center; font-size:25px">` + g_Subtitle4 + `</div><br>` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_createObject"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_addObjectStorage"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_checkTx"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_checkReceipt"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["elephant_walletTx"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<br><br>" + `</td><td align=center>` +
			`<br><div class="text" style="text-align:center; font-size:25px">` + g_Subtitle5 + `</div><br>` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_getObjectsInfo"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_getRentSize"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<br>" +
			`<br><div class="text" style="text-align:center; font-size:25px">` + g_Subtitle6 + `</div><br>` +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_write"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_read"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_writeFile"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>" +
			"\n<input type=\"button\"" + g_ButtonIDDscp["blizcs_readFile"] + "\" style=\"width: 150px; height: 50px;font-size:18px;\" onclick=\"setValue(this)\"/>"
		demoStr4 = `</td></tr></table><br>
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

	return demoStr1 + g_FunctionShow + demoStr2 + demoStr3 + demoStr4, demoStr5
}
