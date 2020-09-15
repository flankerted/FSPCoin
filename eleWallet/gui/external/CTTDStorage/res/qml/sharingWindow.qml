import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: sharingWindow
    width: 640
    height: 400
    flags: Qt.FramelessWindowHint | Qt.Window
    modality: Qt.ApplicationModal
    visible: false
    title: qsTr("分享文件")

    signal mailFileSharingCreated()

    property int selectedFileIdx: 0
    property int selectedSpaceIdx: 0
    property bool addAttachmentFlag: false
    property string mailReceiver: ""

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

//                sharingWindow.setX(sharingWindow.x+delta.x)
//                sharingWindow.setY(sharingWindow.y+delta.y)
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: sharingWindow.width
        height: sharingWindow.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Rectangle {
            id: titleAreaSharingWin
            color: "#EFF2F6"
            border.color: "transparent"
            x: 1
            y: 1
            width: sharingWindow.width - 2
            height: 40 - 1

            Button {
                id: closeSharingWindow

                x: sharingWindow.width - 35
                y: 5
                width: 25
                height: 25

                background: Rectangle {
                    implicitHeight: closeSharingWindow.height
                    implicitWidth:  closeSharingWindow.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: closeSharingWindow.hovered ? (closeSharingWindow.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: sharingWindow.close()
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
                id: downloadTitle
                x: 50
                y: 5
                width: 420
                height: 30
                text: qsTr("分享文件: ") + main.getFileNameByIndex(sharingWindow.selectedFileIdx)
                font.bold: true
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
                color: "black"
            }
        }

        Rectangle {
            id: shareCategoryLine
            color: "#C7C7C7"
            border.color: "transparent"
            x: 0
            y: titleAreaSharingWin.height + 39
            width: sharingWindow.width
            height: 1
        }

        SwipeView {
            id: sharingSwipeView
            anchors.top: parent.top
            anchors.topMargin: shareCategoryLine.y + shareCategoryLine.height
            width: sharingWindow.width
            height: sharingWindow.height - titleAreaSharingWin.height - shareCategoryLine.y - shareCategoryLine.height
            currentIndex: 0 //headertabBar.currentIndex

            Page {
                id: shareToAccountPage
                background: Rectangle {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
//                    Gradient {
//                        GradientStop{ position: 0; color: "#487A89" }
//                        GradientStop{ position: 1; color: "#54DAB3" }
//                    }
                }

                Text {
                    id: shareToWhomLabel
                    x: 50
                    y: 10
                    width: 140
                    height: 30
                    text: qsTr("分享的目标用户的账号: ")
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    font.pixelSize: 14
                }

                TextInput {
                    id: shareToWhomEdit
                    x: shareToWhomLabel.x + shareToWhomLabel.width
                    y: shareToWhomLabel.y
                    width: 360
                    height: 30
                    text: sharingWindow.mailReceiver
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    leftPadding: 12
                    font.pixelSize: 14
                    KeyNavigation.tab: sharingTimeStart1
                }

                Rectangle {
                    color: "#C7C7C7"
                    border.color: "transparent"
                    x: shareToWhomLabel.x + shareToWhomLabel.width + 10
                    y: shareToWhomLabel.y + shareToWhomLabel.height
                    width: 360
                    height: 1
                }

                Text {
                    id: timeLimitToShareHintLabel
                    x: 50
                    y: shareToWhomLabel.y + 1.5 * shareToWhomLabel.height
                    width: 60
                    height: 30
                    text: qsTr("有效期: ")
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    font.pixelSize: 14
                }

                Rectangle {
                    id: timeLimitToShareBox
                    x: timeLimitToShareHintLabel.x + timeLimitToShareHintLabel.width + (main.getLanguage() === "EN" ? 60 : 0)
                    y: timeLimitToShareHintLabel.y
                    width: 400
                    height: 30
                    color: "transparent"

                    Component.onCompleted: {
                        var date = new Date()
                        sharingTimeStart1.text = Qt.formatDateTime(date, "yyyy")
                        sharingTimeStart2.text = Qt.formatDateTime(date, "MM")
                        sharingTimeStart3.text = Qt.formatDateTime(date, "dd")
                    }

                    Row {
                        anchors.fill: parent
                        Rectangle {
                            id: sharingTimeStartBox
                            width: 100
                            height: 30
                            color: "transparent"
                            border.color: "#C7C7C7" //canApplyForService() ? "#C7C7C7" : "transparent"
                            Row {
                                Text {
                                    font.pointSize: 11; text: " "; height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStart1
                                    font.pointSize: 12
                                    width: (sharingTimeStartBox.width-10) / 10 * 4
                                    height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 2019; top: 9999; }
                                    focus: true
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingTimeStart2
                                }
                                Text {
                                    font.pointSize: 12; text: "-"; height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStart2
                                    font.pointSize: 12
                                    width: (sharingTimeStartBox.width-10) / 10 * 2
                                    height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 0; top: 12; }
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingTimeStart3
                                }
                                Text {
                                    font.pointSize: 12; text: "-"; height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStart3
                                    font.pointSize: 12
                                    width: (sharingTimeStartBox.width-10) / 10 * 2
                                    height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 0; top: 12; }
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingTimeStop1
                                }
                                Text {
                                    font.pointSize: 12; text: " "; height: sharingTimeStartBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                            }
                        }
                        Text {
                            text: qsTr(" 到 ")
                            font.pointSize: 11
                            height: 30
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                        }
                        Rectangle {
                            id: sharingTimeStopBox
                            width: 100
                            height: 30
                            color: "transparent"
                            border.color: "#C7C7C7" //canApplyForService() ? "#C7C7C7" : "transparent"
                            Row {
                                Text {
                                    font.pointSize: 11; text: " "; height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStop1
                                    font.pointSize: 12
                                    width: (sharingTimeStopBox.width-10) / 10 * 4
                                    height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 2019; top: 9999; }
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingTimeStop2
                                }
                                Text {
                                    font.pointSize: 12; text: "-"; height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStop2
                                    font.pointSize: 12
                                    width: (sharingTimeStopBox.width-10) / 10 * 2
                                    height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 0; top: 12; }
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingTimeStop3
                                }
                                Text {
                                    font.pointSize: 12; text: "-"; height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                                TextInput {
                                    id: sharingTimeStop3
                                    font.pointSize: 12
                                    width: (sharingTimeStopBox.width-10) / 10 * 2
                                    height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignLeft
                                    validator: IntValidator { bottom: 0; top: 12; }
                                    //enabled: canApplyForService()
                                    KeyNavigation.tab: sharingPriceEdit
                                }
                                Text {
                                    font.pointSize: 12; text: " "; height: sharingTimeStopBox.height
                                    verticalAlignment: Text.AlignVCenter
                                }
                            }
                        }
                    }
                }

                Text {
                    id: sharingPriceHintLabel
                    x: 50
                    y: timeLimitToShareBox.y + 1.5 * timeLimitToShareBox.height
                    width: 70
                    height: 30
                    text: qsTr("分享价格: ")
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    font.pixelSize: 14
                }

                TextInput {
                    id: sharingPriceEdit
                    x: sharingPriceHintLabel.x + sharingPriceHintLabel.width
                    y: sharingPriceHintLabel.y
                    width: 60
                    height: 30
                    text: ""
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    leftPadding: 4
                    font.pixelSize: 14
                    KeyNavigation.tab: shareToWhomEdit
                }

                Rectangle {
                    color: "#C7C7C7"
                    border.color: "transparent"
                    x: sharingPriceHintLabel.x + sharingPriceHintLabel.width
                    y: sharingPriceHintLabel.y + sharingPriceHintLabel.height
                    width: 60
                    height: 1
                }

                Text {
                    id: sharingCodeHintLabel
                    x: 50
                    y: sharingPriceHintLabel.y + 1.5 * sharingPriceHintLabel.height
                    width: 60
                    height: 30
                    visible: (sharingCode.text !== "")
                    text: qsTr("分享码: ")
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignLeft
                    font.pixelSize: 14
                }

                Text {
                    id: sharingCode
                    x: 50
                    y: sharingCodeHintLabel.y + sharingCodeHintLabel.height - 5
                    width: sharingWindow.width - x - 30
                    height: 90
                    text: ""
                    wrapMode: Text.Wrap
                    font.pixelSize: 12
                    visible: !addAttachmentFlag
                }

                Button {
                    id: createSharingCode
                    x: 380
                    y: sharingWindow.height - shareCategoryLine.y - shareCategoryLine.height - 60
                    width: 100
                    height: 30
                    enabled: (sharingCode.text === "" && (shareToWhomEdit.text !== "" && sharingPriceEdit.text !== "" &&
                                                          sharingTimeStart1.text !== "" && sharingTimeStart2.text !== "" &&
                                                          sharingTimeStart3.text !== "" && sharingTimeStop1.text !== "" &&
                                                          sharingTimeStop2.text !== "" && sharingTimeStop3.text !== ""))

                    Text {
                        text: addAttachmentFlag ? qsTr("确定") : qsTr("创建分享")
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

                    onClicked: shareFileToAccount()
                }

                Button {
                    id: sharingCancel
                    x: createSharingCode.x + createSharingCode.width + 30
                    y: createSharingCode.y
                    width: 100
                    height: 30

                    Text {
                        text: addAttachmentFlag ? qsTr("取消") : ((sharingCode.text !== "") ? qsTr("关闭") : qsTr("取消分享"))
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

                    onClicked: sharingWindow.close()
                }

                Button {
                    id: copySharingCode
                    x: createSharingCode.x - createSharingCode.width - 30
                    y: createSharingCode.y
                    width: 100
                    height: 30
                    visible: (sharingCode.text !== "" && !addAttachmentFlag)
                    enabled: (sharingCode.text !== "" && !addAttachmentFlag)

                    Text {
                        text: qsTr("复制分享码")
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
                        if (sharingCode !== "") {
                            main.copyToClipboard(sharingCode.text)
                        }
                    }
                }
            }

            Page {
                background: Rectangle {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    Gradient {
                        GradientStop{ position: 0; color: "#487A89" }
                        GradientStop{ position: 1; color: "#54DAB3" }
                    }
                }

                Label {
                    text: qsTr("分享给公众(待实现)")
                    anchors.centerIn: parent
                    font.pixelSize: 30
                    font.italic: true
                    color: "yellow"
                }
            }
        }
    }

    Connections {
        target: main
        onSharingCodeChanged: {
            sharingCode.text = main.sharingCode
        }
    }

    function shareFileToAccount() {
        if (shareToWhomEdit.text !== "" && sharingPriceEdit.text !== "" &&
                sharingTimeStart1.text !== "" && sharingTimeStart2.text !== "" &&
                sharingTimeStart3.text !== "" && sharingTimeStop1.text !== "" &&
                sharingTimeStop2.text !== "" && sharingTimeStop3.text !== "") {
            if (sharingTimeStart2.text.length == 1) {
                sharingTimeStart2.text = "0" + sharingTimeStart2.text
            }
            if (sharingTimeStart3.text.length == 1) {
                sharingTimeStart3.text = "0" + sharingTimeStart3.text
            }
            if (sharingTimeStop2.text.length == 1) {
                sharingTimeStop2.text = "0" + sharingTimeStop2.text
            }
            if (sharingTimeStop3.text.length == 1) {
                sharingTimeStop3.text = "0" + sharingTimeStop3.text
            }

            var yearStart = sharingTimeStart1.text
            var yearStop = sharingTimeStop1.text
            var monthStart = sharingTimeStart2.text
            var monthStop = sharingTimeStop2.text
            var dayStart = sharingTimeStart3.text
            var dayStop = sharingTimeStop3.text
            if (Number(yearStart) < 2020 || Number(yearStop) < 2020 ||
                    Number(monthStart) < 0 || Number(monthStart) > 12 || Number(monthStop) < 0 || Number(monthStop) > 12 ||
                    Number(dayStart) < 0 || Number(dayStart) > 31 || Number(dayStop) < 0 || Number(dayStop) > 31 ||
                    Number(yearStart)*10000+Number(monthStart)*100+Number(dayStart) > Number(yearStop)*10000+Number(monthStop)*100+Number(dayStop)) {
                main.showMsgBox(guiString.uiHints(GUIString.InvalidSharingDurationHint), GUIString.HintDlg)
                return
            }

            var timeStart = sharingTimeStart1.text + "-" + sharingTimeStart2.text + "-" +
                    sharingTimeStart3.text + " 00:00:00"
            var timeStop = sharingTimeStop1.text + "-" + sharingTimeStop2.text + "-" +
                    sharingTimeStop3.text + " 23:59:59"
            var ret = main.shareFileToAccount(sharingWindow.selectedSpaceIdx, sharingWindow.selectedFileIdx,
                                              shareToWhomEdit.text, timeStart, timeStop, sharingPriceEdit.text)
            if (addAttachmentFlag) {
                sharingWindow.close()
            }
            if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                main.showMsgBox(ret, GUIString.HintDlg)
            } else {
                sharingWindow.mailFileSharingCreated()
            }
        }
    }
}
