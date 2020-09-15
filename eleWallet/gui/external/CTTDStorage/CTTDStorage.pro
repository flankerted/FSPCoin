QT += quick
QT += network
greaterThan(QT_MAJOR_VERSION, 4): QT += widgets
CONFIG += c++11
CONFIG += qtquickcompiler

# The following define makes your compiler emit warnings if you use
# any Qt feature that has been marked deprecated (the exact warnings
# depend on your compiler). Refer to the documentation for the
# deprecated API to know how to port your code away from it.
DEFINES += QT_DEPRECATED_WARNINGS

# You can also make your code fail to compile if it uses deprecated APIs.
# In order to do so, uncomment the following line.
# You can also select to disable deprecated APIs only up to a certain version of Qt.
#DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

SOURCES += \
    cttDStorage.cpp \
    src/eleWalletBackRPC.cpp \
    src/utils.cpp \
    src/mainLoop.cpp \
    src/uiConfig.cpp \
    src/guiStrings.cpp \
    src/agentDetails.cpp \
    src/fileMan.cpp

RESOURCES += res/CTTDStorage.qrc

macx: {
    TARGET = wemore
    ICON = res/images/main/icn-wemore.icns
} else {
    TARGET = CNTDStorage
    RC_ICONS = res/images/main/icon-wemore.ico
}

TRANSLATIONS = en_US.ts

# Additional import path used to resolve QML modules in Qt Creator's code model
QML_IMPORT_PATH =

# Additional import path used to resolve QML modules just for Qt Quick Designer
QML_DESIGNER_IMPORT_PATH =

# Default rules for deployment.
qnx: target.path = /tmp/$${TARGET}/bin
else: unix:!android: target.path = /opt/$${TARGET}/bin
!isEmpty(target.path): INSTALLS += target

INCLUDEPATH += include

HEADERS += \
    include/eleWalletBackRPC.h \
    include/utils.h \
    include/mainLoop.h \
    include/uiConfig.h \
    include/guiStrings.h \
    include/agentDetails.h \
    include/fileMan.h

#DISTFILES += \
#    res/qml/popup.qml \
#    res/qml/newAcc.qml \
#    res/qml/mainwindow.qml \
#    res/qml/login.qml \
#    res/qml/downloadDlg.qml \
#    res/qml/dataDir.qml \
#    res/qml/agentConn.qml \
#    res/qml/timeSelector.qml \
#    res/qml/createSpace.qml \
#    res/qml/sharingWindow.qml \
#    res/qml/sendTx.qml \
#    res/qml/getAttachment.qml \
#    res/qml/addAttachment.qml
