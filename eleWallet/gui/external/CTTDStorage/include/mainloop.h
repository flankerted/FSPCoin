#ifndef MAINLOOP_H
#define MAINLOOP_H

#include <QTranslator>
#include <QStringListModel>
#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include "eleWalletBackRPC.h"
#include "agentDetails.h"
#include "fileMan.h"

// {"Title":"title_1","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"","Signature":"..."}/{"Title":"title_2","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"...","Signature":"..."}/{"Title":"title_3","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"...","Signature":"..."}
typedef struct _MailDetails
{
    QString mailTitle;
    QString mailSender;
    QString mailReceiver;
    QString mailContent;
    QString mailTimeStamp;
    QString mailFileSharingCode;

    QString mailSignature;

    void SetValues(QString title, QString sender, QString receiver, QString content, QString timeStamp,
                   QString fileSharingCode, QString sig="0")
    {
        mailTitle = title;
        mailSender= sender;
        mailReceiver = receiver;
        mailContent = content;
        mailTimeStamp = timeStamp;
        mailFileSharingCode = fileSharingCode;
        mailSignature = sig;
    }

    void SetSignature(QString sig)
    {
        mailSignature = sig;
    }

    void GetValues(QString& title, QString& sender, QString& receiver, QString& content, QString& timeStamp,
                   QString& fileSharingCode, QString& sig)
    {
        title = mailTitle;
        sender = mailSender;
        receiver = mailReceiver;
        content = mailContent;
        timeStamp = mailTimeStamp;
        fileSharingCode = mailFileSharingCode;

        sig = mailSignature;
    }

    QString EncodeQJson()
    {
        QJsonObject json;
        json.insert("Title", mailTitle);
        json.insert("Sender", mailSender);
        json.insert("Receiver", mailReceiver);
        json.insert("Content", mailContent);
        json.insert("TimeStamp", mailTimeStamp);
        json.insert("FileSharing", mailFileSharingCode);
        json.insert("Signature", mailSignature);

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
            mailTitle = json["Title"].toString();
            mailSender= json["Sender"].toString();
            mailReceiver = json["Receiver"].toString();
            mailContent = json["Content"].toString();
            mailTimeStamp = json["TimeStamp"].toString();
            mailFileSharingCode = json["FileSharing"].toString();
            mailSignature = json["Signature"].toString();
        }
        else
        {
            mailSignature = "0";
        }
    }

}MailDetails;


// For "mailList" in mainWindow.qml only
class QMailListModel : public QAbstractListModel
{
    Q_OBJECT

public:
    enum QMailListModelRole
    {
        SenderRole = Qt::UserRole + 30,
        TitleRole,
        TimeStampRole,
        AttachmentRole
    };

    QMailListModel(QObject* parent = nullptr);

    ~QMailListModel() override;

    int rowCount(const QModelIndex& parent = QModelIndex()) const override;

    QVariant data(const QModelIndex& index, int role) const override;

    virtual QHash<int, QByteArray> roleNames() const override;

    Qt::ItemFlags flags(const QModelIndex& index) const override;

    //bool setData(const QModelIndex& index, const QVariant& value, int role) override;

    bool insertRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    //bool removeRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

public:
    bool ClearData(const QModelIndex& parent = QModelIndex());

    void AddModel(const QList<MailDetails>& mails);

    void SetModel(const QList<MailDetails>& mails);

    QString GetMailAttachCode(int index) const;

    QString GetMailTitle(int index) const;

    QString GetMailSender(int index) const;

    QString GetMailTimeStamp(int index) const;

    QString GetMailContent(int index) const;

private:
    QList<MailDetails> m_MailList;

};


