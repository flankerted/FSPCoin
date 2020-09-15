#ifndef MAINWINDOW_H
#define MAINWINDOW_H

#include <QMainWindow>
#include <QDesktopWidget>
#include <QComboBox>
#include <QPushButton>
#include <QTextBrowser>
#include <QBoxLayout>
#include <QStringListModel>
#include <QMessageBox>
#include <QFileInfo>
#include <QStandardItemModel>
#include "eleWalletBackRPC.h"
#include "fileMan.h"
#include "agentDetails.h"
#include "mailsTab.h"
#include "strings.h"

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

namespace Ui {

class MainWindow;

}

class MainWindow : public QMainWindow
{
    Q_OBJECT
public:
    bool HasConnection;

public:
    explicit MainWindow(QWidget *parent, quint16 port);
    ~MainWindow();

    int MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
                       QMessageBox::StandardButton btn, bool modal=true, bool exec=false);

signals:
    void ShowTransferRate(int rate);

    void ShowDownFileOpRetSign(QString qRet);

    void ShowUpFileOpRetSign(QString qRet);

    void ShowOpErrorMsgBoxSign(QString text, bool modal=true, bool exec=false);

    void TXCompleteSign(QString cmdType);

private:
    const int labelWidth = 200;
    const int editorWidth = 480;
    const int minimumWinW = 960;
    const int minimumWinH = 640;
    const int fileTableInitW0 = 440;
    const int fileTableInitW1 = 350;
    const int fileTableInitW2 = 70;
    const QSize dataDirDlgSize = QSize(480, 150);
    const QSize upFileSelectDlgSize = QSize(480, 110);
    const QSize pwdEnterDlgSize = QSize(360, 120);
    const QSize createSpaceDlgSize = QSize(240, 180);
    const QSize expandSpaceDlgSize = QSize(360, 120);
    const QSize btnSize = QSize(100, 25);
    const int fHeadLength = 4096;
    const std::string fHeadLengthStr = std::to_string(fHeadLength);
    const std::string fileTableLabels = "file name" + Ui::spec.toStdString() + "file time" + Ui::spec.toStdString() + "file size";
    const int timeInterval = 1000;
    const int timeRefreshInterv = 5000;

    const int mailTableInitW0 = 320;
    const int mailTableInitW1 = 320;
    const int mailTableInitW2 = 80;
    const int mailTableInitW3 = 140;
    const std::string mailTableLabels = "sender" + Ui::spec.toStdString() + "title" + Ui::spec.toStdString() +
            "attatchment" + Ui::spec.toStdString() + "time stamp";

    const QString sharingLabel = QString("Sharings");

private:
    Ui::MainWindow *ui;
    QLineEdit* pwdEdit;
    QLineEdit* newSpaceLabelEdit;
    QLineEdit* newSpaceSizeEdit;
    QMessageBox* m_MsgBox;

    int* m_TranferRate;
    EleWalletBackRPCCli m_BackRPC;
    std::string m_Result;
    QString m_AccountInUse;
    QString m_AccountPwd;
    QString m_AgentIPInUse;
    QString m_AgentConnectedAccount;
    QString m_AgentDetailsRawData;
    std::map<QString, AgentDetails> m_AgentDetails; // [Agent account]:[Agent details]
    std::map<QString, QString> m_AgentInfo; // [Agent IP address]:[Agent account]
    std::map<Ui::GUIStrings, std::string> m_StringsTable;
    std::map<Ui::GUIStrings, std::string> m_Cmds;
    std::map<Ui::GUIStrings, std::string> m_SysMsgs;
    std::map<Ui::GUIStrings, std::string> m_UIHints;
    std::string m_DataDir;
    std::string m_CreateSpaceTX;

    QStringList m_QAgents;
    QStringListModel* m_QAgentsModel;
    QStringList m_QAccounts;
    QStringListModel* m_QAccountsModel;
    QStandardItemModel* m_FileTableModel;

    std::map<qint64, std::pair<qint64, QString> > m_ObjInfos; // [ObjID]:[Total space, Space label]
    std::vector<std::pair<QList<FileInfo>, qint64> > m_VecFileList; // [File info list]:[Remining space in this obj]
    qint64 m_NewObjID;
    QString m_NewSpaceLabel;

    QString m_TXHash;

    std::vector<MailDetails> m_MailList;
    MailDetails m_MailCurrentSelected;

    bool m_BusyFlag;
    bool m_LangENFlag;
    bool m_IsLoginFlag;
    bool m_HasAgentFlag;
    bool m_HasConnFlag;
    bool m_AtServiceFlag;
    bool m_GetListFlag;
    bool m_WaitingSpaceFlag;
    bool m_CloseFlag;

protected:
    int m_nTimerId;
    int m_RefreshTimerId;
    void timerEvent(QTimerEvent* event);

public slots:
    void UpdateTransferRate(int rate);

