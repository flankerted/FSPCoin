#include <iostream>
#include "fileMan.h"
#include "utils.h"

std::map<QString, std::pair<qint64, qint64> > FileManager::m_EmptySpaceInfos;

FileManager::FileManager()
{

}

FileManager::~FileManager()
{

}

bool FileManager::ParseObjList(QString& objListStr, QList<FileInfo>& sharings,
                               std::map<qint64, std::pair<qint64, QString> >& objInfos)
{
    const QString noObjId("object id not find");
    if (objListStr.compare(noObjId) == 0)
        return true;

    //objListStr = QString("{\"Sharing\":false,\"ObjId\":\"1\",\"Size\":\"256\"}/{\"Sharing\":false,\"ObjId\":\"2\",\"Size\":\"128\"}/{\"Sharing\":true,\"Owner\":\"0x5f663f10F12503Cb126Eb5789A9B5381f594A0eB\",\"ObjId\":\"3\",\"Offset\":\"64\",\"Length\":\"128\",\"StartTime\":\"2019-01-01 00:00:00\",\"StopTime\":\"2020-01-01 00:00:00\",\"FileName\":\"123.txt\",\"SharerCSNodeID\":\"...\"}");
    QStringList list = objListStr.split(CTTUi::spec);
    if (list.length() == 0)
        return false;

    for(int i = 0; i < list.length(); ++i)
    {
        ObjInfo info;
        info.DecodeQJson(list.at(i));

        if (!info.isSharing)
        {
            int objID = info.objectID.toInt();
            QString label = QString("");
            if (!objInfos[objID].second.isEmpty())
                label = objInfos[objID].second;
            objInfos[objID] =
                    std::pair<qint64, QString>(info.objectSize.toLongLong() * MB_Bytes, label);
        }
        else
        {
            // Get sharings
            FileInfo inf(GUIString::GetSingleton()->sysMsgs(GUIString::SharingSpaceLabel),
                    info.fileName, info.offset, info.length, true,
                    info.objectID, info.owner, info.sharerCSNodeID, info.key,
                    info.startTime, info.stopTime);
            sharings.append(inf);
        }
    }

    return true;
}

bool FileManager::ParseMetaData(const QString& metaData, qint64 objID, qint64& spaceTotal, qint64& spaceRemain,
                                std::map<qint64, std::pair<qint64, QString> >& objInfos,
                                std::vector<std::pair<QList<FileInfo>, qint64> >& vecFileList,
                                std::map<QString, std::pair<qint64, qint64> >& spaceInfos)
{
    QString label;
    if (!FileManager::CheckHead4K(metaData, label, spaceTotal, spaceRemain))
        return false;
    else
    {
        objInfos[objID].second = label;
        spaceInfos[label] = std::pair<qint64, qint64>(spaceRemain, spaceTotal);
    }
    QList<FileInfo> myFileList;
    FileManager::GetFileList(metaData, myFileList);
    vecFileList.push_back(std::pair<QList<FileInfo>, qint64>(myFileList, spaceRemain));

    return true;
}

bool FileManager::CheckHead4K(const QString& head4K)
{
    qint64 t;
    qint64 s;
    QString label;
    return CheckHead4K(head4K, label, t, s);
}

bool FileManager::CheckSpace(qint64 spare, qint64 fSize)
{
    return (spare > fSize && spare > 0 && fSize > 0);
}

int getSpareIndex(const QStringList& list)
{
    int spareIdx = list.length() - 1;
//    if (list.at(spareIdx).compare("sharing") == 0)
//        spareIdx = list.length() - 3;

    return spareIdx;
}

qint64 getAlign64(qint64 pos)
{
    qint64 m = pos % 64;
    if (m == 0) {
        return pos;
    }
    return pos + (64 - m);
}

bool FileManager::CheckHead4K(const QString& head4K, QString& label, qint64& total, qint64& spare)
{
    QStringList list = head4K.split(CTTUi::spec);
    if (list.length() < 3)
        return false;
    total = list.at(1).toLongLong();
    qint64 sizeUsed = 4096;
    bool ok = true;
    label = list.at(0);
    int spareIdx = getSpareIndex(list);
    for(int i = 2; i < spareIdx; i+=3)
    {
        sizeUsed += list.at(i + 2).toLongLong(&ok);
        if (!ok)
            return false;
        if (i + 3 < spareIdx)
        {
            qint64 posStart = list.at(i).toLongLong(&ok);
            if (!ok)
                return false;
            qint64 length = list.at(i + 2).toLongLong(&ok);
            if (!ok)
                return false;
            qint64 posStartNext = list.at(i + 3).toLongLong(&ok);
            if (!ok)
                return false;
            if (posStart + getAlign64(length) != posStartNext)
                return false;
        }
    }

    spare = list.at(spareIdx).toLongLong(&ok);
    if (!ok)
        return false;
    if (sizeUsed + spare != total)
        return false;

    return true;
}