class MainLoop : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool isWin32 MEMBER m_Win32OperationSys)
    Q_PROPERTY(QString dataDir MEMBER m_DataDir WRITE setDataDir NOTIFY dataDirChanged)
    Q_PROPERTY(QStringList accounts MEMBER m_QAccounts NOTIFY accountsChanged)
    Q_PROPERTY(bool lockState MEMBER m_LockFlag WRITE setLockState NOTIFY lockChanged)
    Q_PROPERTY(bool connectedServ MEMBER m_HasConnFlag NOTIFY connectedServChanged)
    Q_PROPERTY(bool atService MEMBER m_AtServiceFlag NOTIFY serviceStateChanged)
    Q_PROPERTY(bool busying MEMBER m_Busying NOTIFY uiStatusChanged)
    Q_PROPERTY(QString currAgentAccount READ getCurrAgentAccount NOTIFY currAgentAccountChanged)
    Q_PROPERTY(QString currAgentPayment READ getCurrAgentPayment NOTIFY currAgentPaymentChanged)
    Q_PROPERTY(QString currAgentTStart READ getCurrAgentTStart NOTIFY currAgentTStartChanged)
    Q_PROPERTY(QString currAgentTStop READ getCurrAgentTStop NOTIFY currAgentTStopChanged)
    Q_PROPERTY(QString currAgentFlow READ getCurrAgentFlow NOTIFY currAgentFlowChanged)
    Q_PROPERTY(QString currAgentFlowRemain READ getCurrAgentFlowRemain NOTIFY currAgentFlowRemainChanged)
    Q_PROPERTY(QString account MEMBER m_AccountInUse NOTIFY accountChanged)
    Q_PROPERTY(QString balance MEMBER m_AccBalance NOTIFY balanceChanged)
    Q_PROPERTY(bool mainShowed MEMBER m_MainWindowShowedFlag NOTIFY mainWinStateChanged)
    Q_PROPERTY(bool isCreatingSpace MEMBER m_IsCreatingSpaceFlag WRITE setIsCreatingSpace NOTIFY isCreaetingSpaceChanged)
    Q_PROPERTY(bool isRefreshingFiles MEMBER m_IsRefreshingFilesFlag WRITE setIsRefreshingFiles NOTIFY isRefreshingFilesChanged)
    Q_PROPERTY(QString totalSpace MEMBER m_TotalSpace NOTIFY totalSpaceChanged)
    Q_PROPERTY(QString usedSpace MEMBER m_UsedSpace NOTIFY usedSpaceChanged)
    Q_PROPERTY(QString currentSpace MEMBER m_CurrentSpace NOTIFY currentSpaceChanged)
    Q_PROPERTY(int currentSpaceIdx READ getSpaceIdx WRITE setCurrentSpaceIdx)
    Q_PROPERTY(int currentFileIdx MEMBER m_CurrentFileIdx WRITE setCurrentFileIdx NOTIFY currentFileIdxChanged)
    Q_PROPERTY(bool transmitting MEMBER m_TransmittingFlag NOTIFY transmittingChanged)
    Q_PROPERTY(bool transmitRate READ getTransmitRate NOTIFY transmitRateChanged)
    Q_PROPERTY(QString sharingCode MEMBER m_SharingCode NOTIFY sharingCodeChanged)
    Q_PROPERTY(QString sharingInfoSharer MEMBER m_SharingInfoSharer NOTIFY sharingInfoSharerChanged)
    Q_PROPERTY(QString sharingInfoFileName MEMBER m_SharingInfoFileName NOTIFY sharingInfoFileNameChanged)
    Q_PROPERTY(QString sharingInfoStartTime MEMBER m_SharingInfoStartTime NOTIFY sharingInfoStartTimeChanged)
    Q_PROPERTY(QString sharingInfoStopTime MEMBER m_SharingInfoStopTime NOTIFY sharingInfoStopTimeChanged)
    Q_PROPERTY(QString sharingInfoPrice MEMBER m_SharingInfoPrice NOTIFY sharingInfoPriceChanged)
    Q_PROPERTY(int currentMailIdx MEMBER m_CurrentMailIdx WRITE setCurrentMailIdx NOTIFY currentMailIdxChanged)

signals:
    void dataDirChanged(QString dataDir);
    void accountsChanged(int count);
    void lockChanged(bool lock);
    void connectedServChanged();
    void serviceStateChanged();
    void uiStatusChanged(QString newStatus);
    void currAgentAccountChanged();
    void currAgentPaymentChanged();
    void currAgentTStartChanged();
    void currAgentTStopChanged();
    void currAgentFlowChanged();
    void currAgentFlowRemainChanged();
    void accountChanged(QString account);
    void balanceChanged(QString balance);
    void isCreaetingSpaceChanged();
    void isRefreshingFilesChanged();
    void mainWinStateChanged();
    void totalSpaceChanged(qint64 spaceTotal);
    void usedSpaceChanged(qint64 spaceUsed);
    void currentSpaceChanged(int currSpaceIndex);
    void currentFileIdxChanged();
    void transmittingChanged(int category); // 0:download, 1:upload, 2:completed
    void transmitRateChanged(int rate);
    void sharingCodeChanged();
    void sharingInfoSharerChanged();
    void sharingInfoFileNameChanged();
    void sharingInfoStartTimeChanged();
    void sharingInfoStopTimeChanged();
    void sharingInfoPriceChanged();
    void currentMailIdxChanged();

    void showMsgBoxSig(QString msg, int dlgType, bool copyID=false);
    void showYesNoBoxSig(QString msg);
    void closeMsgBoxSig();
    void refreshFilesSig(QString space="");
    void showTranmitRate(int rate);

