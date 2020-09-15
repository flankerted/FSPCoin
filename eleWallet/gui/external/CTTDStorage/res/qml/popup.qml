import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Layouts 1.3
import ctt.GUIString 1.0

Window {
    id: popupWindow
    visible: false
    flags: Qt.FramelessWindowHint
    modality: Qt.ApplicationModal
    title: qsTr("为陌")

    property alias tipText: msg.text
    property alias tipTextYesNo: yesNoMsg.text
    property Item parentItem : Rectangle {}

    width: 500
    height: 200

    Rectangle {
        x: 0
        y: 0
        width: popupWindow.width
        height: popupWindow.height
        border.color: "#999999"

        Rectangle {
            id: titlePopupWindow
            color: "#EFF2F6"
            border.color: "transparent"
            x: 1
            y: 1
            width: msgHintDlg.width - 2
            height: 40 - 1

            Image {
                x: 15
                y: 5
                width: 30
                height: 30
                source: "qrc:/images/main/icon-wemore.png"
                fillMode: Image.PreserveAspectFit
            }

            Text {
                id: popupWindowTitle
                x: 50
                y: 5
                width: 360
                height: 30
                text: qsTr("为陌")
                font.bold: true
                font.italic: false
                style: Text.Sunken
                font.family: "Verdana"
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
            }
        }

        Rectangle {
            id: msgHintDlg
            x: 1
            y: titlePopupWindow.height
            width: popupWindow.width - 2
            height: popupWindow.height - titlePopupWindow.height - 2
            visible: false

            Rectangle {
                x: 30
                y: 10
                width: msgHintDlg.width - 2*x
                height: 90
                border.color: "transparent"
                color: "transparent"
                Text {
                    anchors.fill: parent
                    anchors.centerIn: parent
                    font.family: "Microsoft Yahei"
                    color: "black"
                    text: tipText
                    wrapMode: Text.WordWrap
                    verticalAlignment: Text.AlignTop
                    horizontalAlignment: Text.AlignLeft

                }
            }

            Button {
                id: okBtn
                //anchors.centerIn: parent
                x: popupWindow.width - width - height
                y: parent.height - 20 - height
                width: 80
                height: 30

                background: Rectangle {
                    anchors.centerIn: parent
                    width: 80
                    height: 30
                    //border.width: 2
                    Text {
                        anchors.centerIn: parent
                        //font.family: "Microsoft Yahei"
                        //font.bold: true
                        font.pixelSize: 14
                        text: qsTr("好的")
                        color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
                    }

                    color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                    border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
                }

                onClicked: {
                    msgHintDlg = false
                    popupWindow.hide()
                    if (tipText === guiString.uiHints(GUIString.AgentInvalidHint) ||
                            tipText === guiString.uiHints(GUIString.ConnFailedHint)) {
                        main.showAgentConnWindow()
                    }
                }
            }

            Button {
                id: copyTxIDBtn
                objectName: "copyBtn"
                //anchors.centerIn: parent
                x: popupWindow.width - width - height - width - height
                y: parent.height - 20 - height
                width: 80
                height: 30
                visible: false

                background: Rectangle {
                    anchors.centerIn: parent
                    width: 80
                    height: 30
                    //border.width: 2
                    Text {
                        anchors.centerIn: parent
                        //font.family: "Microsoft Yahei"
                        //font.bold: true
                        font.pixelSize: 14
                        text: qsTr("复制哈希")
                        color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
                    }

                    color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                    border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
                }

                onClicked: main.copyTxIDToClipboard(tipText)
            }
        }

        Rectangle {
            id: msgHintYesNoDlg
            x: 1
            y: titlePopupWindow.height
            width: popupWindow.width - 2
            height: popupWindow.height - titlePopupWindow.height - 2
            visible: false

            Rectangle {
                x: 30
                y: 10
                width: msgHintYesNoDlg.width - 2*x
                height: 90
                border.color: "transparent"
                color: "transparent"
                Text {
                    anchors.fill: parent
                    anchors.centerIn: parent
                    font.family: "Microsoft Yahei"
                    color: "black"
                    text: tipTextYesNo
                    wrapMode: Text.WordWrap
                    verticalAlignment: Text.AlignTop
                    horizontalAlignment: Text.AlignLeft

                }
            }

            Button {
                id: noBtn
                //anchors.centerIn: parent
                x: popupWindow.width - width - height
                y: parent.height - 20 - height
                width: 80
                height: 30

                background: Rectangle {
                    anchors.centerIn: parent
                    width: 80
                    height: 30
                    //border.width: 2
                    Text {
                        anchors.centerIn: parent
                        //font.family: "Microsoft Yahei"
                        //font.bold: true
                        font.pixelSize: 14
                        text: qsTr("否")
                        color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
                    }

                    color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                    border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
                }

                onClicked: { msgHintYesNoDlg.visible = false; popupWindow.hide() }
            }

            Button {
                id: yesBtn
                //anchors.centerIn: parent
                x: popupWindow.width - width - height - width - height
                y: parent.height - 20 - height
                width: 80
                height: 30

                background: Rectangle {
                    anchors.centerIn: parent
                    width: 80
                    height: 30
                    //border.width: 2
                    Text {
                        anchors.centerIn: parent
                        //font.family: "Microsoft Yahei"
                        //font.bold: true
                        font.pixelSize: 14
                        text: qsTr("是")
                        color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
                    }

                    color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                    border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
                }

                onClicked: {
                    var text = yesNoMsg.text
                    msgHintYesNoDlg.visible = false
                    popupWindow.hide()
                    var ret = ""
                    if (text.indexOf(guiString.uiHints(GUIString.NeedToBeReformattedHint)) >= 0) {
                        ret = main.reformatSpace()
                        if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                            main.showMsgBox(ret, GUIString.HintDlg)
                        } else {
                            main.updateFileList()
                        }
                    } else if(text.indexOf(guiString.uiHints(GUIString.AgentApplyNotCurrentHint)) >= 0) {
                        ret = main.applyForAgentService()
                        if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                            main.showMsgBox(ret, GUIString.HintDlg)
                        }
                    } else if(text.indexOf(guiString.uiHints(GUIString.MakeSureToLeaveHint)) >= 0) {
                        quitProgram()
                    }

                }
            }
        }

        Rectangle {
            id: waitingDlg
            x: 1
            y: titlePopupWindow.height
            width: popupWindow.width - 2
            height: popupWindow.height - titlePopupWindow.height - 2
            visible: false

            Rectangle {
                x: 30
                y: 10
                width: waitingDlg.width - 2*x
                height: 90
                border.color: "transparent"
                color: "transparent"
                Text {
                    anchors.fill: parent
                    anchors.centerIn: parent
                    font.family: "Microsoft Yahei"
                    color: "black"
                    text: tipText
                    wrapMode: Text.WordWrap
                    verticalAlignment: Text.AlignTop
                    horizontalAlignment: Text.AlignLeft

                }
            }
        }
    }

    Text {
        id: msg
        x: 30
        y: titlePopupWindow.height
        width: titlePopupWindow.width - 2*x
        height: 90
        wrapMode: Text.WordWrap
        objectName: "content"
        visible: false
    }

    Text {
        id: yesNoMsg
        x: 30
        y: titlePopupWindow.height
        width: titlePopupWindow.width - 2*x
        height: 90
        wrapMode: Text.WordWrap
        objectName: "contentYesNo"
        visible: false
    }

    function openHintBox() {
        msgHintDlg.visible = true
        //msgHintDlg.open()
    }

    function openYesNoBox() {
        msgHintYesNoDlg.visible = true
        //msgHintDlg.open()
    }

    function openWaitingBox() {
        waitingDlg.visible = true
        //waitingDlg.open()
    }

    function closeWaitingBox() {
        waitingDlg.visible = false
        popupWindow.hide()
        //waitingDlg.close()
    }

    function quitProgram() {
        var params = []
        rpcBackend.sendOperationCmd(GUIString.ExitExternGUICmd, params)
        Qt.quit()
    }
}
