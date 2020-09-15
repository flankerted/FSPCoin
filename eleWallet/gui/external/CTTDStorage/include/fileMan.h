#ifndef FILEMAN_H
#define FILEMAN_H

#include <QList>
#include <QStringListModel>
#include <QJsonParseError>
#include <QJsonObject>
//#include "strings.h"

// {\"Sharing\":\"false\",\"ObjId\":1,\"Size\":256}/{\"Sharing\":\"false\",\"ObjId\":2,\"Size\":128}/{\"Sharing\":\"false\",\"Owner\":\"0x5f663f10F12503Cb126Eb5789A9B5381f594A0eB\",\"ObjId\":3,\"Offset\":\"64\",\"Length\":\"128\",\"StartTime\":\"2019-01-01 00:00:00\",\"StopTime\":\"2020-01-01 00:00:00\",\"FileName\":\"123.txt\",\"SharerCSNodeID\":\"...\"}
typedef struct _ObjInfo
{
    bool isSharing;

    QString objectID;
    QString objectSize;

    QString owner;
    QString offset;
    QString length;
    QString startTime;
    QString stopTime;
    QString fileName;
    QString sharerCSNodeID;
    QString key;

    void DecodeQJson(QString jsonStr)
    {
        QJsonParseError err;
        QJsonDocument jsonDoc = QJsonDocument::fromJson(jsonStr.toUtf8(), &err);
        if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
        {
            QJsonObject json = jsonDoc.object();
            isSharing = json["Sharing"].toBool(false);

            objectID = json["ObjId"].toString();
            objectSize= json["Size"].toString();

            owner = json["Owner"].toString();
            offset = json["Offset"].toString();
            length = json["Length"].toString();
            startTime = json["StartTime"].toString();
            stopTime = json["StopTime"].toString();
            fileName = json["FileName"].toString();
            sharerCSNodeID = json["SharerCSNodeID"].toString();
            key = json["Key"].toString();
        }
    }

}ObjInfo;

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
              QString sharer="0", QString sharerCSNodeID="0", QString key="0", QString start="0", QString stop="0"):
        fileSpaceLabel(label), fileName(fName), fileOffset(offset), fileSize(fSize), isShared(shared), objID(objId),
        sharerAddr(sharer), sharerCSNodeID(sharerCSNodeID), key(key), shareStart(start), shareStop(stop) { }
}FileInfo;

typedef struct _FileSharingInfo
{
    qint64 objID;
    QString fileName;
    qint64 fileOffset;
    qint64 fileSize;
    QString receiver;
    QString timeStart;
    QString timeStop;
    QString price;
    QString sharerAgentNodeID;
    QString key;

    QString signature;

    void SetValues(qint64 objId, QString name, qint64 offset, qint64 size, QString rcver,
                   QString tStart, QString tStop, QString p, QString agent, QString k, QString sig="0")
    {
        objID = objId;
        fileName= name;
        fileOffset = offset;
        fileSize = size;
        receiver = rcver;
        timeStart = tStart;
        timeStop = tStop;
        price = p;
        sharerAgentNodeID = agent;
        key = k;
        signature = sig;
    }

    void GetValues(qint64& objId, QString& name, qint64& offset, qint64& size, QString& rcver,
                   QString& tStart, QString& tStop, QString& p, QString& agent, QString& k, QString& sig)
    {
        objId = objID;
        name = fileName;
        offset = fileOffset;
        size = fileSize;
        rcver = receiver;
        tStart = timeStart;
        tStop = timeStop;
        p = price;
        agent = sharerAgentNodeID;
        k = key;
        sig = signature;
    }

    QString GetSignature()
    {
        return signature;
    }

    QString GetShareAgentNodeID()
    {
        return sharerAgentNodeID;
    }

    QString EncodeQJson()
    {
        QJsonObject json;
        json.insert("ObjectID", objID);
        json.insert("FileName", fileName);
        json.insert("FileOffset", fileOffset);
        json.insert("FileSize", fileSize);
        json.insert("Receiver", receiver);
        json.insert("TimeStart", timeStart);
        json.insert("TimeStop", timeStop);
        json.insert("Price", price);
        json.insert("SharerCSNodeID", sharerAgentNodeID);
        json.insert("Key", key);
        json.insert("Signature", signature);

        QJsonDocument jsonDoc(json);
        return jsonDoc.toJson(QJsonDocument::Compact);
    }

    void DecodeQJson(QString jsonStr)
    {
        QJsonParseError err;
        QJsonDocument jsonDoc = QJsonDocument::fromJson(jsonStr.toUtf8(), &err);
        if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
        {
            QJsonObject json = jsonDoc.object();
            objID = json["ObjectID"].toInt();
            fileName= json["FileName"].toString();
            fileOffset = json["FileOffset"].toInt();
            fileSize = json["FileSize"].toInt();
            receiver = json["Receiver"].toString();
            timeStart = json["TimeStart"].toString();
            timeStop = json["TimeStop"].toString();
            price = json["Price"].toString();
            sharerAgentNodeID = json["SharerCSNodeID"].toString();
            key = json["Key"].toString();
            signature = json["Signature"].toString();
        }
    }

}FileSharingInfo;

