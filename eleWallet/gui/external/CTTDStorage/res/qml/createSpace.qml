import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: createSpaceDlg
    width: 180
    height: 230
    flags: Qt.FramelessWindowHint | Qt.Window
    modality: Qt.ApplicationModal
    title: qsTr("创建新空间 ")

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

//                createSpaceDlg.setX(createSpaceDlg.x+delta.x)
//                createSpaceDlg.setY(createSpaceDlg.y+delta.y)
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: createSpaceDlg.width
        height: createSpaceDlg.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Button {
            id: closeDlg

            x: createSpaceDlg.width - 35
            y: 10
            width: 25
            height: 25

            background: Rectangle {
                implicitHeight: closeDlg.height
                implicitWidth:  closeDlg.width

                color: "transparent"  //transparent background

                BorderImage {
                    property string nomerPic: "qrc:/images/login/icon-close.png"
                    property string hoverPic: "qrc:/images/login/icon-close.png"
                    property string pressPic: "qrc:/images/login/icon-close.png"

                    anchors.fill: parent
                    source: closeDlg.hovered ? (closeDlg.pressed ? pressPic : hoverPic) : nomerPic;
                }
            }

            onClicked: createSpaceDlg.close()
        }

        Text {
            id: createSpaceTitle
            x: 50
            y: 20
            width: 80
            height: 30
            text: qsTr("创建空间")
            fontSizeMode: Text.Fit
            font.italic: false
            style: Text.Sunken
            font.family: "Times New Roman"
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 52
        }

//        Image {
//            x: 25
//            y: 65
//            width: 20
//            height: 20
//            source: "qrc:/images/main/icon-space.png"
//            fillMode: Image.PreserveAspectFit
//        }

        Text {
            id: spaceNameLabel
            x: 25
            y: 60
            width: 40
            height: 30
            text: qsTr("名称: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        TextInput {
            id: spaceNameEdit
            x: 65
            y: 60
            width: 70
            height: 30
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            leftPadding: 5
            font.pixelSize: 14
        }

        Rectangle {
            color: "#C7C7C7"
            border.color: "transparent"
            x: 65
            y: 90
            width: 90
            height: 1
        }

//        Image {
//            x: 25
//            y: 105
//            width: 20
//            height: 20
//            source: "qrc:/images/main/icon-space.png"
//            fillMode: Image.PreserveAspectFit
//        }

        Text {
            id: spaceSizeLabel
            x: 25
            y: 100
            width: 40
            height: 30
            text: qsTr("大小: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        TextInput {
            id: spaceSizeEdit
            x: 65
            y: 100
            width: 70
            height: 30
            text: ""
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            leftPadding: 5
            font.pixelSize: 14
            validator: IntValidator { bottom: 0; top: 999 }
        }

        Rectangle {
            color: "#C7C7C7"
            border.color: "transparent"
            x: 65
            y: 130
            width: 90
            height: 1
        }

        Text {
            id: spaceSizeUnit
            x: spaceSizeEdit.x + spaceSizeEdit.width
            y: spaceSizeEdit.y
            width: 20
            height: 30
            text: qsTr("MB")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 14
        }

        Text {
            id: priceLabel
            x: 25
            y: 140
            width: 40
            height: 30
            text: qsTr("价格: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        Text {
            id: priceValue
            x: priceLabel.x + priceLabel.width + 5
            y: priceLabel.y
            width: 70
            height: 30
            text: spaceSizeEdit.text
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 14
        }

        Button {
            id: spaceNewBtn
            x: 20
            y: 180
            width: 60
            height: 25
            enabled: spaceNameEdit.text !== "" && spaceSizeEdit.text !== ""

            Text {
                text: qsTr("创建")
                font.pointSize: 10
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignHCenter
                color: (parent.pressed || (!parent.enabled)) ? "#999999" : "black"
            }

            background: Rectangle {
                //radius: height/2
                color: parent.enabled ? (parent.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                border.color: (parent.pressed) ? "#E7E7E7" : (parent.hovered ? "#7706A8FF" : "#CFCFCF")
            }

            onClicked: newSpaceCreating()
        }

        Button {
            id: spaceNewCancelBtn
            x: 100
            y: spaceNewBtn.y
            width: 60
            height: 25

            Text {
                text: qsTr("取消")
                font.pointSize: 10
                anchors.fill: parent
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignHCenter
                color: (spaceNewCancelBtn.pressed || (!spaceNewCancelBtn.enabled)) ? "#999999" : "black"
            }

            background: Rectangle {
                //radius: height/2
                color: spaceNewCancelBtn.enabled ? (spaceNewCancelBtn.hovered ? "#1106A8FF" : "#F7F7F7") : "#FBFBFB"
                border.color: (spaceNewCancelBtn.pressed) ? "#E7E7E7" : (spaceNewCancelBtn.hovered ? "#7706A8FF" : "#CFCFCF")
            }

            onClicked: createSpaceDlg.close()
        }
    }

    function newSpaceCreating() {
        if (spaceNameEdit.text !== "" && spaceSizeEdit.text !== "") {
            createSpaceDlg.hide()
            var ret = main.createSpace(spaceNameEdit.text, spaceSizeEdit.text)

            createSpaceDlg.close()
            if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                main.showMsgBox(ret, GUIString.HintDlg)
            }
        }
    }
}
