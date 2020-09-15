import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: addMailAttachmentWin
    width: 640
    height: 400
    flags: Qt.FramelessWindowHint | Qt.Window
    modality: Qt.ApplicationModal
    title: qsTr("添加分享文件")

    signal fileAttached(string fileName)

    property int mainSpaceIndex: 0
    //property int currentFileIdx: -1
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

//                addMailAttachmentWin.setX(addMailAttachmentWin.x+delta.x)
//                addMailAttachmentWin.setY(addMailAttachmentWin.y+delta.y)
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: addMailAttachmentWin.width
        height: addMailAttachmentWin.height
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
            width: addMailAttachmentWin.width - 2
            height: 40 - 1

            Button {
                id: closeAddMailAttachmentWin

                x: addMailAttachmentWin.width - 35
                y: 5
                width: 25
                height: 25

                background: Rectangle {
                    implicitHeight: closeAddMailAttachmentWin.height
                    implicitWidth:  closeAddMailAttachmentWin.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: closeAddMailAttachmentWin.hovered ? (closeAddMailAttachmentWin.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: closeAddAttachmentWin()
            }

            Image {
                x: 15
                y: 10
                width: 20
                height: 20
                source: "qrc:/images/main/icon-attachment.png"
                fillMode: Image.PreserveAspectFit
            }

            Text {
                id: addAttachWinTitle
                x: 50
                y: 5
                width: 420
                height: 30
                text: qsTr("添加要分享的文件")
                font.bold: true
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
                color: "lightsteelblue"
            }
        }

        Rectangle {
            id: addAttachLeftAreaLine
            color: "#D6D8DD"
            border.color: "transparent"
            x: 170
            y: titleAreaSharingWin.height
            width: 1
            height: parent.height - titleAreaSharingWin.height - 90
        }

        Rectangle {
            id: addAttachLeftAreabackground
            x: 1
            y: titleAreaSharingWin.height
            width: addAttachLeftAreaLine.x - 1
            height: parent.height - titleAreaSharingWin.height - 90
            color: "#F9FAFB"
        }

        Rectangle {
            id: bottomAreaLine
            color: "#D6D8DD"
            border.color: "transparent"
            x: 0
            y: addAttachLeftAreaLine.y + addAttachLeftAreaLine.height
            width: parent.width
            height: 1
        }

        Button {
            id: allSpaceAddAttach
            x: 0
            y: titleAreaSharingWin.height
            width: addAttachLeftAreaLine.x
            height: 40

            BorderImage {
                anchors.left: parent.left
                anchors.leftMargin: 20
                anchors.verticalCenter: parent.verticalCenter
                width: 20
                height: 20
                source: "qrc:/images/main/icon-space.png"
            }

            Text {
                text: qsTr("我的空间:")
                anchors.fill: parent
                font.pointSize: 11
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignHCenter
                color: "black" //(mainWindow.spaceCategory === 0 || allSpaceAddAttach.hovered) ? "white" : "black"
            }

            Rectangle {
                color: "transparent" //mainWindow.spaceCategory === 0 ? "lightsteelblue" : "transparent"
                border.color: "transparent"
                x: 0
                anchors.left: parent.left
                anchors.leftMargin: 0
                width: 4
                height: parent.height
            }

            background: Rectangle {
                color: "transparent" //mainWindow.spaceCategory === 0 ? "#77000000" : "transparent"
                border.color: "transparent"
            }

            //onClicked: { setSpace(0); spaceListAddAttach.highlightFollowsCurrentItem = false }
        }

        Rectangle {
            color: "#D6D8DD"
            border.color: "transparent"
            x: 0
            y: allSpaceAddAttach.y + allSpaceAddAttach.height
            width: addAttachLeftAreaLine.x
            height: 1
        }

        ListModel {
            id: spaceModelTest
            ListElement {
                space: "A"
            }
            ListElement {
                space: "B"
            }
            ListElement {
                space: "C"
            }
            ListElement {
                space: "D"
            }
            ListElement {
                space: "E"
            }
            ListElement {
                space: "F"
            }
            ListElement {
                space: "G"
            }
            ListElement {
                space: "H"
            }
        }

        ListView {
            id: spaceListAddAttach
            x: 0
            y: allSpaceAddAttach. y + allSpaceAddAttach.height + 1
            width: addAttachLeftAreaLine.x - spaceLabelScrollbar.width - 1
            height: addAttachLeftAreaLine.height - allSpaceAddAttach.height
            model: spaceListModel
            highlightFollowsCurrentItem: true
            highlightMoveDuration: 0
            highlight: Rectangle {
                Rectangle {
                    color: spaceListAddAttach.highlightFollowsCurrentItem ? "#06A8FF" : "transparent"
                    border.color: "transparent"
                    x: 0
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    width: 4
                    height: parent.height
                }

                color: spaceListAddAttach.highlightFollowsCurrentItem ? "#3306A8FF" : "transparent"
            }
            clip: true
            focus: false

            delegate: Row {
                id: spaceListAddAttachRow
                property bool hovered: false
                Rectangle {
                    width: spaceLabel.width
                    height: spaceLabel.height
                    color: "transparent"
                    Text {
                        id: spaceLabel; width: addAttachLeftAreaLine.x; height: allSpaceAddAttach.height
                        anchors.left: parent.left
                        anchors.leftMargin: 55
                        verticalAlignment: Text.AlignVCenter
                        text: space
                        font.pointSize: 11
                        color: ((spaceListAddAttachRow.ListView.isCurrentItem && spaceListAddAttach.highlightFollowsCurrentItem) ||
                                spaceListAddAttachRow.hovered) ? "#06A8FF" : "black"
                    }

                    MouseArea {
                        acceptedButtons: Qt.LeftButton
                        hoverEnabled: true
                        anchors.fill: parent
                        onEntered: { spaceListAddAttachRow.hovered = true }
                        onExited: { spaceListAddAttachRow.hovered = false }
                        onClicked: {
                            spaceListAddAttach.highlightFollowsCurrentItem = true
                            spaceListAddAttachRow.ListView.view.currentIndex = index
                            main.currentSpaceIdx = index
                            main.showFileList(main.getSpaceLabelByIndex(index))
                        }
                    }
                }
            }
        }

        Rectangle {
            id: spaceLabelScrollbar
            width: 6
            x: addAttachLeftAreaLine.x - width - 1;
            y: spaceListAddAttach.y + 1
            height: spaceListAddAttach.height - 2
            radius: width/2
            clip: true

            Rectangle {
                id: dragButton
                x: 0
                y: spaceListAddAttach.visibleArea.yPosition * spaceLabelScrollbar.height
                width: parent.width
                height: spaceListAddAttach.visibleArea.heightRatio * spaceLabelScrollbar.height;
                color: "#CFD1D3"
                radius: width
                clip: true

                MouseArea {
                    id: mouseArea
                    anchors.fill: dragButton
                    drag.target: dragButton
                    drag.axis: Drag.YAxis
                    drag.minimumY: 0
                    drag.maximumY: spaceLabelScrollbar.height - dragButton.height

                    onMouseYChanged: {
                        spaceListAddAttach.contentY = dragButton.y / spaceLabelScrollbar.height * spaceListAddAttach.contentHeight
                    }
                }
            }
        }

        Rectangle {
            id: fileListAddAttachTitle
            x: addAttachLeftAreaLine.x
            y: titleAreaSharingWin.height
            width: parent.width - addAttachLeftAreaLine.x - 1
            height: allSpaceAddAttach.height
            color: "white"
            Row {
                Rectangle {
                    id: fileListAddAttachTitle1
                    width: fileListAddAttachTitle.width - fileListAddAttachTitle3.width; height: allSpaceAddAttach.height
                    color: fileListAddAttachTitle.color
                    Text {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        verticalAlignment: Text.AlignVCenter
                        width: fileListAddAttachTitle1.width
                        height: fileListAddAttachTitle1.height
                        text: qsTr("文件名 ")
                        font.pointSize: 12
                        color: "#757880"
                    }
                    MouseArea { anchors.fill: parent; onClicked: { } }
                    Rectangle {
                        anchors.right: parent.right
                        anchors.rightMargin: 0
                        width: 1
                        height: fileListAddAttachTitle1.height
                        color: "#EBEBEC"
                    }
                }

//                Rectangle {
//                    id: fileListAddAttachTitle2
//                    width: 330; height: allSpaceAddAttach.height
//                    color: fileListAddAttachTitle.color
//                    Text {
//                        verticalAlignment: Text.AlignVCenter
//                        horizontalAlignment: Text.AlignHCenter
//                        width: fileListAddAttachTitle2.width
//                        height: fileListAddAttachTitle2.height
//                        text: qsTr("有效期")
//                        font.pointSize: 13
//                    }
//                    MouseArea { anchors.fill: parent; onClicked: { } }
//                    Rectangle {
//                        anchors.right: parent.right
//                        anchors.rightMargin: 0
//                        width: 1
//                        height: fileListAddAttachTitle2.height
//                        color: "black"
//                    }
//                }

                Rectangle {
                    id: fileListAddAttachTitle3
                    width: 150; height: allSpaceAddAttach.height
                    color: fileListAddAttachTitle.color
                    Text {
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        width: fileListAddAttachTitle3.width
                        height: fileListAddAttachTitle3.height
                        text: qsTr("大小")
                        font.pointSize: 13
                        color: "#757880"
                    }
                    MouseArea { anchors.fill: parent; onClicked: { } }
                    Rectangle {
                        anchors.right: parent.right
                        anchors.rightMargin: 0
                        width: 1
                        height: fileListAddAttachTitle3.height
                        color: "#EBEBEC"
                    }
                }
            }
        }

        Rectangle {
            id: fileListAddAttachBackground
            color: "transparent"
            border.color: "transparent"
            x: addAttachLeftAreaLine.x
            y: fileListAddAttachTitle.y + fileListAddAttachTitle.height
            width: fileListAddAttachTitle.width
            height: bottomAreaLine.y - fileListAddAttachTitle.y - fileListAddAttachTitle.height

            ListModel {
                id: fileListModelTest
                ListElement {
                    fileName: "Aqe21qwdeqww.txt"
                    timeLimit: "2019-01-01~2019-12-30"
                    fileSize: "1434MB"
                }
                ListElement {
                    fileName: "Bcvbzvcvzv.ini"
                    timeLimit: "-"
                    fileSize: "5MB"
                }
                ListElement {
                    fileName: "Cdfasdfadf"
                    timeLimit: "2019-01-01~2019-12-30"
                    fileSize: "14MB"
                }
            }

            ListView {
                id: fileListAddAttach
                x: 0
                y: 0
                width: parent.width - fileListAddAttachScrollbar.width - 2
                height: parent.height
                model: fileListModel
                highlightFollowsCurrentItem: true
                highlightMoveDuration: 0
                highlight: Rectangle {
                    color: "#3306A8FF"
                }
                clip: true
                focus: false

                Component.onCompleted: {
                    main.showFileList(main.getSpaceLabelByIndex(spaceListAddAttach.currentIndex))
                }

                delegate: Row {
                    id: fileListAddAttachRow
                    property bool hovered: false
                    Rectangle {
                        width: fileNameText.width
                        height: fileNameText.height
                        color: fileListAddAttachRow.hovered ? "#3306A8FF" : "transparent"
                        Text {
                            id: fileNameText; width: fileListAddAttachTitle1.width; height: allSpaceAddAttach.height
                            leftPadding: 20
                            verticalAlignment: Text.AlignVCenter
                            text: fileName
                            font.pointSize: 11
                            elide: Text.ElideRight
                            color: "black"
                        }

                        MouseArea {
                            acceptedButtons: Qt.LeftButton
                            hoverEnabled: true
                            anchors.fill: parent
                            onEntered: { fileListAddAttachRow.hovered = true }
                            onExited: { fileListAddAttachRow.hovered = false }
                            onClicked: {
                                fileListAddAttachRow.ListView.view.currentIndex = index
                                //addMailAttachmentWin.currentFileIdx = fileListAddAttach.currentIndex
                            }
                        }
                    }

//                    Rectangle {
//                        width: timeLimitText.width
//                        height: timeLimitText.height
//                        color: fileListAddAttachRow.hovered ? "#55B0C4DE" : "transparent"
//                        Text {
//                            id: timeLimitText; width: fileListAddAttachTitle2.width; height: allSpaceAddAttach.height
//                            horizontalAlignment: Text.AlignHCenter
//                            verticalAlignment: Text.AlignVCenter
//                            text: timeLimit
//                            font.pointSize: 11
//                            color: (fileListAddAttachRow.ListView.isCurrentItem || fileListAddAttachRow.hovered) ? "black" : "lightsteelblue"
//                        }

//                        MouseArea {
//                            acceptedButtons: Qt.LeftButton
//                            hoverEnabled: true
//                            anchors.fill: parent
//                            onEntered: { fileListAddAttachRow.hovered = true }
//                            onExited: { fileListAddAttachRow.hovered = false }
//                            onClicked: {
//                                fileListAddAttachRow.ListView.view.currentIndex = index
//                                main.currentFileIdx = fileListAddAttach.currentIndex
//                            }
//                        }
//                    }

                    Rectangle {
                        width: fileSizeText.width
                        height: fileSizeText.height
                        color: fileListAddAttachRow.hovered ? "#3306A8FF" : "transparent"
                        Text {
                            id: fileSizeText; width: fileListAddAttachTitle3.width; height: allSpaceAddAttach.height
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            text: fileSize
                            font.pointSize: 11
                            color: "#757880"
                        }

                        MouseArea {
                            acceptedButtons: Qt.LeftButton
                            hoverEnabled: true
                            anchors.fill: parent
                            onEntered: { fileListAddAttachRow.hovered = true }
                            onExited: { fileListAddAttachRow.hovered = false }
                            onClicked: {
                                fileListAddAttachRow.ListView.view.currentIndex = index
                                //addMailAttachmentWin.currentFileIdx = fileListAddAttach.currentIndex
                            }
                        }
                    }
                }
            }

            Rectangle {
                id: fileListAddAttachScrollbar
                width: 6
                x: fileListAddAttachBackground.width - width - 1;
                y: 1
                height: fileListAddAttachBackground.height - 2
                radius: width/2
                clip: true

                Rectangle {
                    x: 0
                    y: fileListAddAttach.visibleArea.yPosition * fileListAddAttachScrollbar.height
                    width: parent.width
                    height: fileListAddAttach.visibleArea.heightRatio * fileListAddAttachScrollbar.height;
                    color: "#CFD1D3"
                    radius: width
                    clip: true

                    MouseArea {
                        anchors.fill: parent
                        drag.target: parent
                        drag.axis: Drag.YAxis
                        drag.minimumY: 0
                        drag.maximumY: fileListAddAttachScrollbar.height - parent.height

                        onMouseYChanged: {
                            fileListAddAttach.contentY = parent.y / fileListAddAttachScrollbar.height * fileListAddAttach.contentHeight
                        }
                    }
                }
            }
        }

        Button {
            id: selectAttachment
            x: parent.width - 2*width - 2*height
            y: parent.height - 2*height
            width: 100
            height: 30
            enabled: (fileListAddAttach.currentIndex >= 0)

            Text {
                text: qsTr("选择文件")
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
                var page = Qt.createComponent("qrc:/qml/sharingWindow.qml");
                if (page.status === Component.Ready) {
                    var sharingWin = page.createObject()
                    sharingWin.selectedSpaceIdx = spaceListAddAttach.currentIndex
                    sharingWin.selectedFileIdx = fileListAddAttach.currentIndex
                    sharingWin.addAttachmentFlag = true
                    sharingWin.mailFileSharingCreated.connect(onFileSharingCreated)
                    sharingWin.mailReceiver = addMailAttachmentWin.mailReceiver

                    sharingWin.show()

                }
            }
        }

        Button {
            id: addAttachCancel
            x: selectAttachment.x + selectAttachment.width + 30
            y: selectAttachment.y
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

            onClicked: closeAddAttachmentWin()
        }
    }

    function onFileSharingCreated() {
        addMailAttachmentWin.fileAttached(main.getFileNameByIndex(fileListAddAttach.currentIndex))
        closeAddAttachmentWin()
    }

    function closeAddAttachmentWin() {
        addMailAttachmentWin.close()
        if (addMailAttachmentWin.mainSpaceIndex >= 0) {
            main.showFileList(main.getSpaceLabelByIndex(addMailAttachmentWin.mainSpaceIndex))
        } else {
            main.showFileList(guiString.sysMsgs(GUIString.SharingSpaceLabel))
        }
    }
}