class FileManager
{
public:
    FileManager();

    ~FileManager();

    static bool ParseObjList(QString& objListStr, QList<FileInfo>& sharings,
                             std::map<qint64, std::pair<qint64, QString> >& objInfos);

    static bool ParseMetaData(const QString& metaData, qint64 objID, qint64& spaceTotal, qint64& spaceRemain,
                              std::map<qint64, std::pair<qint64, QString> >& objInfos,
                              std::vector<std::pair<QList<FileInfo>, qint64> >& vecFileList,
                              std::map<QString, std::pair<qint64, qint64> >& spaceInfos=m_EmptySpaceInfos);

    // "qwe/104857600/4096/CTTFileMan.cbp/26486/104827018"
    // "1/1059061760/4096/FlashFXP_3452.zip/7261207/7265303/config.json/3065/7268368/123.txt/2485/1051790907"
    // "1/1059061760/4096/FlashFXP_3452.zip/7261207/7265303/config.json/3065/7268368/123.txt/2485/1051790907/{json}/sharing/"
    // "2/209715200/4096/FlashFXP_3452.txt/4971/209706133/[{\"Owner\":\"0x790113a1e87a6e845be30358827fee65e0be8a58\",\"ObjID\": 1,\"Offset\": 4096,\"Length\": 233,\"StartTime\": 1546272000,\"EndTime\": 1546272000,\"FileName\": \"staruml.txt\",\"SharerCSNodeID\": \"0c627764bbfaedba641f4a667107a66bc4703c7f03f5f62efbe67e5970fad9259b8681d3938d9cb3f22ae60d6f72ed927a58be399597d0837905fbaeafc6c9fc\"}]/sharing"
    static bool CheckHead4K(const QString& head4K);

    static bool CheckHead4K(const QString& head4K, QString& label, qint64& total, qint64& spare);

    static bool CheckSpace(qint64 spare, qint64 fSize);

    static void GetFileList(const QString& head4K, QList<FileInfo>& myFiles);

    static QString UpdateMetaData(const QString& headOrig, int headLength, QString fName, qint64 fSize, qint64& startPos);

private:
    static std::map<QString, std::pair<qint64, qint64> > m_EmptySpaceInfos;
};

// For "spaceList" in mainWindow.qml only
class QSpaceListModel : public QAbstractListModel
{
    Q_OBJECT

public:
    enum QSpaceModelRole
    {
        SpaceLabelRole = Qt::UserRole + 10
    };

    QSpaceListModel(QObject* parent = nullptr);

    ~QSpaceListModel() override;

    int rowCount(const QModelIndex& parent = QModelIndex()) const override;

    QVariant data(const QModelIndex& index, int role) const override;

    virtual QHash<int, QByteArray> roleNames() const override;

    Qt::ItemFlags flags(const QModelIndex& index) const override;

    //bool setData(const QModelIndex& index, const QVariant& value, int role) override;

    bool insertRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    //bool removeRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    bool clearData(const QModelIndex& parent = QModelIndex());

public:
    void SetModel(const std::map<qint64, std::pair<qint64, QString> >& objInfos);

    QString GetSpaceLabel(int index);

    int GetSpaceIndex(QString spaceLabel);

private:
    QList<QString> m_SpaceList;

};

// For "fileList" in mainWindow.qml only
class QFileListModel : public QAbstractListModel
{
    Q_OBJECT

public:
    enum QFileModelRole
    {
        FileNameRole = Qt::UserRole + 20,
        TimeLimitRole,
        FileSizeRole
    };

    QFileListModel(QObject* parent = nullptr);

    ~QFileListModel() override;

    int rowCount(const QModelIndex& parent = QModelIndex()) const override;

    QVariant data(const QModelIndex& index, int role) const override;

    virtual QHash<int, QByteArray> roleNames() const override;

    Qt::ItemFlags flags(const QModelIndex& index) const override;

    //bool setData(const QModelIndex& index, const QVariant& value, int role) override;

    bool insertRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    //bool removeRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

public:
    bool ClearData(const QModelIndex& parent = QModelIndex());

    void AddModel(const QList<FileInfo>& files);

    void SetModel(const QList<FileInfo>& files);

    QString GetFileName(int index);

private:
    QList<FileInfo> m_FileList;

};

#endif // FILEMAN_H