private slots:
    void ShowDownFileOpRet(QString qRet);

    void ShowUpFileOpRet(QString qRet);

    void ShowOpErrorMsgBox(QString text = "", bool modal=true, bool exec=false);

    void HandleTXComplete(QString cmdType);

    void on_actionEnglish_triggered();

    void on_actionChinese_triggered();

    void on_fastConnBtn_clicked();

    void on_agentSettingBtn_clicked();

    void on_newAccountBtn_clicked();

    void on_accSettingBtn_clicked();

    void on_loginBtn_clicked();

    void on_logoutBtn_clicked();

    void on_agSetDefBtn_clicked();

    void on_deleteAgBtn_clicked();

    void on_addAgentBtn_clicked();

    void on_getAgDetailsBtn_clicked();

    void on_accSetDefBtn_clicked();

    void on_copyAccBtn_clicked();

    void on_refreshAccBtn_clicked();

    void on_downloadBtn_clicked();

    void on_uploadBtn_clicked();

    void on_createSpaceBtn_clicked();

    void on_expSpaceBtn_clicked();

    void on_updateFlistBtn_clicked();

    void on_curSpaceComboBox_currentTextChanged(const QString &arg1);

    void on_copyAgentBtn_clicked();

    void on_agentsComboBox_currentTextChanged(const QString &arg1);

    void on_accountList_clicked(const QModelIndex &index);

    void on_applyForServBtn_clicked();

    void on_cancelServBtn_clicked();

    void on_sharingBtn_clicked();

    void on_sendTxBtn_clicked();

    void on_tabWidget_currentChanged(int index);

    void on_newMailBtn_clicked();

    void on_refreshMailBtn_clicked();

    void on_getMailAttachBtn_clicked();

    void on_mailTable_clicked(const QModelIndex &index);

private:
    void closeEvent(QCloseEvent*e);

    void ShowOpSuccessMsgBox(bool modal=true, bool exec=false);

    int ShowPasswdEnterDlg();

    int ShowCreateSpaceDlg();

    int ShowExpandSpaceDlg();

    void SetLang();

    void TryNewMetaData(qint64 spareSize);

    bool SetDataDir();

    void InitAgents();

    int GetAgentList();

    void GetDefaultAgent();

    void AddAgent(QString agent);

    void SetDefaultAgent(QString agent);

    void DeleteAgent(QString agent);

    void DoGetAgentInfoOp(bool showMsg);

    void DoGetSignedAgentsOp(bool showMsg=true);

    void GetSignedAgents();

    QString GetAgentIPByAcc(const QString& account);

    void GetAgentIPAddresses(std::map<QString, AgentDetails>& agentDetails);

    QString DoGetAgentAccByIPOp(QString IPAddress);

    AgentDetails GetAgentDetailsByAcc(const QString& account);

    AgentDetails GetAgentDetailsByIP(QString IP);

    QString GetMyAgentNoneID();

    void ShowAgentDetails();

    void UpdateAgentDetails(QString TXtoMonitor, std::string cmd);

    void ClearAgentDetails();

    void RefreshAgentDetails();

    void DoWaitTXCompleteOp(QString TXtoMonitor, std::string cmdType);

    std::string DoGetAccBalanceOp(QString account=QString(""));

    void ShowAccBalance(QString val, QString pend, bool isTotal=true);

    void ProgressUpdatingLoop();

    void InitUIElements();

    void InitFileTable();

    void InitMailTable();

    bool CheckDataDir();

    bool InitAccounts(std::string& err);

    void SetAccountList(QString accountsStr);

    void GetDefaultAccount();

    void AddAccount(QString account);

    void SetDefaultAccount(QString account);

    bool GetFileList();

    void RefreshFiles();

    void RefreshMails();

    bool ReadMetaData(qint64 objID, qint64& spaceTotal, qint64& spaceRemain, bool showNoMsg=false);

    bool ParseObjList(QString &objListStr, QList<FileInfo>& sharings);

    void ShowFileTable(qint64 totalSpace, QString curLabel=QString(""));

    void ShowMailTable();

    void ShowSpaceAmount(qint64 totalSpace, qint64 spareBytes);

    bool WriteMetaDataOp(qint64 id, QString& metaData);

    bool NewMetaDataOp(qint64 id, QString label, qint64 size);

    bool UpdateMetaDataOp(qint64 id, QString fName, qint64 fSize, qint64& posStart, bool showNoMsg=false);

    void DoDownloadFileOp(std::string filePath, QString fName);

    void DoUploadFileOp(std::string filePath);

    bool DoSignAgentOp(QString csAddr, QString expireTime, QString flowAmount, QString password, QString& result);

    bool DoCancelAgentOp(QString csAddr, QString password, QString& result);

    void EnableElements();

    void WaitingForNewSpaceCreating();

    bool ParseMailList(QString& mailListStr);
};

#endif // MAINWINDOW_H
