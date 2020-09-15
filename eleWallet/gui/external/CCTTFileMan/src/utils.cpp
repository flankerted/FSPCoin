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

QString AutoSizeString(qint64 size)
{
    QString sizedStr;
    if (size / MB_Bytes > 0)
        sizedStr = QString::number(size / MB_Bytes) + QString(" MB");
    else if(size / KB_Bytes > 0)
        sizedStr = QString::number(size / KB_Bytes) + QString(" KB");
    else
        sizedStr = QString::number(size) + QString(" B");
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

}