void FileManager::GetFileList(const QString& head4K, QList<FileInfo>& myFiles)
{
    QStringList list = head4K.split(CTTUi::spec);
    myFiles.clear();
    int spareIdx = getSpareIndex(list);
    //for(int i = 3; i < list.length()-2; i+=3)
    for(int i = 3; i < spareIdx-1; i+=3)
    {
        FileInfo inf(list.at(0), list.at(i), list.at(i-1), list.at(i+1));
        myFiles.append(inf);
    }
}

QString FileManager::UpdateMetaData(const QString& headOrig, int headLength, QString fName, qint64 fSize, qint64& startPos)
{
    QString ret("");
    QStringList list = headOrig.split(CTTUi::spec);
    QList<FileInfo> fList;
    int spareIdx = getSpareIndex(list);
    if (spareIdx != list.length()-1)
    {
        list.pop_back();
        list.pop_back();
    }

    for(int i = 0; i < list.size()-1; i++)
    {
        if (fName.compare(list.at(i)) != 0)
            ret += list.at(i) + CTTUi::spec;
        else
            return GUIString::GetSingleton()->sysMsgs(GUIString::FNameduplicatedMsg);
    }
    if (list.size() == 3)
    {
        ret += QString::number(headLength) + CTTUi::spec + fName +
                CTTUi::spec + QString::number(fSize) + CTTUi::spec +
                QString::number(list.at(list.size()-1).toLongLong() - fSize);
        startPos = headLength;
        startPos = getAlign64(startPos);
    }
    else
    {
        startPos = list.at(list.size()-2).toLongLong() + list.at(list.size()-4).toLongLong();
        startPos = getAlign64(startPos);
        ret += QString::number(startPos) + CTTUi::spec + fName +
                CTTUi::spec + QString::number(fSize) + CTTUi::spec +
                QString::number(list.at(list.size()-1).toLongLong() - fSize);
    }

    return ret;
}

QSpaceListModel::QSpaceListModel(QObject* parent)
{
    Q_UNUSED(parent);
}

QSpaceListModel::~QSpaceListModel()
{

}

int QSpaceListModel::rowCount(const QModelIndex&) const
{
    return m_SpaceList.count();
}

QVariant QSpaceListModel::data(const QModelIndex& index, int role) const
{
    if (!index.isValid())
        return QVariant();

    if (index.row() >= m_SpaceList.size())
        return QVariant();

    if (role == SpaceLabelRole)
        return m_SpaceList.at(index.row());
    else
        return QVariant();
}

Qt::ItemFlags QSpaceListModel::flags(const QModelIndex& index) const
{
    if (!index.isValid())
        return Qt::ItemIsEnabled;

    return QAbstractItemModel::flags(index) | Qt::ItemIsEditable;
}

//bool QSpaceListModel::setData(const QModelIndex& index, const QVariant& value, int role)
//{
//    if (index.isValid() && role == SpaceLabelRole)
//    {
//        m_SpaceList.replace(index.row(), value.toString());
//        //emit dataChanged(index, index);
//        return true;
//    }
//    return false;
//}

QHash<int, QByteArray> QSpaceListModel::roleNames() const
{
    QHash<int, QByteArray> names;
    names[SpaceLabelRole] = "space";
    return names;
}

bool QSpaceListModel::insertRows(int position, int rows, const QModelIndex& parent)
{
    if (position < 0 || position > m_SpaceList.size() || rows <= 0)
        return false;

    beginInsertRows(parent, position, position+rows-1);
//    for (int row = 0; row < rows; ++row)
//    {
//        m_SpaceList.insert(position, "");
//    }
    endInsertRows();
    return true;
}

//bool QSpaceListModel::removeRows(int position, int rows, const QModelIndex& parent)
//{
//    if (!(position >= 0 && position <= m_SpaceList.size()-1) || rows <= 0)
//        return false;

//    beginRemoveRows(parent, position, position+rows-1);
//    for(int row = 0; row < rows; ++row)
//    {
//        m_SpaceList.removeAt(position);
//    }
//    endRemoveRows();
//    return true;
//}

bool QSpaceListModel::clearData(const QModelIndex& parent)
{
    beginRemoveRows(parent, 0, m_SpaceList.size()-1);
    m_SpaceList.clear();
    endRemoveRows();
    return true;
}

