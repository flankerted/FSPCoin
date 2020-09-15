import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: mainWindow
    visible: false
    width: 1080
    height: 660
    flags: Qt.FramelessWindowHint | Qt.Window
    title: qsTr(" 为陌 ")

    property int mainPage: 0
    property int spaceCategory: -1
    property int transmitCategory: 2
    property bool transmittingFlag: false
    property int sharingCategory: 0
    property bool mailAttached: false
    property string mailAttachSharingCode: ""

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton
        property point clickPos: "0, 0"
        onPressed: {
            clickPos = Qt.point(mouse.x, mouse.y)
        }
        onPositionChanged: {
            if (mouse.x > 0 && mouse.y > 0)
            {
                var delta = Qt.point(mouse.x-clickPos.x, mouse.y-clickPos.y)
                var newX = mainWindow.x + delta.x
                var newY = mainWindow.y + delta.y
                if (newX < Screen.desktopAvailableWidth-mainWindow.width &&
                        newY < Screen.desktopAvailableHeight-mainWindow.height &&
                        newX > 0 && newY > 0)
                {
                    mainWindow.setX(newX)
                    mainWindow.setY(newY)
                }
            }
        }
    }

//    Component.onCompleted: {
//        if (spaceList.currentIndex >= 0 && fileList.currentIndex >= 0) {
//            var page = Qt.createComponent("qrc:/qml/downloadDlg.qml");

