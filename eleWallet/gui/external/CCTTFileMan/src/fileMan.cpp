#include <iostream>
#include "fileMan.h"
#include "utils.h"

FileManager::FileManager()
{

}

FileManager::~FileManager()
{

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
    QStringList list = head4K.split(Ui::spec);
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
            {
                std::cout<<"startpos: "<< posStart << ", length: " << length << ", nextpos: " << posStartNext << std::endl;
                return false;
            }
        }
    }

    spare = list.at(spareIdx).toLongLong(&ok);
    if (!ok)
        return false;
    if (sizeUsed + spare != total)
        return false;

    return true;
}

void FileManager::GetFileList(QString& head4K, QList<FileInfo>& myFiles)
{
    QStringList list = head4K.split(Ui::spec);
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
    QStringList list = headOrig.split(Ui::spec);
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
            ret += list.at(i) + Ui::spec;
        else
        {
            std::map<Ui::GUIStrings, std::string> msg = Ui::SysMessages;
            return QString::fromStdString(msg[Ui::fNameduplicatedMsg]);
        }
    }
    if (list.size() == 3)
    {
        ret += QString::number(headLength) + Ui::spec + fName +
                Ui::spec + QString::number(fSize) + Ui::spec +
                QString::number(list.at(list.size()-1).toLongLong() - fSize);
        startPos = headLength;
        startPos = getAlign64(startPos);
    }
    else
    {
        startPos = list.at(list.size()-2).toLongLong() + list.at(list.size()-4).toLongLong();
        startPos = getAlign64(startPos);
        ret += QString::number(startPos) + Ui::spec + fName +
                Ui::spec + QString::number(fSize) + Ui::spec +
                QString::number(list.at(list.size()-1).toLongLong() - fSize);
    }

    return ret;
}
