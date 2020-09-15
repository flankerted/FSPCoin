#include <iostream>
#include "utils.h"

namespace Utils
{

void CalculateTime(QString yearStr, QString monthStr, QString dayStr, QString time, bool isStartTime,
                   QString* startTime, QString* stopTime)
{
    if (monthStr.toInt() < 10)
        monthStr = QString("0") + monthStr;
    if (dayStr.toInt() < 10)
        dayStr = QString("0") + dayStr;

    if (isStartTime)
        *startTime = yearStr + QString("-") + monthStr + QString("-") + dayStr + QString(" ") + time;
    else
        *stopTime = yearStr + QString("-") + monthStr + QString("-") + dayStr + QString(" ") + time;
}

QString AutoSizeString(qint64 size, qint64& unit)
{
    QString sizedStr;
    if (unit != 0) {
        if (unit == GB_Bytes)
            sizedStr = QString::number(size / GB_Bytes) + QString(" GB");
        else if (unit == MB_Bytes)
            sizedStr = QString::number(size / MB_Bytes) + QString(" MB");
        else if(unit == KB_Bytes)
            sizedStr = QString::number(size / KB_Bytes) + QString(" KB");
        else
            sizedStr = QString::number(size) + QString(" B ");
        return sizedStr;
    }

    if (size / GB_Bytes > 0)
    { sizedStr = QString::number(size / GB_Bytes) + QString(" GB"); unit = GB_Bytes; }
    else if (size / MB_Bytes > 0)
    { sizedStr = QString::number(size / MB_Bytes) + QString(" MB"); unit = MB_Bytes; }
    else if(size / KB_Bytes > 0)
    { sizedStr = QString::number(size / KB_Bytes) + QString(" KB"); unit = KB_Bytes; }
    else
    { sizedStr = QString::number(size) + QString(" B "); unit = Bytes; }
    return sizedStr;
}

qint64 GetSpaceObjID(const std::map<qint64, std::pair<qint64, QString> >* objInfos, QString spaceLabel)
{
    qint64 objID = 0;
    std::map<qint64, std::pair<qint64, QString> >::const_iterator iter;
    for (iter = objInfos->begin(); iter != objInfos->end(); iter++)
    {
        if (iter->second.second.compare(spaceLabel) == 0)
            objID = iter->first;
    }
    return objID;
}

qint64 GetFileOffsetLen(const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList,
                        QString spaceLabel, QString fName, qint64& offset, qint64& length,
                        QString& sharerAddr, QString& sharerAgentNodeID, QString& key)
{
    for(size_t i = 0; i < fileList->size(); i++)
    {
        if ((*fileList)[i].first.length() > 0)
        {
            if ((*fileList)[i].first.at(0).fileSpaceLabel.compare(spaceLabel) != 0)
                continue;
            else
            {
                for(int j = 0; j < (*fileList)[i].first.length(); j++)
                {
                    if ((*fileList)[i].first.at(j).fileName.compare(fName) == 0)
                    {
                        offset = (*fileList)[i].first.at(j).fileOffset.toLongLong();
                        length = (*fileList)[i].first.at(j).fileSize.toLongLong();
                        sharerAddr = (*fileList)[i].first.at(j).sharerAddr;
                        sharerAgentNodeID = (*fileList)[i].first.at(j).sharerCSNodeID;
                        key = (*fileList)[i].first.at(j).key;
                        return (*fileList)[i].first.at(j).objID.toLongLong();
                    }
                }
            }
        }
    }

    return 0;
}

qint64 GetFileCompleteInfo(const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList,
                           const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                           QString fName, QString spaceName, qint64& offset, qint64& length,
                           QString& sharerAddr, QString& sharerAgentNodeID, QString& key, bool isSharing)
{
    qint64 objID = Utils::GetSpaceObjID(objInfos, spaceName);
    if (!isSharing && objID == 0)
        return 0;

    if (isSharing)
        objID = Utils::GetFileOffsetLen(fileList, spaceName, fName, offset, length, sharerAddr, sharerAgentNodeID, key);
    else
        Utils::GetFileOffsetLen(fileList, spaceName, fName, offset, length, sharerAddr, sharerAgentNodeID, key);

    return objID;
}

QString GenMetaDataJson(QString account, qint64 id, QString metaData)
{
    QJsonObject json;
    json.insert("Acc", account);
    json.insert("Id", QString::number(id));
    json.insert("Data", metaData);

    QJsonDocument jsonDoc(json);
    return jsonDoc.toJson(QJsonDocument::Compact);
}

bool ParseMetaDataJson(const QString& metaDataJson, QString& account, qint64& id, QString& metaData)
{
    QJsonParseError err;
    QJsonDocument jsonDoc = QJsonDocument::fromJson(metaDataJson.toUtf8(), &err);
    if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
    {
        QJsonObject json = jsonDoc.object();
        account = json["Acc"].toString();
        id= json["Id"].toString().toLongLong();
        metaData = json["Data"].toString();
        return true;
    }
    else
        return false;
}

QString AddReformatMetaDataJson(QString account, qint64 id, QString oldMetaData, QString metaData)
{
    if (!oldMetaData.isEmpty())
    {
        std::map<qint64, QString> idMetaData;
        if (ParseReformatMetaDataJsons(oldMetaData, account, idMetaData))
        {
            if (idMetaData.find(id) != idMetaData.end())
            {
                idMetaData.erase(idMetaData.find(id));
                oldMetaData = GenReformatMetaDataJsons(account, idMetaData);
            }
        }
    }

    QJsonObject json;
    json.insert("Acc", account);
    json.insert("Id", QString::number(id));
    json.insert("Data", metaData);

    QJsonDocument jsonDoc(json);
    return oldMetaData.isEmpty() ?
                (jsonDoc.toJson(QJsonDocument::Compact)) : (oldMetaData + CTTUi::spec2 + jsonDoc.toJson(QJsonDocument::Compact));
}

QString GenReformatMetaDataJsons(QString account, const std::map<qint64, QString>& idMetaData)
{
    QString metaDatajson = "";
    for (std::map<qint64, QString>::const_iterator it = idMetaData.begin(); it != idMetaData.end(); it++)
    {
        QJsonObject json;
        json.insert("Acc", account);
        json.insert("Id", QString::number(it->first));
        json.insert("Data", it->second);

        QJsonDocument jsonDoc(json);
        metaDatajson = metaDatajson.isEmpty() ?
                    (jsonDoc.toJson(QJsonDocument::Compact)) : (metaDatajson + CTTUi::spec2 + jsonDoc.toJson(QJsonDocument::Compact));
    }

    return metaDatajson;
}

bool ParseReformatMetaDataJsons(const QString& metaDataJsons, QString account, std::map<qint64, QString>& idMetaData)
{
    QStringList list = metaDataJsons.split(CTTUi::spec2);
    if (list.length() == 0)
        return false;
    idMetaData.clear();

    for (int i = 0; i < list.length(); i++)
    {
        QJsonParseError err;
        QJsonDocument jsonDoc = QJsonDocument::fromJson(list[i].toUtf8(), &err);
        if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
        {
            QJsonObject json = jsonDoc.object();
            if (account.compare(json["Acc"].toString()) == 0)
                idMetaData[json["Id"].toString().toLongLong()] = json["Data"].toString();
        }
        else
            continue;
    }

    return true;
}

}