//            if (page.status === Component.Ready) {
//                mainWindow.downloadDlg = page.createObject()
//                //downloadDlg.show();
//            }
//        }
//    }

    Rectangle {
        x: 0
        y: 0
        width: mainWindow.width
        height: mainWindow.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Rectangle {
            id: titleMainWin
            color: "#EEF0F6"
            border.color: "transparent"
            x: 1
            y: 1
            width: mainWindow.width - 2
            height: 80 - 1

            Button {
                id: mainWinQuitBtn

                x: mainWindow.width - 40
                y: 10
                width: 30
                height: 30

                background: Rectangle {
                    implicitHeight: mainWinQuitBtn.height
                    implicitWidth:  mainWinQuitBtn.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: mainWinQuitBtn.hovered ? (mainWinQuitBtn.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: main.preExit() //quitProgram()
            }

            Button {
                id: mainWinMiniBtn

                x: mainWindow.width - 40 - mainWinQuitBtn.width
                y: 10
                width: 30
                height: 30

                background: Rectangle {
                    implicitHeight: mainWinMiniBtn.height
                    implicitWidth:  mainWinMiniBtn.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/main/icon-minimize.png"
                        property string hoverPic: "qrc:/images/main/icon-minimize.png"
                        property string pressPic: "qrc:/images/main/icon-minimize.png"

                        anchors.fill: parent
                        source: mainWinMiniBtn.hovered ? (mainWinMiniBtn.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: doMinimized()
            }

            Button {
                id: mainWinSettingsBtn

                x: mainWindow.width - 40 - 2*mainWinQuitBtn.width
                y: 10
                width: 30
                height: 30

                background: Rectangle {
                    implicitHeight: mainWinSettingsBtn.height
                    implicitWidth:  mainWinSettingsBtn.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/main/icon-settings.png"
                        property string hoverPic: "qrc:/images/main/icon-settings.png"
                        property string pressPic: "qrc:/images/main/icon-settings.png"

                        //anchors.fill: parent
                        anchors.top: parent.top
                        anchors.topMargin: 5
                        anchors.left: parent.left
                        anchors.leftMargin: 5
                        width: parent.width - 10
                        height: parent.height - 10
                        source: mainWinSettingsBtn.hovered ? (mainWinSettingsBtn.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: main.showAgentConnWindow()
            }

            Image {
                x: 15
                y: 20
                width: 40
                height: 40
                source: "qrc:/images/main/icon-wemore.png"
                fillMode: Image.PreserveAspectFit
            }

            Text {
                id: mainTitleText
                x: 60
                y: 20
                width: 100
                height: 40
                text: qsTr("为陌")
                fontSizeMode: Text.Fit
                color: "black"
                font.italic: false
                style: Text.Sunken
                font.family: "Verdana"
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pointSize: 72
            }

            Button {
                id: myStorage
                x: 180
                y: 25
                width: 60
                height: 40

                Text {
                    text: qsTr("我的网盘")
                    anchors.top: parent.top
                    anchors.topMargin: 10
                    font.pointSize: 11
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    color: (mutiPages.currentIndex === 0 || myStorage.hovered) ? "#06A8FF" : "black"
                }

                Rectangle {
                    color: mutiPages.currentIndex === 0 ? "#06A8FF" : "transparent"
                    border.color: "transparent"
                    x: 0
                    anchors.top: parent.bottom
                    anchors.bottomMargin: 2
                    width: parent.width
                    height: 2
                }

                background: Rectangle { color: "transparent"; border.color: "transparent" }

                onClicked: { setMainPage(0); receivingMailView.opened = false }
            }

            BusyIndicator {
                id: transmitting
                x: transmitList.x - 32
                y: transmitList.y - 2
                width: 30
                height: 30
                running: mainWindow.transmittingFlag
                property bool hovering: false

                MouseArea {
                    acceptedButtons: Qt.LeftButton
                    hoverEnabled: true
                    anchors.fill: parent
                    onEntered: { if(transmitting.running) { transmitting.hovering = true } }
                    onExited: { transmitting.hovering = false }
                    onClicked: { setMainPage(1); receivingMailView.opened = false }
                }

                contentItem: Item {
                    //implicitWidth: 64
                    //implicitHeight: 64

                    Item {
                        id: transmittingItem
                        x: parent.width / 4
                        y: parent.height / 4
                        width: parent.width
                        height: parent.height
                        opacity: transmitting.running ? 0.8 : 0

                        Behavior on opacity {
                            OpacityAnimator {
                                duration: 250
                            }
                        }

                        RotationAnimator {
                            target: transmittingItem
                            running: transmitting.visible && transmitting.running
                            from: 0
                            to: 360
                            loops: Animation.Infinite
                            duration: 5000
                        }

                        Repeater {
                            id: repeaterAnim
                            model: 6

                            Rectangle {
                                x: transmittingItem.width / 2 - width / 2
                                y: transmittingItem.height / 2 - height / 2
                                implicitWidth: 4
                                implicitHeight: 4
                                radius: 2
                                color: (mutiPages.currentIndex === 1 || transmitList.hovered || transmitting.hovering) ? "#06A8FF" : "black"
                                transform: [
                                    Translate {
                                        y: -Math.min(transmittingItem.width, transmittingItem.height) * 0.5 + 2
                                    },
                                    Rotation {
                                        angle: index / repeaterAnim.count * 360
                                        origin.x: 2
                                        origin.y: 2
                                    }
                                ]
                            }
                        }
                    }
                }
            }

            Button {
                id: transmitList
                x: myStorage.x + 100
                y: myStorage.y
                width: myStorage.width
                height: myStorage.height

                Text {
                    text: qsTr("传输列表")
                    anchors.top: parent.top
                    anchors.topMargin: 10
                    font.pointSize: 11
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    color: (mutiPages.currentIndex === 1 || transmitList.hovered || transmitting.hovering) ? "#06A8FF" : "black"
                }

                Rectangle {
                    color: mutiPages.currentIndex === 1 ? "#06A8FF" : "transparent"
                    border.color: "transparent"
                    x: 0
                    anchors.top: parent.bottom
                    anchors.bottomMargin: 2
                    width: parent.width
                    height: 2
                }

                background: Rectangle { color: "transparent"; border.color: "transparent" }

                onClicked: { setMainPage(1); receivingMailView.opened = false }
            }

            Button {
                id: mySharing
                x: myStorage.x + 200
                y: myStorage.y
                width: myStorage.width
                height: myStorage.height

                Text {
                    text: qsTr("资源分享")
                    anchors.top: parent.top
                    anchors.topMargin: 10
                    font.pointSize: 11
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    color: (mutiPages.currentIndex === 2 || mySharing.hovered) ? "#06A8FF" : "black"
                }

                Rectangle {
                    color: mutiPages.currentIndex === 2 ? "#06A8FF" : "transparent"
                    border.color: "transparent"
                    x: 0
                    anchors.top: parent.bottom
                    anchors.bottomMargin: 2
                    width: parent.width
                    height: 2
                }

                background: Rectangle { color: "transparent"; border.color: "transparent" }

                onClicked: setMainPage(2)
            }

//            Button {
//                id: myMail
//                x: myStorage.x + 300
//                y: myStorage.y
//                width: myStorage.width
//                height: myStorage.height

//                Text {
//                    text: qsTr("我的邮件")
//                    anchors.top: parent.top
//                    anchors.topMargin: 10
//                    font.pointSize: 11
//                    verticalAlignment: Text.AlignVCenter
//                    horizontalAlignment: Text.AlignHCenter
//                    color: (mutiPages.currentIndex === 3 || myMail.hovered) ? "#B4F5C9" : "#06A8FF"
//                }

//                Rectangle {
//                    color: mutiPages.currentIndex === 3 ? "#B4F5C9" : "transparent"
//                    border.color: "transparent"
//                    x: 0
//                    anchors.top: parent.bottom
//                    anchors.bottomMargin: 2
//                    width: parent.width
//                    height: 2
//                }

//                background: Rectangle { color: "transparent"; border.color: "transparent" }

//                onClicked: setMainPage(3)
//            }

            Image {
                id: imgUser
                x: myStorage.x + 300
                y: 25
                source: "qrc:/images/main/icon-user.png"
                width: 30
                height: 30
            }

            Rectangle {
                id: accountState
                x: imgUser.x + imgUser.width + 10
                y: 20
                width: 420
                height: 40
                color: "transparent"

                Row {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    Text {
                        id: accountHintLabel
                        width: 40 + (main.getLanguage() === "EN" ? 20 : 0)
                        height: 20
                        text: qsTr("账号: ")
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }
                    Text {
                        id: userAccount
                        width: 360 - (main.getLanguage() === "EN" ? 20 : 0)
                        height: 20
                        text: main.account
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        color: "black"
                    }
                    Image {
                        id: imgLockState
                        source: main.lockState ? "qrc:/images/main/icon-lock.png" : "qrc:/images/main/icon-unlock.png"
                        width: 20; height: 20
                    }
                }

                Row {
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    Text {
                        id: balanceHint
                        width: 40 + (main.getLanguage() === "EN" ? 20 : 0)
                        height: 20
                        text: qsTr("余额: ")
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }
                    Text {
                        id: balanceValue
                        text: "0"
                        width: 170 - (main.getLanguage() === "EN" ? 20 : 0)
                        height: 20
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        color: "black"
                    }
                    Text {
                        text: " | "
                        width: 20
                        height: 20
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }
                    Button {
                        id: sendTransaction
                        width: 30 + (main.getLanguage() === "EN" ? 30 : 0)
                        height: 20

                        Text {
                            text: qsTr("转账")
                            anchors.fill: parent
                            font.pointSize: 11
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.underline: sendTransaction.hovered
                            color: sendTransaction.pressed ? "#059BEC" : (sendTransaction.hovered ? "#06A8FF" : "black")
                        }

                        background: Rectangle { color: "transparent"; border.color: "transparent" }

                        onClicked: {
                            var page = Qt.createComponent("qrc:/qml/sendTx.qml");
                            if (page.status === Component.Ready) {
                                page.createObject().show()
                            }
                        }
                    }
                    Text {
                        text: " | "
                        width: 20
                        height: 20
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }
                    Button {
                        id: lockUnlock
                        width: 60
                        height: 20
                        enabled: false

                        Text {
                            text: main.lockState ? qsTr("解锁账户") : qsTr("锁定账户")
                            anchors.fill: parent
                            font.pointSize: 11
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.underline: lockUnlock.hovered
                            color:  parent.enabled ? (lockUnlock.pressed ? "#059BEC" : (lockUnlock.hovered ? "#06A8FF" : "black")) : "#999999"
                        }

                        background: Rectangle { color: "transparent"; border.color: "transparent" }

                        onClicked: {
                            console.log(main.atService)
                            console.log(main.getSpaceLabelByIndex(spaceList.currentIndex))
                            console.log(!main.isCreatingSpace)
                            console.log(mainWindow.spaceCategory)
                            console.log(main.transmitting)
                            console.log(main.isRefreshingFiles)
//                            ( && (main.getSpaceLabelByIndex(spaceList.currentIndex) !== "") && !main.isCreatingSpace &&
//                              (main.getSpaceLabelByIndex(spaceList.currentIndex) !== guiString.sysMsgs(GUIString.SharingSpaceLabel)) &&
//                               mainWindow.spaceCategory !== -1 && !main.transmitting && !main.isRefreshingFiles)
                        }
                    }
                    Text {
                        text: " | "
                        width: 20
                        height: 20
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }
                    Button {
                        id: copyAccount
                        width: 60 - (main.getLanguage() === "EN" ? 30 : 0)
                        height: 20

                        Text {
                            text: qsTr("复制账号")
                            anchors.fill: parent
                            font.pointSize: 11
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.underline: copyAccount.hovered
                            color: copyAccount.pressed ? "#059BEC" : (copyAccount.hovered ? "#06A8FF" : "black")
                        }

                        background: Rectangle { color: "transparent"; border.color: "transparent" }

                        onClicked: main.copyToClipboard(main.account)
                    }
                }
            }
        }

        Rectangle {
            id: titleMainWinLine
            color: "#D6D8DD"
            border.color: "transparent"
            x: 0
            y: titleMainWin.height - 1
            width: mainWindow.width
            height: 1
        }

        SwipeView {
            id: mutiPages
            anchors.top: parent.top
            anchors.topMargin: titleMainWin.height
            width: mainWindow.width
            height: mainWindow.height - titleMainWin.height
            currentIndex: mainWindow.mainPage

            Page {
                id: mainPage1
                background: Rectangle {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    //Gradient {
                    //    GradientStop{ position: 0; color: "#487A89" }
                    //    GradientStop{ position: 1; color: "#54DAB3" }
                    //}
                }

                Rectangle {
                    id: mainLeftAreaLinePage1
                    color: "#D6D8DD"
                    border.color: "transparent"
                    x: 170
                    y: 0 //titleMainWin.height
                    width: 1
                    height: mainWindow.height - titleMainWin.height
                }

                Rectangle {
                    id: mainLeftAreabackground
                    x: 1
                    y: 0
                    width: mainLeftAreaLinePage1.x - 1
                    height: mainWindow.height - titleMainWin.height - 1
                    color: "#F9FAFB"
                }

                Rectangle {
                    id: mainBottomAreaLine
                    color: "#D6D8DD"
                    border.color: "transparent"
                    x: mainLeftAreaLinePage1.x
                    y: mutiPages.height - 40
                    width: mainWindow.width - mainLeftAreaLinePage1.x
                    height: 1
                }

                Button {
                    id: allSpace
                    x: 0
                    y: 0 //titleMainWin.height
                    width: mainLeftAreaLinePage1.x
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
                        font.bold: true
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black" //(mainWindow.spaceCategory === 0 || allSpace.hovered) ? "white" : "black"
                    }

                    Rectangle {
                        color: "transparent" //mainWindow.spaceCategory === 0 ? "#06A8FF" : "transparent"
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

                    //onClicked: { setSpace(0); spaceList.highlightFollowsCurrentItem = false }
                }

//                Rectangle {
//                    color: "#D6D8DD"
//                    border.color: "transparent"
//                    x: 0
//                    y: 2 * allSpace.height //titleMainWin.height + allSpace.height
//                    width: mainWindow.width
//                    height: 1
//                }

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
                    id: spaceList
                    x: 0
                    y: allSpace.height + 1 //titleMainWin.height + allSpace.height + 1
                    width: mainLeftAreaLinePage1.x - spaceLabelScrollbar.width - 1
                    height: 240
                    model: spaceListModel
                    highlightFollowsCurrentItem: true
                    highlightMoveDuration: 0
                    highlight: Rectangle {
                        Rectangle {
                            color: spaceList.highlightFollowsCurrentItem ? "#06A8FF" : "transparent"
                            border.color: "transparent"
                            x: 0
                            anchors.left: parent.left
                            anchors.leftMargin: 0
                            width: 4
                            height: parent.height
                        }

                        color: spaceList.highlightFollowsCurrentItem ? "#3306A8FF" : "transparent"
                    }
                    clip: true
                    focus: false

                    delegate: Row {
                        id: spaceListRow
                        property bool hovered: false
                        Rectangle {
                            width: spaceLabel.width
                            height: spaceLabel.height
                            color: "transparent"
                            Text {
                                id: spaceLabel; width: mainLeftAreaLinePage1.x; height: allSpace.height
                                anchors.left: parent.left
                                anchors.leftMargin: 55
                                verticalAlignment: Text.AlignVCenter
                                text: space
                                font.pointSize: 11
                                font.bold: true
                                color: ((spaceListRow.ListView.isCurrentItem && spaceList.highlightFollowsCurrentItem) ||
                                        spaceListRow.hovered) ? "#06A8FF" : "black"
                            }

                            MouseArea {
                                acceptedButtons: Qt.LeftButton
                                hoverEnabled: true
                                anchors.fill: parent
                                onEntered: { spaceListRow.hovered = true }
                                onExited: { spaceListRow.hovered = false }
                                onClicked: {
                                    spaceList.highlightFollowsCurrentItem = true
                                    spaceListRow.ListView.view.currentIndex = index
                                    setSpace(index+1)
                                    main.showFileList(main.getSpaceLabelByIndex(index))
                                    main.currentSpaceIdx = index
                                    main.currentFileIdx = 0
                                    //expandSpace.enabled = expandSpaceEnabled()
                                    uploadBtn.enabled = expandSpaceEnabled()
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    id: spaceLabelScrollbar
                    width: 6
                    x: mainLeftAreaLinePage1.x - width - 1;
                    y: spaceList.y + 1
                    height: spaceList.height - 2
                    radius: width/2
                    clip: true

                    Rectangle {
                        id: dragButton
                        x: 0
                        y: spaceList.visibleArea.yPosition * spaceLabelScrollbar.height
                        width: parent.width
                        height: spaceList.visibleArea.heightRatio * spaceLabelScrollbar.height;
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
                                spaceList.contentY = dragButton.y / spaceLabelScrollbar.height * spaceList.contentHeight
                            }
                        }
                    }
                }

                Rectangle {
                    color: "#D6D8DD"
                    border.color: "transparent"
                    x: 0
                    y: allSpace.height + spaceList.height //titleMainWin.height + allSpace.height + spaceList.height
                    width: mainLeftAreaLinePage1.x
                    height: 1
                }

                Button {
                    id: sharingSpace
                    x: 0
                    y: allSpace.height + spaceList.height //titleMainWin.height + allSpace.height + spaceList.height
                    width: mainLeftAreaLinePage1.x
                    height: 40

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-fromSharing.png"
                    }

                    Text {
                        text: qsTr("来自分享")
                        anchors.left: parent.left
                        anchors.leftMargin: 55
                        anchors.verticalCenter: parent.verticalCenter
                        font.pointSize: 11
                        font.bold: true
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: (mainWindow.spaceCategory === -1 || sharingSpace.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.spaceCategory === -1 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.spaceCategory === -1 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: {
                        setSpace(-1)
                        main.currentSpaceIdx = -1
                        spaceList.highlightFollowsCurrentItem = false
                        main.showFileList(guiString.sysMsgs(GUIString.SharingSpaceLabel))
                        //expandSpace.enabled = expandSpaceEnabled()
                        uploadBtn.enabled = expandSpaceEnabled()
                    }
                }

                // ProgressBar
                Rectangle { // background
                    id: spaceRemained
                    x: 10
                    y: mutiPages.height - 50 //mainWindow.height - 50
                    width: 150
                    height: 10

                    property double maximum: 100
                    property double value:   0
                    property double minimum: 0

                    border.width: 1
                    border.color: "transparent" //"#55000000"
                    color: "#E1E1E1"
                    //radius: height/2

                    Rectangle { // foreground
                        id: spaceRemainedBar
                        visible: spaceRemained.value > spaceRemained.minimum
                        x: 0.1 * spaceRemained.height;  y: 0.1 * spaceRemained.height
                        width: Math.max(height,
                               Math.min((spaceRemained.value - spaceRemained.minimum) / (spaceRemained.maximum - spaceRemained.minimum) *
                                        (spaceRemained.width - 0.2 * spaceRemained.height), spaceRemained.width - 0.2 * spaceRemained.height))
                        height: 0.8 * spaceRemained.height
                        color: "#06A8FF"
                        //radius: parent.radius
                    }
                }

                Text {
                    id: spaceInfo
                    width: spaceRemained.width
                    height: 10
                    x: 10
                    y: spaceRemained.y - height - 5
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter

                    property string used: "" //"0"
                    property string all: "" //"100"
                    text: (used !== "" && all !== "") ? (used + "/" + all) : ""
                    font.pointSize: 10
                    color: "black"
                }

                Button {
                    id: createSpace
                    x: 10
                    y: spaceRemained.y + spaceRemained.height + 8
                    width: 60
                    height: 10
                    enabled: main.atService && !main.isCreatingSpace && !main.transmitting && !main.isRefreshingFiles

                    Text {
                        text: qsTr("创建空间")
                        anchors.fill: parent
                        font.pointSize: 10
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.underline: createSpace.hovered
                        color: parent.enabled ? (parent.pressed ? "#059BEC" : (parent.hovered ? "#06A8FF" : "black")) : "#999999"
                    }

                    background: Rectangle { color: "transparent"; border.color: "transparent" }

                    onClicked: {
                        var page = Qt.createComponent("qrc:/qml/createSpace.qml");

                        if (page.status === Component.Ready) {
                            var newSpaceDlg = page.createObject()
                            newSpaceDlg.show();
                        }
                    }
                }

                Button {
                    id: expandSpace
                    x: 10 + createSpace.width + 20
                    y: spaceRemained.y + spaceRemained.height + 8
                    width: 60
                    height: 10
                    enabled: false //expandSpaceEnabled()

                    Text {
                        text: qsTr("扩展该空间")
                        anchors.fill: parent
                        font.pointSize: 10
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.underline: expandSpace.hovered
                        color: parent.enabled ? (parent.pressed ? "#059BEC" : (parent.hovered ? "#06A8FF" : "black")) : "#999999"
                    }

                    background: Rectangle { color: "transparent"; border.color: "transparent" }

                    //onClicked: console.log("main.isCreatingSpace:", main.isCreatingSpace)
                }

                AnimatedImage {
                    id: advertisment;
                    x: spaceRemained.x
                    y: sharingSpace.y + 1.5 * sharingSpace.height
                    width: 150; height: 150
                    source: main.getLanguage() === "CN" ? "qrc:/images/ad/gif-candycn.gif" : "qrc:/images/ad/gif-candyen.gif";

                    MouseArea {
                        id: adMouse
                        acceptedButtons: Qt.LeftButton
                        hoverEnabled: true
                        anchors.fill: parent
                        onEntered: { adMouse.cursorShape = Qt.PointingHandCursor }
                        onExited: { adMouse.cursorShape = Qt.ArrowCursor }
                        onClicked: Qt.openUrlExternally("https://www.contatract.org/candy/" + main.account)
                    }
                }

//                Image {
//                    id: advertisment
//                    x: spaceRemained.x
//                    y: sharingSpace.y + 1.5 * sharingSpace.height
//                    width: 150; height: 150
//                    source: "qrc:/images/ad/img-contatract.png"

//                    MouseArea {
//                        id: adMouse
//                        acceptedButtons: Qt.LeftButton
//                        hoverEnabled: true
//                        anchors.fill: parent
//                        onEntered: { adMouse.cursorShape = Qt.PointingHandCursor }
//                        onExited: { adMouse.cursorShape = Qt.ArrowCursor }
//                        onClicked: Qt.openUrlExternally("https://www.contatract.org/register")
//                    }
//                }

                Rectangle {
                    x: mainLeftAreaLinePage1.x + 1
                    y: 0
                    width: parent.width - mainLeftAreaLinePage1.x - 2
                    height: allSpace.height
                    color: "#F9FAFB"
                }

                Rectangle {
                    id: fileListTitle
                    x: mainLeftAreaLinePage1.x + 1
                    y: allSpace.height + 1 //titleMainWin.height + allSpace.height
                    width: parent.width - mainLeftAreaLinePage1.x - 2
                    height: allSpace.height - 1
                    color: "white"
                    Row {
                        Rectangle {
                            id: fileListTitle1
                            width: 420; height: allSpace.height
                            color: fileListTitle.color
                            Text {
                                anchors.left: parent.left
                                anchors.leftMargin: 20
                                verticalAlignment: Text.AlignVCenter
                                width: fileListTitle1.width
                                height: fileListTitle1.height
                                text: qsTr("文件名")
                                color: "#757880"
                                font.pointSize: 12
                            }
                            MouseArea { anchors.fill: parent; onClicked: { } }
                            Rectangle {
                                anchors.right: parent.right
                                anchors.rightMargin: 0
                                width: 1
                                height: fileListTitle1.height
                                color: "#EBEBEC"
                            }
                        }

                        Rectangle {
                            id: fileListTitle2
                            width: 330; height: allSpace.height
                            color: fileListTitle.color
                            Text {
                                verticalAlignment: Text.AlignVCenter
                                horizontalAlignment: Text.AlignHCenter
                                width: fileListTitle2.width
                                height: fileListTitle2.height
                                text: qsTr("有效期")
                                color: "#757880"
                                font.pointSize: 13
                            }
                            MouseArea { anchors.fill: parent; onClicked: { } }
                            Rectangle {
                                anchors.right: parent.right
                                anchors.rightMargin: 0
                                width: 1
                                height: fileListTitle2.height
                                color: "#EBEBEC"
                            }
                        }

                        Rectangle {
                            id: fileListTitle3
                            width: 150; height: allSpace.height
                            color: fileListTitle.color
                            Text {
                                verticalAlignment: Text.AlignVCenter
                                horizontalAlignment: Text.AlignHCenter
                                width: fileListTitle3.width
                                height: fileListTitle3.height
                                text: qsTr("大小")
                                color: "#757880"
                                font.pointSize: 13
                            }
                            MouseArea { anchors.fill: parent; onClicked: { } }
                            Rectangle {
                                anchors.right: parent.right
                                anchors.rightMargin: 0
                                width: 1
                                height: fileListTitle3.height
                                color: "#EBEBEC"
                            }
                        }
                    }
                }

                Button {
                    id: downloadBtn
                    x: mainLeftAreaLinePage1.x + 20
                    y: 5 //titleMainWin.height + 5
                    width: 80 + (main.getLanguage() === "EN" ? 20 : 0)
                    height: 30
                    enabled: main.atService && (fileList.currentIndex >= 0) && !main.transmitting && !main.isRefreshingFiles && !main.isCreatingSpace

                    background: Rectangle {
                        implicitHeight: downloadBtn.height
                        implicitWidth:  downloadBtn.width
                        color: "transparent"
                        border.color: downloadBtn.pressed ? "#E6E6E6" : (downloadBtn.hovered ? "#06A8FF" : "transparent")
                        radius: 2

                        BorderImage {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.top: parent.top
                            anchors.topMargin: 8
                            width: 14; height: 14

                            property string nomerPic: "qrc:/images/main/icon-download.png"
                            property string hoverPic: "qrc:/images/main/icon-download.png"
                            property string pressPic: "qrc:/images/main/icon-download.png"

                            source: downloadBtn.hovered ? (downloadBtn.pressed ? pressPic : hoverPic) : nomerPic;
                        }

                        Text {
                            width: parent.width - 40; height: parent.height
                            anchors.right: parent.right
                            anchors.rightMargin: 10
                            verticalAlignment: Text.AlignVCenter
                            text: qsTr("下载")
                            font.pointSize: 11
                            color: downloadBtn.enabled ? (downloadBtn.hovered ? "#06A8FF" : "black") : "#999999"
                        }
                    }

                    onClicked: {
                        if (mainWindow.spaceCategory == -1) {
                            main.showDownloadDlg(true)
                        } else if (spaceList.currentIndex >= 0 && fileList.currentIndex >= 0) {
                            main.showDownloadDlg()
                        }
                    }
                }

                Button {
                    id: uploadBtn
                    x: mainLeftAreaLinePage1.x + 20 + 1*(width + width/8)
                    y: downloadBtn.y
                    width: downloadBtn.width
                    height: downloadBtn.height
                    enabled: expandSpaceEnabled()

                    background: Rectangle {
                        implicitHeight: uploadBtn.height
                        implicitWidth:  uploadBtn.width
                        color: "transparent"
                        border.color: uploadBtn.pressed ? "#E6E6E6" : (uploadBtn.hovered ? "#06A8FF" : "transparent")
                        radius: 2

                        BorderImage {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.top: parent.top
                            anchors.topMargin: 8
                            width: 14; height: 14

                            property string nomerPic: "qrc:/images/main/icon-upload.png"
                            property string hoverPic: "qrc:/images/main/icon-upload.png"
                            property string pressPic: "qrc:/images/main/icon-upload.png"

                            source: uploadBtn.hovered ? (uploadBtn.pressed ? pressPic : hoverPic) : nomerPic;
                        }

                        Text {
                            width: parent.width - 40; height: parent.height
                            anchors.right: parent.right
                            anchors.rightMargin: 10
                            verticalAlignment: Text.AlignVCenter
                            text: qsTr("上传")
                            font.pointSize: 11
                            color: uploadBtn.enabled ? (uploadBtn.hovered ? "#06A8FF" : "black") : "#999999"
                        }
                    }

                    onClicked: {
                        uploadFds.open()
                    }
                }

                Button {
                    id: shareFileBtn
                    x: mainLeftAreaLinePage1.x + 20 + 2*(width + width/8)
                    y: downloadBtn.y
                    width: downloadBtn.width
                    height: downloadBtn.height
                    enabled: main.atService && (mainWindow.spaceCategory !== -1) && (fileList.currentIndex >= 0)

                    background: Rectangle {
                        implicitHeight: shareFileBtn.height
                        implicitWidth:  shareFileBtn.width
                        color: "transparent"
                        border.color: shareFileBtn.pressed ? "#E6E6E6" : (shareFileBtn.hovered ? "#06A8FF" : "transparent")
                        radius: 2

                        BorderImage {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.top: parent.top
                            anchors.topMargin: 8
                            width: 14; height: 14

                            property string nomerPic: "qrc:/images/main/icon-sharing.png"
                            property string hoverPic: "qrc:/images/main/icon-sharing.png"
                            property string pressPic: "qrc:/images/main/icon-sharing.png"

                            source: shareFileBtn.hovered ? (shareFileBtn.pressed ? pressPic : hoverPic) : nomerPic;
                        }

                        Text {
                            width: parent.width - 40; height: parent.height
                            anchors.right: parent.right
                            anchors.rightMargin: 10
                            verticalAlignment: Text.AlignVCenter
                            text: qsTr("分享")
                            font.pointSize: 11
                            color: shareFileBtn.enabled ? (shareFileBtn.hovered ? "#06A8FF" : "black") : "#999999"
                        }
                    }

                    onClicked: {
                        var page = Qt.createComponent("qrc:/qml/sharingWindow.qml");
                        if (page.status === Component.Ready) {
                            var sharingWin = page.createObject()
                            sharingWin.show()
                            sharingWin.selectedSpaceIdx = main.currentSpaceIdx
                            sharingWin.selectedFileIdx = main.currentFileIdx
                        }
                    }
                }

                Button {
                    id: refreshBtn
                    x: mainLeftAreaLinePage1.x + 20 + 3*(width + width/8)
                    y: downloadBtn.y
                    width: downloadBtn.width
                    height: downloadBtn.height
                    enabled: main.atService && !main.isCreatingSpace && !main.transmitting && !main.isRefreshingFiles

                    background: Rectangle {
                        implicitHeight: refreshBtn.height
                        implicitWidth:  refreshBtn.width
                        color: "transparent"
                        border.color: refreshBtn.pressed ? "#E6E6E6" : (refreshBtn.hovered ? "#06A8FF" : "transparent")
                        radius: 2

                        BorderImage {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.top: parent.top
                            anchors.topMargin: 8
                            width: 14; height: 14

                            property string nomerPic: "qrc:/images/main/icon-refresh.png"
                            property string hoverPic: "qrc:/images/main/icon-refresh.png"
                            property string pressPic: "qrc:/images/main/icon-refresh.png"

                            source: refreshBtn.hovered ? (refreshBtn.pressed ? pressPic : hoverPic) : nomerPic;
                        }

                        Text {
                            width: parent.width - 40; height: parent.height
                            anchors.right: parent.right
                            anchors.rightMargin: 10
                            verticalAlignment: Text.AlignVCenter
                            text: qsTr("刷新")
                            font.pointSize: 11
                            color: refreshBtn.enabled ? (refreshBtn.hovered ? "#06A8FF" : "black") : "#999999"
                        }
                    }

                    onClicked: {
                        refreshFiles()
                        //expandSpace.enabled = expandSpaceEnabled()
                        uploadBtn.enabled = expandSpaceEnabled()
                    }
                }

                Rectangle {
                    id: fileListBackground
                    color: "transparent"
                    border.color: "transparent"
                    x: mainLeftAreaLinePage1.x
                    y: fileListTitle.y + fileListTitle.height
                    width: fileListTitle.width
                    height: mainBottomAreaLine.y - fileListTitle.y - fileListTitle.height

                    Rectangle {
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: 0
                        y: 0
                        width: parent.width
                        height: 1
                    }

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
                        ListElement {
                            fileName: "DEWQRSAEsE"
                            timeLimit: "-"
                            fileSize: "134KB"
                        }
                        ListElement {
                            fileName: "WAWA哇哈哈"
                            timeLimit: "2019-01-01~2019-12-30"
                            fileSize: "1B"
                        }
                    }

                    ListView {
                        id: fileList
                        x: 0
                        y: 0
                        width: parent.width - fileListScrollbar.width - 2
                        height: parent.height
                        model: fileListModel //fileListModelTest
                        highlightFollowsCurrentItem: true
                        highlightMoveDuration: 0
                        highlight: Rectangle {
                            color: "#3306A8FF"
                        }
                        clip: true
                        focus: false

                        delegate: Row {
                            id: fileListRow
                            property bool hovered: false
                            Rectangle {
                                width: fileNameText.width
                                height: fileNameText.height
                                color: (fileListRow.hovered && fileListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                Text {
                                    id: fileNameText; width: fileListTitle1.width; height: allSpace.height
                                    leftPadding: 20
                                    verticalAlignment: Text.AlignVCenter
                                    text: fileName
                                    font.pointSize: 11
                                    elide: Text.ElideRight
                                    color: "black" //(fileListRow.ListView.isCurrentItem || fileListRow.hovered) ? "black" : "lightsteelblue"
                                }

                                MouseArea {
                                    acceptedButtons: Qt.LeftButton
                                    hoverEnabled: true
                                    anchors.fill: parent
                                    onEntered: { fileListRow.hovered = true }
                                    onExited: { fileListRow.hovered = false }
                                    onClicked: {
                                        fileListRow.ListView.view.currentIndex = index
                                        main.currentFileIdx = fileList.currentIndex
                                    }
                                }
                            }

                            Rectangle {
                                width: timeLimitText.width
                                height: timeLimitText.height
                                color: (fileListRow.hovered && fileListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                Text {
                                    id: timeLimitText; width: fileListTitle2.width; height: allSpace.height
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                    text: timeLimit
                                    font.pointSize: 11
                                    color: "#757880" //(fileListRow.ListView.isCurrentItem || fileListRow.hovered) ? "black" : "lightsteelblue"
                                }

                                MouseArea {
                                    acceptedButtons: Qt.LeftButton
                                    hoverEnabled: true
                                    anchors.fill: parent
                                    onEntered: { fileListRow.hovered = true }
                                    onExited: { fileListRow.hovered = false }
                                    onClicked: {
                                        fileListRow.ListView.view.currentIndex = index
                                        main.currentFileIdx = fileList.currentIndex
                                    }
                                }
                            }

                            Rectangle {
                                width: fileSizeText.width
                                height: fileSizeText.height
                                color: (fileListRow.hovered && fileListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                Text {
                                    id: fileSizeText; width: fileListTitle3.width; height: allSpace.height
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                    text: fileSize
                                    font.pointSize: 11
                                    color: "#757880" //(fileListRow.ListView.isCurrentItem || fileListRow.hovered) ? "black" : "lightsteelblue"
                                }

                                MouseArea {
                                    acceptedButtons: Qt.LeftButton
                                    hoverEnabled: true
                                    anchors.fill: parent
                                    onEntered: { fileListRow.hovered = true }
                                    onExited: { fileListRow.hovered = false }
                                    onClicked: {
                                        fileListRow.ListView.view.currentIndex = index
                                        main.currentFileIdx = fileList.currentIndex
                                    }
                                }
                            }
                        }
                    }

                    Rectangle {
                        id: fileListScrollbar
                        width: 6
                        x: fileListBackground.width - width - 1;
                        y: 1
                        height: fileListBackground.height - 2
                        radius: width/2
                        clip: true

                        Rectangle {
                            x: 0
                            y: fileList.visibleArea.yPosition * fileListScrollbar.height
                            width: parent.width
                            height: fileList.visibleArea.heightRatio * fileListScrollbar.height;
                            color: "#CFD1D3"
                            radius: width
                            clip: true

                            MouseArea {
                                anchors.fill: parent
                                drag.target: parent
                                drag.axis: Drag.YAxis
                                drag.minimumY: 0
                                drag.maximumY: fileListScrollbar.height - parent.height

                                onMouseYChanged: {
                                    fileList.contentY = parent.y / fileListScrollbar.height * fileList.contentHeight
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    id: mainPage1StatusBox
                    x: mainLeftAreaLinePage1.x + 5
                    y: mainWindow.height - allSpace.height - titleMainWin.height
                    width: mainWindow.width - x - allSpace.height
                    height: allSpace.height
                    color: "transparent"
                    border.color: "transparent"

                    BusyIndicator {
                        id: busyingInd
                        x: 0
                        y: 1
                        width: 30
                        height: 30
                        running: main.busying

                        contentItem: Item {
                            //implicitWidth: 64
                            //implicitHeight: 64

                            Item {
                                id: busyingItem
                                x: parent.width / 4
                                y: parent.height / 4
                                width: parent.width
                                height: parent.height
                                opacity: busyingInd.running ? 0.8 : 0

                                Behavior on opacity {
                                    OpacityAnimator {
                                        duration: 250
                                    }
                                }

                                RotationAnimator {
                                    target: busyingItem
                                    running: busyingInd.visible && busyingInd.running
                                    from: 0
                                    to: 360
                                    loops: Animation.Infinite
                                    duration: 5000
                                }

                                Repeater {
                                    id: busyingRepeaterAnim
                                    model: 6

                                    Rectangle {
                                        x: busyingItem.width / 2 - width / 2
                                        y: busyingItem.height / 2 - height / 2
                                        implicitWidth: 4
                                        implicitHeight: 4
                                        radius: 2
                                        color: "#BB000000"//"lightsteelblue"
                                        transform: [
                                            Translate {
                                                y: -Math.min(busyingItem.width, busyingItem.height) * 0.5 + 2
                                            },
                                            Rotation {
                                                angle: index / busyingRepeaterAnim.count * 360
                                                origin.x: 2
                                                origin.y: 2
                                            }
                                        ]
                                    }
                                }
                            }
                        }
                    }

                    Text {
                        id: mainPage1Status
                        width: parent.width - busyingInd.width
                        height: parent.height
                        text: ""
                        font.pointSize: 11
                        leftPadding: busyingInd.width + 10
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                    }
                }

                FileDialog {
                    id: uploadFds
                    modality: Qt.WindowModal //Qt.ApplicationModal
                    title: qsTr("请选择要上传的文件")
                    folder: shortcuts.desktop
                    selectExisting: true
                    selectFolder: false
                    selectMultiple: false
                    //nameFilters: ["json文件 (*.json)"]
                    onAccepted: {
                        if (spaceList.currentIndex >= 0) {
                            var filePath = uploadFds.fileUrl
                            var ret = main.uploadFileByIndex(main.getDirPathFromUrl(filePath), spaceList.currentIndex)
                            if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                                main.showMsgBox(ret, GUIString.HintDlg)
                            }
                        }
                    }
                }

                Connections {
                    target: main
                    onTransmittingChanged: {
                        mainWindow.transmittingFlag = main.transmitting
                        mainWindow.transmitCategory = category
                    }

                    onAccountChanged: {
                        userAccount.text = account
                    }

                    onBalanceChanged: {
                        balanceValue.text = balance
                    }

                    onTotalSpaceChanged: {
                        spaceRemained.maximum = (main.totalSpace === "N/A") ? 0 : spaceTotal
                        spaceRemainedBar.width = updateSpaceRemainedBar()
                        spaceInfo.all = main.totalSpace
                    }

                    onUsedSpaceChanged: {
                        spaceRemained.value = (main.usedSpace === "N/A") ? 0 : spaceUsed
                        spaceRemainedBar.width = updateSpaceRemainedBar()
                        spaceInfo.used = main.usedSpace
                    }

                    onCurrentSpaceChanged: {
                        if (main.currentSpace === guiString.sysMsgs(GUIString.SharingSpaceLabel)) {
                            setSpace(-1)
                            spaceList.highlightFollowsCurrentItem = false
                        } else {
                            spaceList.highlightFollowsCurrentItem = true
                            spaceList.currentIndex = currSpaceIndex
                            setSpace(currSpaceIndex+1)
                        }
                    }

                    onIsRefreshingFilesChanged: {
                        //expandSpace.enabled = expandSpaceEnabled()
                        uploadBtn.enabled = expandSpaceEnabled()
                    }

                    onUiStatusChanged: {
                        mainPage1Status.text = newStatus
                        //expandSpace.enabled = expandSpaceEnabled()
                        uploadBtn.enabled = expandSpaceEnabled()
                    }
                }
            }

            Page {
                id: mainPage2
                background: Rectangle {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    //Gradient {
                    //    GradientStop{ position: 0; color: "#487A89" }
                    //    GradientStop{ position: 1; color: "#54DAB3" }
                    //}
                }

                Rectangle {
                    id: mainLeftAreabackground2
                    x: 1
                    y: 0
                    width: mainLeftAreaLinePage1.x - 1
                    height: mainWindow.height - titleMainWin.height - 1
                    color: "#F9FAFB"
                }

                Rectangle {
                    id: mainLeftAreaLinePage2
                    color: "#D6D8DD"
                    border.color: "transparent"
                    x: 170
                    y: 0 //titleMainWin.height
                    width: 1
                    height: mainWindow.height - titleMainWin.height
                }

                Button {
                    id: downloadingLabel
                    x: 0
                    y: 0 //titleMainWin.height
                    width: mainLeftAreaLinePage2.x
                    height: 40

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-download.png"
                    }

                    Text {
                        text: qsTr("正在下载")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.transmitCategory === 0 || downloadingLabel.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.transmitCategory === 0 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.transmitCategory === 0 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setTransmitCat(0) }
                }

                Button {
                    id: uploadingLabel
                    x: 0
                    y: downloadingLabel.height //titleMainWin.height
                    width: mainLeftAreaLinePage2.x
                    height: downloadingLabel.height

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-upload.png"
                    }

                    Text {
                        text: qsTr("正在上传")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.transmitCategory === 1 || uploadingLabel.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.transmitCategory === 1 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.transmitCategory === 1 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setTransmitCat(1) }
                }

                Button {
                    id: completedLabel
                    x: 0
                    y: uploadingLabel.y + uploadingLabel.height //titleMainWin.height
                    width: mainLeftAreaLinePage2.x
                    height: downloadingLabel.height

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-completed.png"
                    }

                    Text {
                        text: qsTr("传输完成")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.transmitCategory === 2 || completedLabel.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.transmitCategory === 2 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.transmitCategory === 2 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setTransmitCat(2) }
                }

                Row {
                    id: allProgerss
                    x: mainLeftAreaLinePage2.x
                    y: 0
                    visible: transmitProgress.value === 0 ? false : true
                    Text {
                        id: transmitProgressLabel
                        text: mainWindow.transmitCategory === 0 ? qsTr("下载总进度") : qsTr("上传总进度")
                        width: 100
                        height: downloadingLabel.height
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        color: "black"
                    }

                    // ProgressBar
                    Rectangle { // background
                        id: transmitProgress
                        x: transmitProgressLabel.width
                        y: 10 //mainWindow.height - 50
                        width: 400
                        height: 20

                        property double maximum: 100
                        property double value:   0
                        property double minimum: 0

                        border.width: 1
                        border.color: "transparent"
                        color: "#E1E1E1"
                        //radius: height/2

                        Rectangle { // foreground
                            id: alreadyTransmitBar
                            visible: transmitProgress.value > transmitProgress.minimum
                            x: 0.1 * transmitProgress.height;  y: 0.1 * transmitProgress.height
                            width: Math.max(height,
                                            Math.min((transmitProgress.value - transmitProgress.minimum) / (transmitProgress.maximum - transmitProgress.minimum) *
                                                     (transmitProgress.width - 0.2 * transmitProgress.height), transmitProgress.width - 0.2 * transmitProgress.height))
                            height: 0.8 * transmitProgress.height
                            color: "#06A8FF"
                            //radius: parent.radius
                        }

                        Connections {
                            target: main
                            onTransmitRateChanged: {
                                transmitProgress.value = rate
                                if (rate === 100) {
                                    transmitProgress.value = 0
                                }
                            }
                        }
                    }
                }
            }

            Page {
                id: mainPage3
                background: Rectangle {
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.horizontalCenter: parent.horizontalCenter
                    //Gradient {
                    //    GradientStop{ position: 0; color: "#487A89" }
                    //    GradientStop{ position: 1; color: "#54DAB3" }
                    //}
                }

                Rectangle {
                    id: mainLeftAreabackground3
                    x: 1
                    y: 0
                    width: mainLeftAreaLinePage1.x - 1
                    height: mainWindow.height - titleMainWin.height - 1
                    color: "#F9FAFB"
                }

                Rectangle {
                    id: mainLeftAreaLinePage3
                    color: "#D6D8DD"
                    border.color: "transparent"
                    x: 170
                    y: 0 //titleMainWin.height
                    width: 1
                    height: mainWindow.height - titleMainWin.height
                }

                Button {
                    id: getSharing
                    x: 0
                    y: 0
                    width: mainLeftAreaLinePage3.x
                    height: 40

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-sharing.png"
                    }

                    Text {
                        text: qsTr("获取分享")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.sharingCategory === 0 || getSharing.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.sharingCategory === 0 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.sharingCategory === 0 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setSharingCat(0); receivingMailView.opened = false }
                }

                Button {
                    id: sendingMail
                    x: 0
                    y: getSharing.height
                    width: mainLeftAreaLinePage3.x
                    height: getSharing.height

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-recvMail.png"
                    }

                    Text {
                        text: qsTr("收取邮件")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.sharingCategory === 1 || sendingMail.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.sharingCategory === 1 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.sharingCategory === 1 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setSharingCat(1); refreshMails() }
                }

                Button {
                    id: receivingMail
                    x: 0
                    y: sendingMail.y + getSharing.height
                    width: mainLeftAreaLinePage3.x
                    height: getSharing.height

                    BorderImage {
                        anchors.left: parent.left
                        anchors.leftMargin: 20
                        anchors.verticalCenter: parent.verticalCenter
                        width: 20
                        height: 20
                        source: "qrc:/images/main/icon-sendMail.png"
                    }

                    Text {
                        text: qsTr("发送邮件")
                        anchors.fill: parent
                        font.pointSize: 11
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        leftPadding: 20 + 25
                        color: (mainWindow.sharingCategory === 2 || receivingMail.hovered) ? "#06A8FF" : "black"
                    }

                    Rectangle {
                        color: mainWindow.sharingCategory === 2 ? "#06A8FF" : "transparent"
                        border.color: "transparent"
                        x: 0
                        anchors.left: parent.left
                        anchors.leftMargin: 0
                        width: 4
                        height: parent.height
                    }

                    background: Rectangle {
                        color: mainWindow.sharingCategory === 2 ? "#3306A8FF" : "transparent"
                        border.color: "transparent"
                    }

                    onClicked: { setSharingCat(2); receivingMailView.opened = false }
                }

                Rectangle {
                    id: getSharingView
                    x: mainLeftAreaLinePage3.x
                    y: 0
                    width: mainWindow.width - mainLeftAreaLinePage3.x
                    height: mainWindow.height - titleMainWin.height
                    color: "transparent"
                    border.color: "transparent"
                    visible: mainWindow.sharingCategory == 0

                    Text {
                        id: typeSharingCodeHint
                        x: 20
                        y: 50
                        height: 20
                        text: qsTr("请将文件的分享码复制到下面的输入框中, 然后可以查看到该文件的相关分享信息:")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Rectangle {
                        id: sharingCodeInputBox
                        x: 20
                        y: typeSharingCodeHint.y + 30
                        width: 560
                        height: 210
                        color: "transparent"
                        border.color: "#D6D8DD"

                        TextInput {
                            id: sharingCodeInput
                            anchors.left: parent.left
                            anchors.leftMargin: 4
                            anchors.top: parent.top
                            anchors.topMargin: 4
                            width: parent.width - 8
                            height: parent.height - 8
                            text: ""
                            cursorVisible: true
                            anchors.fill: parent
                            font.pixelSize: 14
                            wrapMode: Text.Wrap
                            selectByMouse: true
                            selectedTextColor: "white"
                            selectionColor: "#06A8FF"

                            onTextChanged: {
                                if (text !== "") {
                                    var ret = main.getSharingInfo(text)
                                    if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
                                        sharingCodeInvalidHint.visible = true

                                        sharer.text = ""
                                        sharingFileName.text = ""
                                        sharingTimeLimitStart.text = ""
                                        sharingTimeLimitStop.text = ""
                                        sharingPrice.text = ""
                                    } else {
                                        sharingCodeInvalidHint.visible = false
                                    }
                                }
                            }
                        }
                    }

                    Text {
                        id: sharingCodeInvalidHint
                        x: 20
                        y: sharingCodeInputBox.y + sharingCodeInputBox.height + 10
                        height: 20
                        visible: false
                        text: qsTr("你输入的文件分享码无效, 请再次确认")
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                        font.pixelSize: 14
                    }

                    Button {
                        id: payforSharingFile
                        x: 620
                        y: sharingCodeInputBox.y + sharingCodeInputBox.height - 30
                        width: 100 + (main.getLanguage() === "EN" ? 10 : 0)
                        height: 30
                        enabled: (!sharingCodeInvalidHint.visible && (sharingCodeInput.text !== ""))

                        Text {
                            text: qsTr("获取分享")
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
                            var ret = main.payforSharing(sharingCodeInput.text)
                            main.showMsgBox(ret, GUIString.HintDlg,
                                            ret.indexOf(guiString.uiHints(GUIString.PayForSharingSuccHint)) >= 0)
                        }
                    }

                    Button {
                        id: clearSharingCode
                        x: 770 - (main.getLanguage() === "EN" ? 20 : 0)
                        y: payforSharingFile.y
                        width: 100 + (main.getLanguage() === "EN" ? 10 : 0)
                        height: 30

                        Text {
                            text: qsTr("清空输入")
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
                            sharer.text = ""
                            sharingFileName.text = ""
                            sharingTimeLimitStart.text = ""
                            sharingTimeLimitStop.text = ""
                            sharingPrice.text = ""
                            sharingCodeInput.text = ""
                        }
                    }

                    Text {
                        id: sharerHint
                        x: 620
                        y: sharingCodeInputBox.y
                        height: 20
                        text: qsTr("分享者: ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        id: sharer
                        x: sharerHint.x + 60
                        y: sharingCodeInputBox.y
                        height: 20
                        text: ""
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14

                        Connections {
                            target: main
                            onSharingInfoSharerChanged: {
                                sharer.text = main.sharingInfoSharer
                            }
                        }
                    }

                    Text {
                        id: sharingFileHint
                        x: 620
                        y: sharerHint.y + 1.5 * sharerHint.height
                        height: 20
                        text: qsTr("文件名: ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        id: sharingFileName
                        x: sharerHint.x + 60
                        y: sharingFileHint.y
                        height: 20
                        text: ""
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14

                        Connections {
                            target: main
                            onSharingInfoFileNameChanged: {
                                sharingFileName.text = main.sharingInfoFileName
                            }
                        }
                    }

                    Text {
                        id: sharingTimeLimitHint
                        x: 620
                        y: sharerHint.y + 3 * sharerHint.height
                        height: 20
                        text: qsTr("有效期: ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        x: 620 + 20
                        y: sharerHint.y + 4.5 * sharerHint.height
                        height: 20
                        text: qsTr("从 ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        id: sharingTimeLimitStart
                        x: 620 + 20 + 20
                        y: sharerHint.y + 4.5 * sharerHint.height
                        height: 20
                        text: ""
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14

                        Connections {
                            target: main
                            onSharingInfoStartTimeChanged: {
                                sharingTimeLimitStart.text = main.sharingInfoStartTime
                            }
                        }
                    }

                    Text {
                        x: 620 + 20
                        y: sharerHint.y + 6 * sharerHint.height
                        height: 20
                        text: qsTr("到 ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        id: sharingTimeLimitStop
                        x: 620 + 20 + 20
                        y: sharerHint.y + 6 * sharerHint.height
                        height: 20
                        text: ""
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14

                        Connections {
                            target: main
                            onSharingInfoStopTimeChanged: {
                                sharingTimeLimitStop.text = main.sharingInfoStopTime
                            }
                        }
                    }

                    Text {
                        id: sharingPriceHint
                        x: 620
                        y: sharerHint.y + 7.5 * sharerHint.height
                        height: 20
                        text: qsTr("分享价格: ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14
                    }

                    Text {
                        id: sharingPrice
                        x: sharingPriceHint.x + 70
                        y: sharerHint.y + 7.5 * sharerHint.height
                        height: 20
                        text: ""
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: 14

                        Connections {
                            target: main
                            onSharingInfoPriceChanged: {
                                sharingPrice.text = main.sharingInfoPrice
                            }
                        }
                    }
                }

                Rectangle {
                    id: receivingMailView
                    x: mainLeftAreaLinePage3.x
                    y: 0
                    width: mainWindow.width - mainLeftAreaLinePage3.x
                    height: mainWindow.height - titleMainWin.height
                    color: "transparent"
                    border.color: "transparent"
                    visible: mainWindow.sharingCategory == 1

                    property bool opened: false

                    Rectangle {
                        x: 1
                        y: 0
                        width: parent.width - 1
                        height: allSpace.height
                        color: "#F9FAFB"
                    }

                    Rectangle {
                        id: mailListTopLine
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: 0
                        y: 2 * allSpace.height
                        width: parent.width
                        height: 1
                    }

                    Rectangle {
                        id: mailListBottomLine
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: 0
                        y: parent.height - allSpace.height
                        width: parent.width
                        height: 1
                        visible: mailListTopLine.visible
                    }

                    Button {
                        id: refreshMailsBtn
                        x: 20 + 0*(width + width/8)
                        y: 5
                        width: 80
                        height: 30
                        enabled: !receivingMailView.opened

                        background: Rectangle {
                            implicitHeight: refreshMailsBtn.height
                            implicitWidth:  refreshMailsBtn.width
                            color: "transparent"
                            border.color: parent.pressed ? "#E6E6E6" : (parent.hovered ? "#06A8FF" : "transparent")
                            radius: 2

                            BorderImage {
                                anchors.left: parent.left
                                anchors.leftMargin: 10
                                anchors.top: parent.top
                                anchors.topMargin: 8
                                width: 14; height: 14

                                property string nomerPic: "qrc:/images/main/icon-refresh.png"
                                property string hoverPic: "qrc:/images/main/icon-refresh.png"
                                property string pressPic: "qrc:/images/main/icon-refresh.png"

                                source: refreshMailsBtn.hovered ? (refreshMailsBtn.pressed ? pressPic : hoverPic) : nomerPic;
                            }

                            Text {
                                width: parent.width - 40; height: parent.height
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                verticalAlignment: Text.AlignVCenter
                                text: qsTr("刷新")
                                font.pointSize: 11
                                color: parent.enabled ? (parent.hovered ? "#06A8FF" : "black") : "#999999"
                            }
                        }

                        onClicked: refreshMails()
                    }

                    Button {
                        id: openMailBtn
                        x: 20 + 1*(width + width/8)
                        y: refreshMailsBtn.y
                        width: refreshMailsBtn.width
                        height: refreshMailsBtn.height
                        enabled: !receivingMailView.opened

                        background: Rectangle {
                            implicitHeight: openMailBtn.height
                            implicitWidth:  openMailBtn.width
                            color: "transparent"
                            border.color: parent.pressed ? "#E6E6E6" : (parent.hovered ? "#06A8FF" : "transparent")
                            radius: 2

                            BorderImage {
                                anchors.left: parent.left
                                anchors.leftMargin: 10
                                anchors.top: parent.top
                                anchors.topMargin: 8
                                width: 14; height: 14

                                property string nomerPic: "qrc:/images/main/icon-read.png"
                                property string hoverPic: "qrc:/images/main/icon-read.png"
                                property string pressPic: "qrc:/images/main/icon-read.png"

                                source: openMailBtn.hovered ? (openMailBtn.pressed ? pressPic : hoverPic) : nomerPic;
                            }

                            Text {
                                width: parent.width - 40; height: parent.height
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                verticalAlignment: Text.AlignVCenter
                                text: qsTr("查看")
                                font.pointSize: 11
                                color: parent.enabled ? (parent.hovered ? "#06A8FF" : "black") : "#999999"
                            }
                        }

                        onClicked: readMailDetails(main.currentMailIdx)
                    }

                    Button {
                        id: returnToListBtn
                        x: 20 + 2*(width + width/8)
                        y: refreshMailsBtn.y
                        width: refreshMailsBtn.width
                        height: refreshMailsBtn.height
                        enabled: receivingMailView.opened

                        background: Rectangle {
                            implicitHeight: returnToListBtn.height
                            implicitWidth:  returnToListBtn.width
                            color: "transparent"
                            border.color: parent.pressed ? "#E6E6E6" : (parent.hovered ? "#06A8FF" : "transparent")
                            radius: 2

                            BorderImage {
                                anchors.left: parent.left
                                anchors.leftMargin: 10
                                anchors.top: parent.top
                                anchors.topMargin: 8
                                width: 14; height: 14

                                property string nomerPic: "qrc:/images/main/icon-return.png"
                                property string hoverPic: "qrc:/images/main/icon-return.png"
                                property string pressPic: "qrc:/images/main/icon-return.png"

                                source: returnToListBtn.hovered ? (returnToListBtn.pressed ? pressPic : hoverPic) : nomerPic;
                            }

                            Text {
                                width: parent.width - 40; height: parent.height
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                verticalAlignment: Text.AlignVCenter
                                text: qsTr("返回")
                                font.pointSize: 11
                                color: parent.enabled ? (parent.hovered ? "#06A8FF" : "black") : "#999999"
                            }
                        }

                        onClicked: { receivingMailView.opened = false; mailListTopLine.visible = true; }
                    }

                    Button {
                        id: getAttachmentBtn
                        x: 20 + 3*(width + width/8)
                        y: refreshMailsBtn.y
                        width: refreshMailsBtn.width
                        height: refreshMailsBtn.height
                        enabled: mainWindow.mailAttached

                        background: Rectangle {
                            implicitHeight: getAttachmentBtn.height
                            implicitWidth:  getAttachmentBtn.width
                            color: "transparent"
                            border.color: parent.pressed ? "#E6E6E6" : (parent.hovered ? "#06A8FF" : "transparent")
                            radius: 2

                            BorderImage {
                                anchors.left: parent.left
                                anchors.leftMargin: 10
                                anchors.top: parent.top
                                anchors.topMargin: 8
                                width: 14; height: 14

                                property string nomerPic: "qrc:/images/main/icon-attachment.png"
                                property string hoverPic: "qrc:/images/main/icon-attachment.png"
                                property string pressPic: "qrc:/images/main/icon-attachment.png"

                                source: getAttachmentBtn.hovered ? (getAttachmentBtn.pressed ? pressPic : hoverPic) : nomerPic;
                            }

                            Text {
                                width: parent.width - 40; height: parent.height
                                anchors.right: parent.right
                                anchors.rightMargin: 10
                                verticalAlignment: Text.AlignVCenter
                                text: qsTr("附件")
                                font.pointSize: 11
                                color: parent.enabled ? (parent.hovered ? "#06A8FF" : "black") : "#999999"
                            }
                        }

                        onClicked: getAttachment()
                    }

                    Rectangle {
                        id: mailListTitle
                        x: 1
                        y: 40
                        width: parent.width - 1
                        height: 40
                        color: "white"
                        visible: !receivingMailView.opened
                        Row {
                            Rectangle {
                                id: mailListTitle1
                                width: 240; height: mailListTitle.height
                                color: mailListTitle.color
                                Text {
                                    anchors.left: parent.left
                                    anchors.leftMargin: 20
                                    verticalAlignment: Text.AlignVCenter
                                    width: mailListTitle1.width
                                    height: mailListTitle1.height
                                    text: qsTr("发件人 ")
                                    font.pointSize: 12
                                    color: "#757880"
                                }
                                MouseArea { anchors.fill: parent; onClicked: { } }
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.rightMargin: 0
                                    width: 1
                                    height: mailListTitle1.height
                                    color: "#EBEBEC"
                                }
                            }

                            Rectangle {
                                id: mailListTitle2
                                width: 490; height: mailListTitle.height
                                color: mailListTitle.color
                                Text {
                                    verticalAlignment: Text.AlignVCenter
                                    leftPadding: 20
                                    width: mailListTitle2.width
                                    height: mailListTitle2.height
                                    text: qsTr("标题")
                                    font.pointSize: 13
                                    color: "#757880"
                                }
                                MouseArea { anchors.fill: parent; onClicked: { } }
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.rightMargin: 0
                                    width: 1
                                    height: mailListTitle2.height
                                    color: "#EBEBEC"
                                }
                            }

                            Rectangle {
                                id: mailListTitle3
                                width: 170; height: mailListTitle.height
                                color: mailListTitle.color
                                Text {
                                    verticalAlignment: Text.AlignVCenter
                                    horizontalAlignment: Text.AlignHCenter
                                    width: mailListTitle3.width
                                    height: mailListTitle3.height
                                    text: qsTr("发送时间")
                                    font.pointSize: 13
                                    color: "#757880"
                                }
                                MouseArea { anchors.fill: parent; onClicked: { } }
                                Rectangle {
                                    anchors.right: parent.right
                                    anchors.rightMargin: 0
                                    width: 1
                                    height: mailListTitle3.height
                                    color: "#EBEBEC"
                                }
                            }
                        }
                    }

                    Rectangle {
                        id: mailListBackground
                        color: "transparent"
                        border.color: "transparent"
                        x: 0
                        y: mailListTitle.y + mailListTitle.height
                        width: mailListTitle.width
                        height: parent.height - mailListTitle.y - mailListTitle.height - 40
                        visible: !receivingMailView.opened

                        ListModel {
                            id: mailListModelTest
                            ListElement {
                                sender: "0x5f663f10...81f594a0eb"
                                title: "Aqe21qwdeqww"
                                timeStamp: "2019-01-01 00:05:00"
                                attached: ""
                            }
                            ListElement {
                                sender: "0x790113A1...65E0BE8A58"
                                title: "Bcvbzvcvzv"
                                timeStamp: "-"
                                attached: "setaet.txt"
                            }
                            ListElement {
                                sender: "0x5f663f10...81f594a0eb"
                                title: "Cdfasdfadf"
                                timeStamp: "2019-01-01 00:05:00"
                                attached: ""
                            }
                            ListElement {
                                sender: "0x790113A1...65E0BE8A58"
                                title: "DEWQRSAEsE"
                                timeStamp: "-"
                                attached: ""
                            }
                            ListElement {
                                sender: "0x5f663f10...81f594a0eb"
                                title: "WAWA哇哈哈"
                                timeStamp: "2019-01-01 00:05:00"
                                attached: "哇哈哈.txt"
                            }
                        }

                        ListView {
                            id: mailList
                            x: 0
                            y: 0
                            width: parent.width - mailListScrollbar.width - 2
                            height: parent.height
                            model: mailListModel //mailListModelTest
                            highlightFollowsCurrentItem: true
                            highlightMoveDuration: 0
                            highlight: Rectangle {
                                color: "#3306A8FF"
                            }
                            clip: true
                            focus: false

                            delegate: Row {
                                id: mailListRow
                                property bool hovered: false
                                Rectangle {
                                    width: senderText.width
                                    height: senderText.height
                                    color: (mailListRow.hovered && mailListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                    Text {
                                        id: senderText; width: mailListTitle1.width; height: mailListTitle.height
                                        leftPadding: 20
                                        verticalAlignment: Text.AlignVCenter
                                        text: sender
                                        elide: Text.ElideRight
                                        font.pointSize: 11
                                        color: "black"
                                    }

                                    MouseArea {
                                        acceptedButtons: Qt.LeftButton
                                        hoverEnabled: true
                                        anchors.fill: parent
                                        onEntered: { mailListRow.hovered = true }
                                        onExited: { mailListRow.hovered = false }
                                        onClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                        }
                                        onDoubleClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                            readMailDetails(index)
                                        }
                                    }
                                }

                                Rectangle {
                                    width: titleText.width + attachFlagImg.width + 5
                                    height: titleText.height
                                    color: (mailListRow.hovered && mailListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                    Text {
                                        id: titleText; width: mailListTitle2.width - attachFlagImg.width - 5; height: mailListTitle.height
                                        leftPadding: 20
                                        verticalAlignment: Text.AlignVCenter
                                        text: title
                                        elide: Text.ElideRight
                                        font.pointSize: 11
                                        color: "black"
                                    }

                                    Image {
                                        id: attachFlagImg; width: 20; height: 20
                                        anchors.right: parent.right
                                        anchors.rightMargin: 5
                                        anchors.top: parent.top
                                        anchors.topMargin: 10
                                        source: "qrc:/images/download/icon-file.png"
                                        fillMode: Image.PreserveAspectFit
                                        visible: (attached !== "")
                                    }

                                    MouseArea {
                                        acceptedButtons: Qt.LeftButton
                                        hoverEnabled: true
                                        anchors.fill: parent
                                        onEntered: { mailListRow.hovered = true }
                                        onExited: { mailListRow.hovered = false }
                                        onClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                        }
                                        onDoubleClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                            readMailDetails(index)
                                        }
                                    }
                                }

                                Rectangle {
                                    width: timeStampText.width
                                    height: timeStampText.height
                                    color: (mailListRow.hovered && mailListRow.ListView.view.currentIndex !== index) ? "#3306A8FF" : "transparent"
                                    Text {
                                        id: timeStampText; width: mailListTitle3.width; height: mailListTitle.height
                                        horizontalAlignment: Text.AlignHCenter
                                        verticalAlignment: Text.AlignVCenter
                                        text: timeStamp
                                        font.pointSize: 11
                                        color: "#757880"
                                    }

                                    MouseArea {
                                        acceptedButtons: Qt.LeftButton
                                        hoverEnabled: true
                                        anchors.fill: parent
                                        onEntered: { mailListRow.hovered = true }
                                        onExited: { mailListRow.hovered = false }
                                        onClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                        }
                                        onDoubleClicked: {
                                            mailListRow.ListView.view.currentIndex = index
                                            mainWindow.mailAttached = (attached !== "")
                                            main.currentMailIdx = mailList.currentIndex
                                            readMailDetails(index)
                                        }
                                    }
                                }
                            }
                        }

                        Rectangle {
                            id: mailListScrollbar
                            width: 6
                            x: mailListBackground.width - width - 1;
                            y: 1
                            height: mailListBackground.height - 2
                            radius: width/2
                            clip: true

                            Rectangle {
                                x: 0
                                y: mailList.visibleArea.yPosition * mailListScrollbar.height
                                width: parent.width
                                height: mailList.visibleArea.heightRatio * mailListScrollbar.height;
                                color: "#CFD1D3"
                                radius: width
                                clip: true

                                MouseArea {
                                    anchors.fill: parent
                                    drag.target: parent
                                    drag.axis: Drag.YAxis
                                    drag.minimumY: 0
                                    drag.maximumY: mailListScrollbar.height - parent.height

                                    onMouseYChanged: {
                                        mailList.contentY = parent.y / mailListScrollbar.height * mailList.contentHeight
                                    }
                                }
                            }
                        }
                    }

                    Rectangle {
                        id: mailDetailsBox
                        x: 0
                        y: 50
                        width: parent.width
                        height: parent.height - y
                        color: "transparent"
                        border.color: "transparent"
                        visible: receivingMailView.opened

                        Text {
                            id: mailTitle
                            x: 20
                            y: 0
                            height: 30
                            text: ""
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.pixelSize: 20
                        }

                        Text {
                            id: mailSenderLabel
                            x: 20
                            y: mailTitle.y + mailTitle.height + 10
                            width: 60
                            height: 20
                            text: qsTr("发件人: ")
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.pixelSize: 14
                        }

                        Text {
                            id: mailSender
                            x: mailSenderLabel.x + mailSenderLabel.width
                            y: mailSenderLabel.y
                            width: 320
                            height: 20
                            text: ""
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignLeft
                            elide: Text.ElideRight
                            font.pixelSize: 14
                        }

                        Text {
                            id: mailTimeStampLabel
                            x: 20
                            y: mailSenderLabel.y + mailSenderLabel.height + 10
                            height: 20
                            width: 60
                            text: qsTr("时  间: ")
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.pixelSize: 14
                        }

                        Text {
                            id: mailTimeStamp
                            x: mailTimeStampLabel.x + mailTimeStampLabel.width
                            y: mailSenderLabel.y + mailSenderLabel.height + 10
                            height: 20
                            text: ""
                            verticalAlignment: Text.AlignVCenter
                            horizontalAlignment: Text.AlignHCenter
                            font.pixelSize: 14
                        }

                        Rectangle {
                            id: contentLine
                            color: "#D6D8DD"
                            border.color: "transparent"
                            x: 0
                            y: mailTimeStampLabel.y + mailTimeStampLabel.height + 10 - 1
                            width: parent.width
                            height: 1
                        }

                        Rectangle {
                            id: mailContentBox
                            x: 20
                            y: mailTimeStampLabel.y + mailTimeStampLabel.height + 20
                            width: parent.width - 2 * 20
                            height: parent.height - y - 50
                            color: "transparent"
                            border.color: "#D6D8DD"
                            clip: true

                            MouseArea{
                                anchors.fill: parent
                                onWheel: {
                                    if (wheel.angleDelta.y > 0 && mailContentTextScrollBarDragBtn.y >= 5) {
                                        mailContentTextScrollBarDragBtn.y -= 5;
                                        mailContentText.y = 4 - mailContentTextScrollBarDragBtn.y / mailContentTextScrollBar.height * mailContentText.contentHeight
                                    }
                                    else if(wheel.angleDelta.y < 0 && (mailContentTextScrollBarDragBtn.y + mailContentTextScrollBarDragBtn.height <= mailContentTextScrollBar.height - 5)) {
                                        mailContentTextScrollBarDragBtn.y += 5;
                                        mailContentText.y = 4 - mailContentTextScrollBarDragBtn.y / mailContentTextScrollBar.height * mailContentText.contentHeight
                                    }
                                }
                            }

                            TextEdit {
                                id: mailContentText
//                                anchors.left: parent.left
//                                anchors.leftMargin: 4
//                                anchors.top: parent.top
//                                anchors.topMargin: 4
                                width: parent.width - 8 - mailContentTextScrollBar.width - 1
                                height: contentHeight //parent.height - 8
                                x: 4
                                y: 4// - mailContentTextScrollBar.position * mailContentText.height
                                text: ""
                                font.pixelSize: 14
                                wrapMode: Text.Wrap
                                selectByKeyboard: true
                                selectByMouse: true
                                selectedTextColor: "white"
                                selectionColor: "#06A8FF"
                                readOnly: true
                            }

                            Rectangle {
                                id: mailContentTextScrollBar
                                width: 6
                                x: parent.width - width - 1;
                                y: 4
                                height: parent.height - 8
                                radius: width/2
                                clip: true

                                Rectangle {
                                    id: mailContentTextScrollBarDragBtn
                                    x: 0
                                    y: 0
                                    width: parent.width
                                    height: (mailContentBox.height - 8) / (mailContentText.height > (mailContentBox.height - 8) ? mailContentText.height: (mailContentBox.height - 8)) * mailContentTextScrollBar.height
                                    color: "#CFD1D3"
                                    radius: width
                                    visible: mailContentText.height > (mailContentBox.height - 8)
                                    clip: true

                                    MouseArea {
                                        anchors.fill: parent
                                        drag.target: parent
                                        drag.axis: Drag.YAxis
                                        drag.minimumY: 0
                                        drag.maximumY: mailContentTextScrollBar.height - parent.height

                                        onMouseYChanged: {
                                            mailContentText.y = 4 - parent.y / mailContentTextScrollBar.height * mailContentText.contentHeight
                                        }
                                    }
                                }
                            }
                        }
                    }


                }

                Rectangle {
                    id: sendingMailView
                    x: mainLeftAreaLinePage3.x
                    y: 0
                    width: mainWindow.width - mainLeftAreaLinePage3.x
                    height: mainWindow.height - titleMainWin.height
                    color: "transparent"
                    border.color: "transparent"
                    visible: mainWindow.sharingCategory == 2

                    Text {
                        id: mailTitleLabel
                        x: 20
                        y: 20
                        width: 60
                        height: 20
                        text: qsTr("标  题  : ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14
                    }

                    TextInput {
                        id: mailTitleEdit
                        x: mailTitleLabel.x + mailTitleLabel.width
                        y: mailTitleLabel.y
                        width: 360
                        height: 20
                        text: ""
                        leftPadding: 5
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14
                    }

                    Rectangle {
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: mailTitleLabel.x + mailTitleLabel.width
                        y: mailTitleLabel.y + mailTitleLabel.height
                        width: 360
                        height: 1
                    }

                    Text {
                        id: mailReceiverLabel
                        x: 20
                        y: mailTitleLabel.y + mailTitleLabel.height + 10
                        width: 60
                        height: 20
                        text: qsTr("收件人  :")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14
                    }

                    TextInput {
                        id: mailReceiverEdit
                        x: mailReceiverLabel.x + mailReceiverLabel.width
                        y: mailReceiverLabel.y
                        width: 360
                        height: 20
                        text: ""
                        leftPadding: 5
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14

                        onTextChanged: clearMailAttachmentFile()
                    }

                    Rectangle {
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: mailReceiverLabel.x + mailReceiverLabel.width
                        y: mailReceiverLabel.y + mailReceiverLabel.height
                        width: 360
                        height: 1
                    }

                    Text {
                        id: mailAttachmentLabel
                        x: 20
                        y: mailReceiverLabel.y + mailReceiverLabel.height + 10
                        height: 20
                        width: 60
                        text: qsTr("分享文件: ")
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14
                    }

                    Text {
                        id: mailAttachment
                        x: mailAttachmentLabel.x + mailAttachmentLabel.width
                        y: mailReceiverLabel.y + mailReceiverLabel.height + 10
                        width: 180
                        height: 20
                        text: ""
                        leftPadding: 5
                        verticalAlignment: Text.AlignVCenter
                        horizontalAlignment: Text.AlignLeft
                        font.pixelSize: 14
                        elide: Text.ElideRight
                    }

                    Button {
                        id: addMailAttachment
                        x: mailAttachment.x + mailAttachment.width + 10
                        y: mailAttachmentLabel.y - 3
                        width: 80
                        height: 26
                        enabled: (mailAttachment.text === "")

                        Text {
                            text: qsTr("添加文件")
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
                            var page = Qt.createComponent("qrc:/qml/addAttachment.qml");

                            if (page.status === Component.Ready) {
                                var addAttachWin = page.createObject()
                                addAttachWin.mainSpaceIndex = mainWindow.spaceCategory - 1
                                addAttachWin.mailReceiver = mailReceiverEdit.text
                                addAttachWin.fileAttached.connect(onFileAttached)
                                addAttachWin.show();

                                main.currentFileIdx = 0
                            }
                        }
                    }

                    Button {
                        id: clearMailAttachment
                        x: addMailAttachment.x + addMailAttachment.width + 10
                        y: mailAttachmentLabel.y - 3
                        width: 80
                        height: 26
                        enabled: (mailAttachment.text !== "")

                        Text {
                            text: qsTr("清  除")
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

                        onClicked: clearMailAttachmentFile()
                    }

                    Rectangle {
                        color: "#D6D8DD"
                        border.color: "transparent"
                        x: 0
                        y: mailAttachmentLabel.y + mailAttachmentLabel.height + 10 - 1
                        width: parent.width
                        height: 1
                    }

                    Rectangle {
                        id: mailContentEditBox
                        x: 20
                        y: mailAttachmentLabel.y + mailAttachmentLabel.height + 20
                        width: parent.width - 2 * 20
                        height: parent.height - y - 90
                        color: "transparent"
                        border.color: "#D6D8DD"
                        TextEdit {
                            id: mailContentTextEdit
                            anchors.left: parent.left
                            anchors.leftMargin: 4
                            anchors.top: parent.top
                            anchors.topMargin: 4
                            width: parent.width - 8
                            height: parent.height - 8
                            cursorVisible: true
                            text: ""
                            font.pixelSize: 14
                            wrapMode: Text.Wrap
                            selectByMouse: true
                            selectedTextColor: "white"
                            selectionColor: "#06A8FF"
                        }
                    }

                    Button {
                        id: sendMail
                        x: parent.width - mailAttachmentLabel.x - width //(parent.width - width) / 2
                        y: mailContentEditBox.y + mailContentEditBox.height + 30
                        width: 100
                        height: 30
                        enabled: (mailTitleEdit.text !== "" && mailReceiverEdit.text !== "")

                        Text {
                            text: qsTr("发  送")
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
                            var ret = main.sendNewMail(mailTitleEdit.text, mailReceiverEdit.text, mailContentTextEdit.text,
                                                       mainWindow.mailAttachSharingCode)
                            main.showMsgBox(ret, GUIString.HintDlg,
                                            ret.indexOf(guiString.uiHints(GUIString.MailSuccessfullySentHint)) >= 0)
                            mailTitleEdit.text = ""
                            mailReceiverEdit.text = ""
                            mailContentTextEdit.text = ""
                        }
                    }
                }
            }

        }
    }

    onVisibilityChanged: {
        if (mainWindow.visibility !== Window.Minimized) {
            flags = Qt.Window | Qt.FramelessWindowHint
        }
    }

    function doMinimized() {
        if (!main.isWin32) {
            flags = Qt.Window | Qt.WindowFullscreenButtonHint | Qt.CustomizeWindowHint | Qt.WindowMinimizeButtonHint
        }
        mainWindow.visibility = Window.Minimized
    }

    function expandSpaceEnabled() {
        return (main.atService && (main.getSpaceLabelByIndex(spaceList.currentIndex) !== "") && !main.isCreatingSpace &&
                (main.getSpaceLabelByIndex(spaceList.currentIndex) !== guiString.sysMsgs(GUIString.SharingSpaceLabel)) &&
                 mainWindow.spaceCategory !== -1 && !main.transmitting && !main.isRefreshingFiles)
    }

    function updateSpaceRemainedBar() {
        return Math.max(0, Math.min((spaceRemained.value - spaceRemained.minimum)
                                    / (spaceRemained.maximum - spaceRemained.minimum) *
                                    (spaceRemained.width - 0.2 * spaceRemained.height),
                                    spaceRemained.width - 0.2 * spaceRemained.height))
    }

    function setMainPage(index) {
        mainWindow.mainPage = index
        mutiPages.currentIndex = index
    }

    function setSpace(index) {
        mainWindow.spaceCategory = index
    }

    function refreshFiles() {
        if (spaceList.highlightFollowsCurrentItem === true) {
            var spaceCategory = mainWindow.spaceCategory
            main.updateFileList(main.getSpaceLabelByIndex(spaceList.currentIndex))
            spaceList.currentIndex = spaceCategory - 1
            setSpace(spaceCategory)
            spaceList.highlightFollowsCurrentItem = true
        } else if(mainWindow.spaceCategory === -1) {
            main.updateFileList(guiString.sysMsgs(GUIString.SharingSpaceLabel))
            setSpace(-1)
            spaceList.highlightFollowsCurrentItem = false
        } else {
            main.updateFileList()
        }

        //console.log("main.atService," ,main.atService, "\nmain.getSpaceLabelByIndex(spaceList.currentIndex),",
        //            main.getSpaceLabelByIndex(spaceList.currentIndex), "\nmainWindow.spaceCategory,", mainWindow.spaceCategory)

        //( !== guiString.sysMsgs(GUIString.SharingSpaceLabel)) && mainWindow.spaceCategory !== -1)
    }


    function setTransmitCat(index) {
        mainWindow.transmitCategory = index
    }


    function setSharingCat(index) {
        mainWindow.sharingCategory = index
    }

    function refreshMails() {
        main.showMsgBox(guiString.uiHints(GUIString.MailsGettingHint), GUIString.WaitingDlg)
        var ret = main.refreshMails()
        if (ret !== guiString.sysMsgs(GUIString.OkMsg)) {
            main.closeMsgBox()
            main.showMsgBox(ret, GUIString.HintDlg)
        } else {
            main.closeMsgBox()
            main.currentMailIdx = 0
        }
    }

    function readMailDetails(index) {
        mailListTopLine.visible = false;
        receivingMailView.opened = true
        mailContentTextScrollBarDragBtn.y = 0
        mailContentText.y = 4

        mailTitle.text = main.getMailTitle(main.currentMailIdx)
        mailSender.text = main.getMailSender(main.currentMailIdx)
        mailTimeStamp.text = main.getMailTimeStamp(main.currentMailIdx)
        mailContentText.text = main.getMailContent(main.currentMailIdx)
    }

    function getAttachment() {
        var page = Qt.createComponent("qrc:/qml/getAttachment.qml");
        if (page.status === Component.Ready) {
            page.createObject().show();
        }
    }

    function clearMailAttachmentFile() {
        mailAttachment.text = ""
        mainWindow.mailAttachSharingCode = ""
    }

    function onFileAttached(fileName) {
        mailAttachment.text = fileName
        mainWindow.mailAttachSharingCode = main.sharingCode
    }


    function quitProgram() {
        var params = []
        rpcBackend.sendOperationCmd(GUIString.ExitExternGUICmd, params)
        Qt.quit()
    }
}