public:
    Q_INVOKABLE void showLoginWindow();
    Q_INVOKABLE void setLanguage(QString language);
    Q_INVOKABLE QString getLanguage();
    Q_INVOKABLE void setDataDir(QString dir);
    Q_INVOKABLE void setLockState(bool state);
    Q_INVOKABLE void showMsgBox(QString msg, int dlgType, bool copyID=false);
    Q_INVOKABLE void showYesNoBox(QString msg);
    Q_INVOKABLE void closeMsgBox();
    Q_INVOKABLE void showAgentConnWindow();
    Q_INVOKABLE void autoConnectAgent(QString ip);
    Q_INVOKABLE void showMainWindow();
    Q_INVOKABLE QString initAccounts();
    Q_INVOKABLE QString newAccount(QString passwd);
    Q_INVOKABLE void setDefaultAccount(QString account);
    Q_INVOKABLE void copyToClipboard(QString text);
    Q_INVOKABLE void copyTxIDToClipboard(QString text);
    Q_INVOKABLE QString getAgentInfo();
    Q_INVOKABLE bool verifyPassword(QString account, QString password);
    Q_INVOKABLE QString getAutoConnectAgentServ();
    Q_INVOKABLE void setAutoConnectAgentServ(QString ip);
    Q_INVOKABLE void unsetAutoConnectAgentServ();
    Q_INVOKABLE QString connectAgentServ(QString agentIP);
    Q_INVOKABLE QString getAgentIP(int index);
    Q_INVOKABLE QString getAgentAccount(int index, bool shortMode=false);
    Q_INVOKABLE QString getAgentAvailablePayment(int index);
    Q_INVOKABLE QString getCurrAgentAccount();
    Q_INVOKABLE QString getSignedAgentDetail(QString agentConnectedAccount="");
    Q_INVOKABLE QString getSpaceLabelByIndex(int index);
    Q_INVOKABLE QString showFileList(QString spaceLabel="");
    Q_INVOKABLE QString updateFileList(QString spaceLabel="");
    Q_INVOKABLE QString reformatSpace();
    Q_INVOKABLE QString sendTransaction(QString receiver, QString amount, QString passwd);
    Q_INVOKABLE QString preApplyForAgentService(QString agentAccount, bool paymentTimeFlag,
                                                QString timePeriod, QString flowAmount, bool isNotCurrtAgent=false);
    Q_INVOKABLE QString applyForAgentService();
    Q_INVOKABLE QString cancelAgentService(QString agentAccount);
    Q_INVOKABLE QString createSpace(QString spaceLabel, QString spaceSize);
    Q_INVOKABLE void showDownloadDlg(bool isShare=false);
    Q_INVOKABLE int getSpaceIdx(QString spaceLabel="");
    Q_INVOKABLE QString getFileNameByIndex(int fileIndex);
    Q_INVOKABLE QString getFileSizeByIndex(int spaceIndex, int fileIndex);
    Q_INVOKABLE QString downloadFileByIndex(QString filePath, int spaceIndex, int fileIndex);
    Q_INVOKABLE QString getDirPathFromUrl(QString url);
    Q_INVOKABLE QString uploadFileByIndex(QString filePath, int spaceIndex);
    Q_INVOKABLE QString shareFileToAccount(int spaceIndex, int fileIndex, QString receiver,
                                           QString startTime, QString stopTime, QString price);
    Q_INVOKABLE QString getSharingInfo(QString sharingCode);
    Q_INVOKABLE QString payforSharing(QString sharingCode);
    Q_INVOKABLE QString refreshMails();
    Q_INVOKABLE QString getMailTitle(int index);
    Q_INVOKABLE QString getMailSender(int index);
    Q_INVOKABLE QString getMailTimeStamp(int index);
    Q_INVOKABLE QString getMailContent(int index);
    Q_INVOKABLE QString sendNewMail(QString title, QString receiver, QString content, QString fileSharingCode);
    Q_INVOKABLE void preExit();

