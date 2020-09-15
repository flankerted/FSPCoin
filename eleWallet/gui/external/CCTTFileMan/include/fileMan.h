#ifndef FILEMAN_H
#define FILEMAN_H

#include <QList>
#include <QStringListModel>
#include "strings.h"

typedef struct _FileInfo
{
    QString fileSpaceLabel;
    QString fileName;
    QString fileOffset;
    QString fileSize;

    bool isShared;
    QString objID;
    QString sharerAddr;
    QString sharerCSNodeID;
    QString key;
    QString shareStart;
    QString shareStop;

    _FileInfo(QString label, QString fName, QString offset, QString fSize, bool shared=false, QString objId="0",
              QString sharer="0", QString sharerCSNodeID="0", QString key="0", QString k="0", QString start="0", QString stop="0"):
        fileSpaceLabel(label), fileName(fName), fileOffset(offset), fileSize(fSize), isShared(shared), objID(objId),
        sharerAddr(sharer), sharerCSNodeID(sharerCSNodeID), key(key), shareStart(start), shareStop(stop) { };
}FileInfo;

class FileManager
{
public:
    FileManager();

    ~FileManager();

    // "qwe/104857600/4096/CTTFileMan.cbp/26486/104827018"
    // "1/1059061760/4096/FlashFXP_3452.zip/7261207/7265303/config.json/3065/7268368/123.txt/2485/1051790907"
    // "1/1059061760/4096/FlashFXP_3452.zip/7261207/7265303/config.json/3065/7268368/123.txt/2485/1051790907/{json}/sharing/"
    // "2/209715200/4096/FlashFXP_3452.txt/4971/209706133/[{\"Owner\":\"0x790113a1e87a6e845be30358827fee65e0be8a58\",\"ObjID\": 1,\"Offset\": 4096,\"Length\": 233,\"StartTime\": 1546272000,\"EndTime\": 1546272000,\"FileName\": \"staruml.txt\",\"SharerCSNodeID\": \"0c627764bbfaedba641f4a667107a66bc4703c7f03f5f62efbe67e5970fad9259b8681d3938d9cb3f22ae60d6f72ed927a58be399597d0837905fbaeafc6c9fc\"}]/sharing"
    static bool CheckHead4K(const QString& head4K);

    static bool CheckHead4K(const QString& head4K, QString& label, qint64& total, qint64& spare);

    static bool CheckSpace(qint64 spare, qint64 fSize);

    static void GetFileList(QString& head4K, QList<FileInfo>& myFiles);

    static QString UpdateMetaData(const QString& headOrig, int headLength, QString fName, qint64 fSize, qint64& startPos);

private:

};

#endif // FILEMAN_H
