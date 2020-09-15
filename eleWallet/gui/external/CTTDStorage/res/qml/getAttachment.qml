import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: getMailAttachmentWin
    width: 320
    height: 300
    flags: Qt.FramelessWindowHint | Qt.Window
    modality: Qt.ApplicationModal
    title: qsTr("获取附件")

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

//                getMailAttachmentWin.setX(getMailAttachmentWin.x+delta.x)
//                getMailAttachmentWin.setY(getMailAttachmentWin.y+delta.y)
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: getMailAttachmentWin.width
        height: getMailAttachmentWin.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Rectangle {
            id: titleAreaGetAttachmentWin
            color: "#EFF2F6"
            border.color: "transparent"
            x: 0
            y: 0
            width: getMailAttachmentWin.width
            height: 40

            Button {
                id: closeGetMailAttachmentWin

                x: getMailAttachmentWin.width - 35
                y: 5
                width: 25
                height: 25

                background: Rectangle {
                    implicitHeight: closeGetMailAttachmentWin.height
                    implicitWidth:  closeGetMailAttachmentWin.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: closeGetMailAttachmentWin.hovered ? (closeGetMailAttachmentWin.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: getMailAttachmentWin.close()
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
                id: getMailAttachmentTitle
                x: 50
                y: 5
                width: 220
                height: 30
                text: qsTr("获取附件: ") + main.sharingInfoFileName
                font.bold: true
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
                elide: Text.ElideRight
                color: "lightsteelblue"
            }
        }

        Text {
            id: sharingFileHint
            x: 70
            y: titleAreaGetAttachmentWin.height + 20
            height: 20
            text: qsTr("文件: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: sharingFileName
            x: sharingFileHint.x + 40
            y: sharingFileHint.y
            height: 20
            text: main.sharingInfoFileName
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        Text {
            id: sharingTimeLimitHint
            x: sharingFileHint.x
            y: sharingFileHint.y + 1.5 * sharingFileHint.height
            height: 20
            text: qsTr("有效期: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            x: sharingFileHint.x + 20
            y: sharingFileHint.y + 3 * sharingFileHint.height
            height: 20
            text: qsTr("从 ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: sharingTimeLimitStart
            x: sharingFileHint.x + 20 + 35
            y: sharingFileHint.y + 3 * sharingFileHint.height
            height: 20
            text: main.sharingInfoStartTime
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            x: sharingFileHint.x + 20
            y: sharingFileHint.y + 4.5 * sharingFileHint.height
            height: 20
            text: qsTr("到 ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: sharingTimeLimitStop
            x: sharingFileHint.x + 20 + 35
            y: sharingFileHint.y + 4.5 * sharingFileHint.height
            height: 20
            text: main.sharingInfoStopTime
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: sharingPriceHint
            x: sharingFileHint.x
            y: sharingFileHint.y + 6 * sharingFileHint.height
            height: 20
            text: qsTr("分享价格: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: sharingPrice
            x: sharingPriceHint.x + 70
            y: sharingFileHint.y + 6 * sharingFileHint.height
            height: 20
            text: main.sharingInfoPrice
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Button {
            id: getAttachment
            x: getMailAttachmentWin.width - 2 * width - 2 * 40
            y: getMailAttachmentWin.height - titleAreaGetAttachmentWin.height - 30
            width: 100
            height: 30

            Text {
                text: qsTr("获取附件")
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

            onClicked: {
                getMailAttachmentWin.close()
                var ret = main.payforSharing(main.sharingCode)
                main.showMsgBox(ret, GUIString.HintDlg, ret.indexOf(guiString.uiHints(GUIString.PayForSharingSuccHint)) >= 0)
            }
        }

        Button {
            id: getAttachmentCancel
            x: getMailAttachmentWin.width - width - 40
            y: getAttachment.y
            width: 100
            height: 30

            Text {
                text: qsTr("取消")
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

            onClicked: getMailAttachmentWin.close()
        }
    }
}
