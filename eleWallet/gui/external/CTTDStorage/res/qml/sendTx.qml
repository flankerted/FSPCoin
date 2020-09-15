import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: sendTxWindow
    width: 520
    height: 210
    flags: Qt.FramelessWindowHint | Qt.Window
    modality: Qt.ApplicationModal
    title: qsTr("发起转账")

//    MouseArea {
//        anchors.fill: parent
//        acceptedButtons: Qt.LeftButton
//        property point clickPosSetting: "0, 0"
//        onPressed: {
//            clickPosSetting = Qt.point(mouse.x, mouse.y)
//        }
//        onPositionChanged: {
//            if (pressed) {
//                var delta = Qt.point(mouse.x-clickPosSetting.x, mouse.y-clickPosSetting.y)

//                sendTxWindow.setX(sendTxWindow.x+delta.x)
//                sendTxWindow.setY(sendTxWindow.y+delta.y)
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: sendTxWindow.width
        height: sendTxWindow.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Rectangle {
            id: titleAreaSendingTxWin
            color: "#EFF2F6"
            border.color: "transparent"
            x: 0
            y: 0
            width: sendTxWindow.width
            height: 40

            Button {
                id: closeSendTxWindow

                x: sendTxWindow.width - 35
                y: 5
                width: 25
                height: 25

                background: Rectangle {
                    implicitHeight: closeSendTxWindow.height
                    implicitWidth:  closeSendTxWindow.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: closeSendTxWindow.hovered ? (closeSendTxWindow.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: sendTxWindow.close()
            }

            Image {
                x: 15
                y: 5
                width: 30
                height: 30
                source: "qrc:/images/main/icon-wemore.png"
                fillMode: Image.PreserveAspectFit
            }

            Text {
                id: sendTxTitle
                x: 50
                y: 5
                width: 420
                height: 30
                text: qsTr("发起转账: ") + main.account
                font.bold: true
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
                color: "black"
            }
        }

        Text {
            id: txToWhomLabel
            x: 50
            y: titleAreaSendingTxWin.height + 10
            width: 60
            height: 30
            text: qsTr("收款账号: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        TextInput {
            id: txToWhomEdit
            x: txToWhomLabel.x + txToWhomLabel.width
            y: txToWhomLabel.y
            width: 360
            height: 30
            text: ""
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            leftPadding: 12
            font.pixelSize: 14
        }

        Rectangle {
            color: "#C7C7C7"
            border.color: "transparent"
            x: txToWhomLabel.x + txToWhomLabel.width + 10
            y: txToWhomLabel.y + txToWhomLabel.height
            width: 360
            height: 1
        }

        Text {
            id: txAmoutLabel
            x: 50
            y: txToWhomLabel.y + 1.5 * txToWhomLabel.height
            width: 60
            height: 30
            text: qsTr("转账数额: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        TextInput {
            id: txAmoutEdit
            x: txAmoutLabel.x + txAmoutLabel.width
            y: txAmoutLabel.y
            width: 100
            height: 30
            text: ""
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            leftPadding: 12
            font.pixelSize: 14
        }

        Rectangle {
            color: "#C7C7C7"
            border.color: "transparent"
            x: txAmoutLabel.x + txAmoutLabel.width + 10
            y: txAmoutLabel.y + txAmoutLabel.height
            width: 100
            height: 1
        }

        Text {
            id: accountPwdLabel
            x: 50
            y: txToWhomLabel.y + 3 * txToWhomLabel.height
            width: 60
            height: 30
            text: qsTr("账户密码: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        TextInput {
            id: accountPwdEdit
            x: accountPwdLabel.x + accountPwdLabel.width
            y: accountPwdLabel.y
            width: 100
            height: 30
            text: ""
            echoMode: TextInput.Password
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            leftPadding: 12
            font.pixelSize: 14
        }

        Rectangle {
            color: "#C7C7C7"
            border.color: "transparent"
            x: accountPwdLabel.x + accountPwdLabel.width + 10
            y: accountPwdLabel.y + accountPwdLabel.height
            width: 100
            height: 1
        }

        Button {
            id: startSendingTx
            x: sendTxWindow.width - 2 * width - 2 * 30
            y: sendTxWindow.height - titleAreaSendingTxWin.height - 30
            width: 100
            height: 30
            enabled: (txToWhomEdit.text !== "" && txAmoutEdit.text !== "" && accountPwdEdit.text !== "")

            Text {
                text: qsTr("发起转账")
                font.pointSize: 10
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignHCenter
                color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
            }

            background: Rectangle {
                color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
            }

            onClicked: sendTransaction()
        }

        Button {
            id: sendTxCancel
            x: sendTxWindow.width - width - 30
            y: startSendingTx.y
            width: 100
            height: 30

            Text {
                text: qsTr("取消转账")
                font.pointSize: 10
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignHCenter
                color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
            }

            background: Rectangle {
                color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
            }

            onClicked: sendTxWindow.close()
        }
    }

    function sendTransaction() {
        if (txToWhomEdit.text !== "" && txAmoutEdit.text !== "" && accountPwdEdit.text !== "") {
            var receiver = txToWhomEdit.text
            var amount = txAmoutEdit.text
            var accPwd = accountPwdEdit.text
            txToWhomEdit.text = ""
            txAmoutEdit.text = ""
            accountPwdEdit.text = ""
            var ret = main.sendTransaction(receiver, amount, accPwd)
            main.showMsgBox(ret, GUIString.HintDlg, ret.indexOf(guiString.uiHints(GUIString.TxSuccessfullySentHint)) >= 0)
        }
    }
}
