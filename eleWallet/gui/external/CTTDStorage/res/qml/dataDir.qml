import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2
import ctt.GUIString 1.0

Window {
    id: dataDirWindow
    width: 520
    height: 150
    flags: Qt.FramelessWindowHint | Qt.Window
    title: qsTr("Set data storage path(设置数据存储路径)")
    visible: true

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton
        property point clickPosSetting: "0, 0"
        onPressed: {
            clickPosSetting = Qt.point(mouse.x, mouse.y)
        }
        onPositionChanged: {
            if (pressed) {
                var delta = Qt.point(mouse.x-clickPosSetting.x, mouse.y-clickPosSetting.y)

                dataDirWindow.setX(dataDirWindow.x+delta.x)
                dataDirWindow.setY(dataDirWindow.y+delta.y)
            }
        }
    }

    Rectangle {
        id: backGroundDataDir
        x: 0
        y: 0
        width: dataDirWindow.width
        height: dataDirWindow.height
        border.color: "#999999"

//        gradient: Gradient {
//            GradientStop{ position: 0; color: "#B4F5C9" }
//            GradientStop{ position: 1; color: "#79E89A" }
//        }

        Rectangle {
            id: titleAreaDataDir
            color: "#EFF2F6"
            border.color: "transparent"
            x: 1
            y: 1
            width: dataDirWindow.width - 2
            height: 40 - 1

            Button {
                id: closeDataDirBtn

                x: dataDirWindow.width - 35
                y: 5
                width: 30
                height: 30

                background: Rectangle {
                    implicitHeight: closeDataDirBtn.height
                    implicitWidth:  closeDataDirBtn.width

                    color: "transparent"  //transparent background

                    BorderImage {
                        property string nomerPic: "qrc:/images/login/icon-close.png"
                        property string hoverPic: "qrc:/images/login/icon-close.png"
                        property string pressPic: "qrc:/images/login/icon-close.png"

                        anchors.fill: parent
                        source: closeDataDirBtn.hovered ? (closeDataDirBtn.pressed ? pressPic : hoverPic) : nomerPic;
                    }
                }

                onClicked: quitProgram()
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
                x: 55
                y: 5
                width: 420
                height: 30
                text: qsTr("Set private data storage path(设置私密数据存储路径)")
                font.bold: true
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                font.pixelSize: 14
                color: "black"
            }
        }

        Rectangle {
            id: dataPathBox
            x: 65
            y: titleAreaDataDir.height + 10
            width: 335
            height: 24
            color: "transparent"
            border.color: "#B5C6DB"
            Text {
                id: dataPath
                text: ""
                font.pointSize: 10
                anchors.left: parent.left
                anchors.leftMargin: 5
                width: parent.width - 10
                height: parent.height
                verticalAlignment: Text.AlignVCenter
                horizontalAlignment: Text.AlignLeft
                elide: Text.ElideMiddle
            }
        }

        Text {
            x: 20
            y: dataPathBox.y
            width: 40
            height: 24
            text: qsTr("Path: ")
            verticalAlignment: Text.AlignVCenter
            horizontalAlignment: Text.AlignLeft
            font.pixelSize: 13
        }

        Button {
            id: dirBrowseBtn
            x: 410
            y: dataPathBox.y
            width: 80
            height: 24

            Text {
                text: qsTr("Browse")
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

            onClicked: dataDirFds.open()
        }

        Rectangle {
            color: "#F2F2F2"
            border.color: "transparent"
            x: 1
            y: dataDirWindow.height - titleAreaDataDir.height - 20
            width: titleAreaDataDir.width - 1
            height: 1
        }

        Button {
            id: downloadConfirmBtn
            x: 320
            y: dataDirWindow.height - titleAreaDataDir.height - 2
            width: 80
            height: 24
            enabled: dataPath.text !== ""

            Text {
                text: qsTr("Ok")
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

            onClicked: setDataDir()
        }

        Button {
            id: downloadCancelBtn
            x: 410
            y: dataDirWindow.height - titleAreaDataDir.height - 2
            width: 80
            height: 24

            Text {
                text: qsTr("Quit")
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

            onClicked: quitProgram()
        }
    }

    FileDialog {
        id: dataDirFds
        modality: Qt.WindowModal //Qt.ApplicationModal
        title: qsTr("Set private data storage path(设置私密数据存储路径)")
        folder: shortcuts.desktop
        selectExisting: true
        selectFolder: true
        selectMultiple: false
        //nameFilters: ["json文件 (*.json)"]
        onAccepted: {
            dataPath.text = main.getDirPathFromUrl(dataDirFds.fileUrl)
        }
    }

    function setDataDir() {
        if (dataPath.text !== "") {
            main.setDataDir(dataPath.text)
            dataDirWindow.close()
            main.showLoginWindow()
            main.initAccounts()

            if (main.dataDir !== "") {
                var getAgentInfoRet = main.getAgentInfo()
                if (getAgentInfoRet !== guiString.sysMsgs(GUIString.OkMsg)) {
                    // TODO: show message box and refresh as well as disable login function
                    console.log("Error: get agent infomation failed, ", getAgentInfoRet)
                    //loginBtn.enabled = false
                }
            }
        }
    }

    function quitProgram() {
        var params = []
        rpcBackend.sendOperationCmd(GUIString.ExitExternGUICmd, params)
        Qt.quit()
    }
}
