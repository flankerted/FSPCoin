cmake_minimum_required(VERSION 2.8.11)
project(CTTFileMan)

IF(NOT CMAKE_BUILD_TYPE)
SET(CMAKE_BUILD_TYPE Release)
ENDIF()

MESSAGE("Build type: " ${CMAKE_BUILD_TYPE})

if (${CMAKE_BUILD_TYPE} EQUAL "Release")
    set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS}  -Wall  -O3 -march=native ")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall   -O3 -march=native")
else()
    set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS}  -Wall -march=native ")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -march=native")
endif()

# Find includes in corresponding build directories
set(CMAKE_INCLUDE_CURRENT_DIR ON)

# Check C++11 or C++0x support
include(CheckCXXCompilerFlag)
CHECK_CXX_COMPILER_FLAG("-std=c++11" COMPILER_SUPPORTS_CXX11)
if(COMPILER_SUPPORTS_CXX11)
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -std=c++11")
    add_definitions(-DCOMPILEDWITHC11)
    message(STATUS "Using flag -std=c++11.")
else()
    message(FATAL_ERROR "The compiler ${CMAKE_CXX_COMPILER} has no C++11 support.")
endif()

#LIST(APPEND CMAKE_MODULE_PATH ${PROJECT_SOURCE_DIR}/cmake_modules)

# Find Qt installation
# Find the QtWidgets library
find_package(Qt5Widgets CONFIG REQUIRED)
find_package(Qt5Network REQUIRED)
message(STATUS "Using QT ${Qt5Widgets_VERSION}")

include_directories(
    ${PROJECT_SOURCE_DIR}
    ${PROJECT_SOURCE_DIR}/include
)

qt5_wrap_ui(ui_HEADERS ui/mainwindow.ui ui/dataDirDlg.ui ui/signAgentDlg.ui ui/sharingFileDlg.ui ui/newMailDlg.ui ui/newMailAttachDlg.ui)
qt5_wrap_cpp(ui_HEADERS_MOC include/mainwindow.h include/dataDirDlg.h include/signAgentDlg.h include/sharingFileDlg.h include/newMailDlg.h include/newMailAttachDlg.h include/eleWalletBackRPC.h)

add_library(${PROJECT_NAME}
    src/utils.cpp
    src/mainwindow.cpp
    src/config.cpp
    src/dataDirDlg.cpp
    src/fileMan.cpp
    src/agentDetails.cpp
    src/signAgentDlg.cpp
    src/sharingFileDlg.cpp
    src/eleWalletBackRPC.cpp
    src/mailsTab.cpp
    src/newMailDlg.cpp
    src/newMailAttachDlg.cpp
    ${ui_HEADERS_MOC}
    ${ui_HEADERS}
)

target_link_libraries(${PROJECT_NAME}
    Qt5::Widgets
    Qt5::Network
)

# Build
add_executable(CCTTFileMan
    src/main.cpp
)
target_link_libraries(CCTTFileMan ${PROJECT_NAME})

