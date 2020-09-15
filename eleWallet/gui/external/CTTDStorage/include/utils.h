#ifndef UTILS_HPP
#define UTILS_HPP

#include <vector>
#include <string>
#include <QString>
#include <QObject>
#include <QJsonObject>
#include <QJsonDocument>
#include <QJsonArray>
#include "fileMan.h"
#include "guiStrings.h"

namespace Utils
{

typedef struct _OperationCmdReq
{
    QString OpCmd;
    QList<QString> OpParams;
    QString OpIndex;

    void SetValues(QString opCmd, const QList<QString>& opParams, int idx)
    {
        OpCmd = opCmd;
        OpParams = opParams;
        OpIndex = QString::number(idx);
    }

    QString EncodeQJson()
    {
        QJsonObject json;
        json.insert("OpCmd", OpCmd);
        json.insert("OpIndex", OpIndex);

        QJsonArray jsons;
        for (int i = 0; i < OpParams.length(); ++i)
            jsons.append(OpParams[i]);

        json.insert("OpParams", jsons);

        QJsonDocument jsonDoc(json);
        return jsonDoc.toJson(QJsonDocument::Compact);
    }

}OperationCmdReq;

typedef struct _BackendMsg
{
    QString MsgType;
    QString OpCmd;
    QString OpRet;
    QString OpIndex;

    bool IsOpRspMsg()
    {
        return (MsgType.compare(GUIString::GetSingleton()->sysMsgs(GUIString::OpRspMsg)) == 0);
    }

    bool IsRateMsg()
    {
        return (MsgType.compare(GUIString::GetSingleton()->sysMsgs(GUIString::RateMsg)) == 0);
    }

    void GetValues(QString& msgType, QString& opCmd, QString& opRet, QString& opInx)
    {
        msgType = MsgType;
        opCmd = OpCmd;
        opRet = OpRet;
        opInx = OpIndex;
    }

    QString GetMsgType() const
    {
        return MsgType;
    }

    QString GetOpCmd() const
    {
        return OpCmd;
    }

    QString GetOpRet() const
    {
        return OpRet;
    }

    int GetOpIndex() const
    {
        return OpIndex.toInt();
    }

    void DecodeQJson(QString jsonStr)
    {
        QJsonParseError err;
        QJsonDocument jsonDoc = QJsonDocument::fromJson(jsonStr.toUtf8(), &err);
        if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
        {
            QJsonObject json = jsonDoc.object();
            MsgType = json["MsgType"].toString();
            OpCmd = json["OpCmd"].toString();
            OpRet = json["OpRet"].toString();
            OpIndex = json["OpIndex"].toString();
        }
    }

}BackendMsg;

extern void CalculateTime(QString yearStr, QString monthStr, QString dayStr, QString time, bool isStartTime,
                   QString* startTime, QString* stopTime);

extern QString AutoSizeString(qint64 size, qint64& unit);

extern qint64 GetSpaceObjID(const std::map<qint64, std::pair<qint64, QString> >* objInfos, QString spaceLabel);

extern qint64 GetFileOffsetLen(const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList,
                               QString spaceLabel, QString fName, qint64& offset, qint64& length,
                               QString& sharerAddr, QString& sharerAgentNodeID, QString& key);

qint64 GetFileCompleteInfo(const std::vector<std::pair<QList<FileInfo>, qint64> > *fileList,
                           const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                           QString fName, QString spaceName, qint64& offset, qint64& length, QString& sharerAddr,
                           QString& sharerAgentNodeID, QString& key, bool isSharing=false);

QString GenMetaDataJson(QString account, qint64 id, QString metaData);

bool ParseMetaDataJson(const QString& metaDataJson, QString& account, qint64& id, QString& metaData);

QString AddReformatMetaDataJson(QString account, qint64 id, QString oldMetaData, QString metaData);

QString GenReformatMetaDataJsons(QString account, const std::map<qint64, QString>& idMetaData);

bool ParseReformatMetaDataJsons(const QString& metaDataJsons, QString account, std::map<qint64, QString>& idMetaData);

}

#endif // UTILS_HPP