public:
    explicit MainLoop(EleWalletBackRPCCli* rpc, QGuiApplication* app, QQmlApplicationEngine* engine, QTranslator* translator);

    ~MainLoop(void);

    QString InitAccounts();

    void InitAgentsInfo();

    void SetLoginWindow(QObject* qmlObj);

    void HideLoginWindow();

    void SetAgentWindow(QObject* qmlObj);

    void SetMainWindow(QObject* qmlObj);

    void SetDownloadDlg(QObject* qmlObj);

    void SetMsgDialog(QObject* qmlObj);

    QAgentModel* GetAgentsModel() const;

    QSpaceListModel* GetSpaceListModel() const;

    QFileListModel* GetFileListModel() const;

    QMailListModel* GetMailListModel() const;

private:
    QGuiApplication* m_App;
    QQmlApplicationEngine* m_Engine;
    QTranslator* m_Translator;

    bool m_Win32OperationSys;

    QObject* m_LoginWin;
    QObject* m_AgentWin;
    QObject* m_MainWin;
    QObject* m_DownloadDlg;
    QObject* m_MsgDialog;

    GUIString* m_GuiString;
    EleWalletBackRPCCli* m_BackRPC;

    QString m_DataDir;
    QStringList m_QAccounts;
    QString m_AccountInUse;
    QString m_AccountPwd;

    std::map<QString, AgentInfo> m_AgentInfo; // [Agent IP address]:[Agent information]
    std::map<QString, AgentInfo> m_AgentTestInfo; // [Agent IP address]:[Agent information]
    std::map<QString, AgentDetails> m_AgentDetails; // [Agent account]:[Agent details]
    QAgentModel* m_QAgentsModel;
    QString m_AgentConnectedAccount;
    QString m_AgentConnectedIP;
    QString m_AgentDetailsRawData;

    QString m_PreServApplyAccount;
    QString m_PreServApplyTimePeriod;
    QString m_PreServApplyFlowAmount;

    QString m_AccBalance;

    std::map<qint64, std::pair<qint64, QString> > m_ObjInfos; // [ObjID]:[Total space(Bytes), Space label]
    std::vector<std::pair<QList<FileInfo>, qint64> > m_VecFileList; // [File info list]:[Remining space in this obj]
    std::map<QString, std::pair<qint64, qint64> > m_SpaceInfos; // [Space label]:[Remining space in this obj, Total space]
    std::map<QString, QString > m_SpaceReformatInfos; // [Space label]:[Meta data json of the space need to be reformatted]
    QSpaceListModel* m_QSapceListModel;
    QFileListModel* m_QFileListModel;
    QString m_TotalSpace, m_UsedSpace;
    QString m_CurrentSpace;
    QString m_ReformatMetaJson;
    int m_CurrentSpaceIdx;
    int m_CurrentFileIdx;

    QString m_CreateSpaceTX;
    QString m_NewSpaceLabel;
    qint64 m_NewSpaceSize;
    int m_NewObjID;

    int* m_TranmitRate;

    QString m_SharingCode;
    QString m_SharingInfoSharer;
    QString m_SharingInfoFileName;
    QString m_SharingInfoStartTime;
    QString m_SharingInfoStopTime;
    QString m_SharingInfoPrice;
    QString m_SharingCodePay;

    QList<MailDetails> m_MailList;
    QMailListModel* m_QMailListModel;
    int m_CurrentMailIdx;

    bool m_HasConnFlag;
    bool m_AtServiceFlag;
    bool m_Busying;
    bool m_MainWindowShowedFlag;
    bool m_LockFlag;
    bool m_WaitingSpaceFlag;
    bool m_TransmittingFlag;
    bool m_IsCreatingSpaceFlag;
    bool m_IsRefreshingFilesFlag;
    bool m_PreExitFlag;
    bool m_ExitFlag;

    bool m_AgentTestInitedFlag;

protected:
    int m_nTimerId;
    int m_RefreshTimerId;
    void timerEvent(QTimerEvent* event);

private slots:
    void refreshFilesSigConn(QString space);
    void updateTranmitRate(int rate);