void QSpaceListModel::SetModel(const std::map<qint64, std::pair<qint64, QString> >& objInfos)
{
    clearData();
    int i = 0;
    for(std::map<qint64, std::pair<qint64, QString> >::const_iterator it = objInfos.begin(); it != objInfos.end(); it++)
    {
        m_SpaceList.append(it->second.second);
        insertRows(i, 1);
        //setData(createIndex(i, 0), it->second.second, SpaceLabelRole);
        i++;
    }

    //emit dataChanged(createIndex(0, 0), createIndex(m_SpaceList.count()-1, 0));
}

QString QSpaceListModel::GetSpaceLabel(int index)
{
    if (index == -1)
        return GUIString::GetSingleton()->sysMsgs(GUIString::SharingSpaceLabel);
    else if (index >= m_SpaceList.size() || index < 0)
        return "";

    return m_SpaceList.at(index);
}

int QSpaceListModel::GetSpaceIndex(QString spaceLabel)
{
    for(int i = 0; i < m_SpaceList.size(); i++)
    {
        if (m_SpaceList.at(i).compare(spaceLabel) == 0)
            return i;
    }

    return 0;
}

QFileListModel::QFileListModel(QObject* parent)
{
    Q_UNUSED(parent);
}

QFileListModel::~QFileListModel()
{

}

int QFileListModel::rowCount(const QModelIndex&) const
{
    return m_FileList.count();
}

QVariant QFileListModel::data(const QModelIndex& index, int role) const
{
    if (!index.isValid())
        return QVariant();

    qint64 uint = 0;
    if (index.row() >= m_FileList.size())
        return QVariant();

    if (role == FileNameRole)
        return m_FileList.at(index.row()).fileName;
    else if(role == TimeLimitRole)
    {
        if (!m_FileList.at(index.row()).isShared)
            return "-";
        else
            return m_FileList.at(index.row()).shareStart + "~" + m_FileList.at(index.row()).shareStop;
    }
    else if(role == FileSizeRole)
        return Utils::AutoSizeString(m_FileList.at(index.row()).fileSize.toLongLong(), uint);
    else
        return QVariant();
}

Qt::ItemFlags QFileListModel::flags(const QModelIndex& index) const
{
    if (!index.isValid())
        return Qt::ItemIsEnabled;

    return QAbstractItemModel::flags(index) | Qt::ItemIsEditable;
}

//bool QFileListModel::setData(const QModelIndex& index, const QVariant& value, int role)
//{
//    if (index.isValid() && role == FileNameRole)
//    {
//        m_FileList.at(index.row()).fileName = value.data()
//        return true;
//    }
//    else if(index.isValid() && role == TimeLimitRole)
//    {

//    }
//    else if(index.isValid() && role == FileSizeRole)
//    {

//    }

//    //emit dataChanged(index, index);
//    return false;
//}

QHash<int, QByteArray> QFileListModel::roleNames() const
{
    QHash<int, QByteArray> names;
    names[FileNameRole] = "fileName";
    names[TimeLimitRole] = "timeLimit";
    names[FileSizeRole] = "fileSize";
    return names;
}

bool QFileListModel::insertRows(int position, int rows, const QModelIndex& parent)
{
    if (position < 0 || position > m_FileList.size() || rows <= 0)
        return false;

    beginInsertRows(parent, position, position+rows-1);
//    for (int row = 0; row < rows; ++row)
//    {
//        m_FileList.insert(position, "");
//    }
    endInsertRows();
    return true;
}

//bool QFileListModel::removeRows(int position, int rows, const QModelIndex& parent)
//{
//    if (!(position >= 0 && position <= m_SpaceList.size()-1) || rows <= 0)
//        return false;

//    beginRemoveRows(parent, position, position+rows-1);
//    for(int row = 0; row < rows; ++row)
//    {
//        m_SpaceList.removeAt(position);
//    }
//    endRemoveRows();
//    return true;
//}

bool QFileListModel::ClearData(const QModelIndex& parent)
{
    beginRemoveRows(parent, 0, m_FileList.size()-1);
    m_FileList.clear();
    endRemoveRows();
    return true;
}

void QFileListModel::AddModel(const QList<FileInfo>& files)
{
    for (int i = 0; i < files.size(); i++)
        m_FileList.append(files.at(i));
    insertRows(0, files.size());
}

void QFileListModel::SetModel(const QList<FileInfo>& files)
{
    ClearData();
    m_FileList = files;
    insertRows(0, files.size());
//    for(int i = 0; i < files.size(); i++)
//        setData(createIndex(i, 0), it->second.second, SpaceLabelRole);

    //emit dataChanged(createIndex(0, 0), createIndex(m_SpaceList.count()-1, 0));
}

QString QFileListModel::GetFileName(int index)
{
    return m_FileList.at(index).fileName;
}
