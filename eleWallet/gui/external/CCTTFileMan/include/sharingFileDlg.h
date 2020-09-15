#ifndef SHARINGFILEDLG_H
#define SHARINGFILEDLG_H

#include <QDialog>
#include <QButtonGroup>
#include <QMessageBox>
#include "eleWalletBackRPC.h"
#include "strings.h"

typedef struct _FileSharingInfo
{
    qint64 objID;
    QString fileName;
    QString fileOffset;
    QString fileSize;
    QString receiver;
    QString timeStart;
    QString timeStop;
    QString price;
    QString sharerAgentNodeID;

    void SetValues(qint64 objId, QString name, qint64 offset, qint64 size, QString rcver,
                   QString tStart, QString tStop, qint64 p, QString agent)
    {
        objID = objId;
        fileName= name;
        fileOffset = QObject::tr("%1").arg(offset);
        fileSize = QObject::tr("%1").arg(size);
        receiver = rcver;
        timeStart = tStart;
        timeStop = tStop;
        price = QObject::tr("%1").arg(p);
        sharerAgentNodeID = agent;
    }

    void GetValues(qint64& objId, QString& name, qint64& offset, qint64& size, QString& rcver,
                   QString& tStart, QString& tStop, qint64& p, QString& agent)
    {
        objId = objID;
        name = fileName;
        offset = fileOffset.toLongLong();
        size = fileSize.toLongLong();
        rcver = receiver;
        tStart = timeStart;
        tStop = timeStop;
        p = price.toLongLong();
        agent = sharerAgentNodeID;
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
            fileOffset = json["FileOffset"].toString();
            fileSize = json["FileSize"].toString();
            receiver = json["Receiver"].toString();
            timeStart = json["TimeStart"].toString();
            timeStop = json["TimeStop"].toString();
            price = json["Price"].toString();
            sharerAgentNodeID = json["SharerCSNodeID"].toString();
        }
    }

}FileSharingInfo;

namespace Ui {
class SharingFileDialog;
}

class SharingFileDlg : public QDialog
{
    Q_OBJECT

public:
    explicit SharingFileDlg(bool noFileSelected, QString passwd, QString agent, const std::map<Ui::GUIStrings, std::string>& uiHints,
                            EleWalletBackRPCCli* backRPC, bool isAttachment=false, QString* mailToOrCode=nullptr, QWidget* parent = nullptr);

    ~SharingFileDlg();

    void SetFileInfo(QString fileName, qint64 fileObjID, qint64 fileOffset, qint64 fileSize);

    void SetSharingCode(const QString& code);

private slots:

    void on_cancelBtn_clicked();

    void on_doneBtn_clicked();

    void on_shareBtn_clicked();

    void on_copyBtn_clicked();

    void on_getSharingInfoBtn_clicked();

    void on_paySharingBtn_clicked();

private:
    Ui::SharingFileDialog *ui;

    std::map<Ui::GUIStrings, std::string> m_UiHints;

    QMessageBox* m_MsgBox;

    QString m_StartTime;
    QString m_StopTime;
    qint64 m_Price;
    QString m_Receiver;

    QString m_FileName;
    qint64 m_FileObjID;
    qint64 m_FileOffset;
    qint64 m_FileSize;
    QString m_SharerAgentNodeID;

    QString m_Signature;
    QString m_SharingCode;

    QString m_AccountPwd;
    EleWalletBackRPCCli* m_BackRPC;
    std::map<Ui::GUIStrings, std::string> m_SysMsgs;
    std::map<Ui::GUIStrings, std::string> m_Cmds;

    bool m_IsAttached;
    QString* m_MailSharingCode;

private:  
    QString GenerateSharingCode();

    int MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon=QMessageBox::Warning,
                       QMessageBox::StandardButton btn=QMessageBox::Ok, bool modal=true, bool exec=true);
};

#endif // SHARINGFILEDLG_H
