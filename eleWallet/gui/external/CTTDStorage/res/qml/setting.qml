import QtQuick 2.10
import QtQuick.Window 2.10
import QtQuick.Controls 2.10
import QtQuick.Controls.Styles 1.4
import QtQuick.Dialogs 1.2

Window {
    id: settingWindow
    width: 240
    height: 140
    flags: Qt.FramelessWindowHint | Qt.Window
    title: qsTr("设置")

    MouseArea { //为窗口添加鼠标事件
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton
        property point clickPosSetting: "0, 0"
        onPressed: {
            clickPosSetting = Qt.point(mouse.x, mouse.y)
        }
        onPositionChanged: {
            if (pressed) {
                var delta = Qt.point(mouse.x-clickPosSetting.x, mouse.y-clickPosSetting.y)

                settingWindow.setX(settingWindow.x+delta.x)
                settingWindow.setY(settingWindow.y+delta.y)
            }
        }
    }

    Rectangle {
        id: backGroundSetting
        x: 0
        y: 0
        width: 240
        height: 140

        gradient: Gradient {
            GradientStop{ position: 0; color: "#487A89" }
            GradientStop{ position: 1; color: "#54DAB3" }
        }

        Button {
            id: closeSettingBtn

            x: 200
            y: 10
            width: 30
            height: 30

            background: Rectangle {
                implicitHeight: closeSettingBtn.height
                implicitWidth:  closeSettingBtn.width

                color: "transparent"  //transparent background

                BorderImage {
                    property string nomerPic: "qrc:/images/login/icon-close.png"
                    property string hoverPic: "qrc:/images/login/icon-close.png"
                    property string pressPic: "qrc:/images/login/icon-close.png"

                    anchors.fill: parent
                    source: closeSettingBtn.hovered ? (closeSettingBtn.pressed ? pressPic : hoverPic) : nomerPic;
                }
            }

            onClicked: settingWindow.close()
        }
    }
}