private:
    void closeMshBoxAndWaitForShowMsgBox();

    void setAccountList(QString accountsStr="", QString defAccount="");

    QString getDefaultAccount();

    void setAccountInUse(QString account);

    void addAccount(QString account);

    void initAgentTestInfoModel();

    void setAgentInfoModel(const std::map<QString, AgentInfo>& agentInfo);

    QString doGetSignedAgentDetailOp();

    QString getAgentIPByAcc(const QString& account);

    void setAgentIPAddress(std::map<QString, AgentDetails>& agentDetails);

    void setAgentIPAddress(std::map<QString, AgentDetails>& agentDetails, QString ip, QString account);

    void updateAgentInfoByIP(const std::map<QString, AgentDetails>& agentDetails, QString ipAddress);

    AgentDetails getAgentDetailsByIP(QString IP);

    QString getMyAgentNoneID();

    QString doGetAgentAccByIPOp(QString IPAddress);

    AgentDetails getAgentDetailsByAcc(const QString& account);

    QString getCurrAgentPayment();

    QString getCurrAgentTStart();

    QString getCurrAgentTStop();

    QString getCurrAgentFlow();

    QString getCurrAgentFlowRemain();

    void setServiceState();

    void setIsCreatingSpace(bool flag);

    void setIsRefreshingFiles(bool flag);

    void setBusyingFlag(bool state, QString status);

    bool doSignAgentOp(QString csAddr, QString expireTime, QString flowAmount, QString password, QString& result);

    bool doCancelAgentOp(QString agentAcc, QString password, QString& result);

    void updateAgentDetails(QString agentAccount, QString TXtoMonitor, GUIString::RpcCmds cmd, bool newState);

    bool doWaitTXCompleteOp(QString TXtoMonitor, GUIString::RpcCmds cmdType);

    QString doGetAccBalanceOp(QString account);

    void setAccountBalance(QString bal);

    void setCurrentFileIdx(int index);

    void setCurrentSpaceIdx(int index);

    qint64 getRemainingSpaceSizeByIndex(int spaceIndex);

    bool getCorruptedSpaceInfo(QString account, qint64 objId, std::map<qint64, std::pair<qint64, QString> >& objInfos,
                               std::map<QString, std::pair<qint64, qint64> >& spaceInfos,
                               std::map<QString, QString> &spaceReformatInfos);

    QString refreshFiles(QString spaceLabel="");

    QString getFileList(QString spaceLabel);

    void setSpaceListModel(const std::map<qint64, std::pair<qint64, QString> >& objInfos);

    void clearFileListModel();

    void addFileListModel(const QList<FileInfo>& files);

    void setFileListModel(const QList<FileInfo>& files);

    bool doReadMetaDataOp(qint64 objID, QString& metaData);

    void showFileTableAndInfo(QString spaceLabel, qint64 totalSpace);

    void setSpaceInfo(qint64 totalSpace, qint64 remainSpace);

    void waitingForNewSpaceCreating(QString TXtoMonitor);

    bool readMetaDataByObjID(qint64 objID, qint64& spaceTotal, qint64& spaceRemain, QString& result);

    bool writeMetaDataOp(qint64 id, QString& metaData);

    QString getNewMetaData(QString label, qint64 size);

    bool newMetaDataOp(qint64 id, QString label, qint64 size);

    void tryNewMetaData(qint64 totalSize);

    bool recoverNewSpaceMetaData(qint64 newSpaceObjID, QString &lastMetaData);

    QString doDownloadFileOp(QString filePath, QString fName, qint64 objID, qint64 offset, qint64 length,
                             QString sharerAddr, QString sharerAgentNodeID, QString key);

    QString getDownFileOpRet(QString qRet);

    bool updateMetaDataOp(qint64 id, QString fName, qint64 fSize, qint64& posStart, QString& updateMetaData);

    bool checkFileName(int spaceIndex, QString fileName);

    void doUploadFileOp(qint64 objID, qint64 offset, QString fileName, qint64 fileSize, QString filePath);

    bool restoreLastMetaData(qint64 targetObjID, QString &lastMetaData);

    void setTransmittingFlag(bool state, int category);

    int getTransmitRate() const;

    void progressUpdatingLoop();

    QString generateSharingCode(qint64 objID, QString fileName, qint64 offset, qint64 size, QString receiver,
                                QString startTime, QString stopTime, QString price, QString sharerAgentNodeID);

    void setSharingCode(QString code);

    void setSharingFileInfo(QString sharer, QString fName, QString startTime, QString stopTime, QString price);

    bool parseMailList(QString& mailListStr);

    void setMailListModel(const QList<MailDetails>& mails);

    void setCurrentMailIdx(int index);

private:
    const int timeRefreshInterv = 5000;
    const int fHeadLength = 4096;
    const std::string fHeadLengthStr = std::to_string(fHeadLength);
    const int delayTimeInterv = 100;
#ifdef WIN32
    const bool isWin32 = true;
#else
    const bool isWin32 = false;
#endif
};

#endif // MAINLOOP_H
