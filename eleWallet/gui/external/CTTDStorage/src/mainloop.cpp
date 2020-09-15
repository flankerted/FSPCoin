#include <iostream>
#include <QThread>
#include <QApplication>
#include <QFileInfo>
#include <QClipboard>
#include <QDateTime>
#include "mainLoop.h"
#include "uiConfig.h"

using namespace std;

void Delay(int time)
{
    clock_t now = clock();
    while(clock() - now < time);
}

void DelayMS(int ms)
{
#ifdef WIN32
    Delay(ms);
#else
    Delay(ms * 1000);
#endif
}

MainLoop::MainLoop(EleWalletBackRPCCli* rpc, QGuiApplication* app, QQmlApplicationEngine* engine, QTranslator* translator) :
    m_App(app),
    m_Engine(engine),
    m_Translator(translator),
    m_Win32OperationSys(true),
    m_GuiString(GUIString::GetSingleton()),
    m_BackRPC(rpc),
    m_DataDir(""),
    m_QAgentsModel(new QAgentModel),
    m_AccBalance("0"),
    m_QSapceListModel(new QSpaceListModel),
    m_QFileListModel(new QFileListModel),
    m_CurrentSpace(""),
    m_ReformatMetaJson(""),
    m_CurrentFileIdx(0),
    m_QMailListModel(new QMailListModel),
    m_CurrentMailIdx(0),
    m_HasConnFlag(false),
    m_AtServiceFlag(false),
    m_Busying(false),
    m_MainWindowShowedFlag(false),
    m_LockFlag(true),
    m_WaitingSpaceFlag(false),
    m_TransmittingFlag(false),
    m_IsCreatingSpaceFlag(false),
    m_IsRefreshingFilesFlag(false),
    m_PreExitFlag(false),
    m_ExitFlag(false),
    m_AgentTestInitedFlag(false),
    m_nTimerId(0),
    m_RefreshTimerId(0)
{
    m_Win32OperationSys = isWin32;
    m_TranmitRate = new int;
    *m_TranmitRate = 0;
    m_BackRPC->SetRate(m_TranmitRate);

    connect(this, SIGNAL(refreshFilesSig(QString)), this, SLOT(refreshFilesSigConn(QString)), Qt::QueuedConnection);
    connect(this, SIGNAL(showTranmitRate(int)), SLOT(updateTranmitRate(int)), Qt::QueuedConnection);
    connect(this, SIGNAL(showMsgBoxSig(QString, int, bool)), this, SLOT(showMsgBox(QString, int, bool)), Qt::QueuedConnection);
    connect(this, SIGNAL(showYesNoBoxSig(QString)), this, SLOT(showYesNoBox(QString)), Qt::QueuedConnection);
    connect(this, SIGNAL(closeMsgBoxSig()), SLOT(closeMsgBox()), Qt::QueuedConnection);
}

MainLoop::~MainLoop()
{
    m_ExitFlag = true;
    if (m_nTimerId != 0)
        killTimer(m_nTimerId);
    if (m_RefreshTimerId != 0)
        killTimer(m_RefreshTimerId);

    delete m_QAgentsModel;
    m_QAgentsModel = nullptr;

    delete m_QSapceListModel;
    m_QSapceListModel = nullptr;

    delete m_QFileListModel;
    m_QFileListModel = nullptr;

    delete m_QMailListModel;
    m_QMailListModel = nullptr;

    delete m_TranmitRate;
}

void MainLoop::refreshFilesSigConn(QString space)
{
    refreshFiles(space);
}

void MainLoop::updateTranmitRate(int rate)
{
    emit transmitRateChanged(rate);
    if (rate == 100)
        *m_TranmitRate = 0;
}

void MainLoop::showLoginWindow()
{
    if (m_LoginWin == nullptr)
        return;
    m_LoginWin->setProperty("visible", true);
}

void MainLoop::setLanguage(QString language)
{
    QString lang = "";
    if (language.isEmpty())
    {
        lang = UiConfig().Get(CTTUi::WalletSettings, CTTUi::Lang).toString();
        if (lang.isEmpty())
            lang = "CN";
    }
    else
        lang = language;

    QVariant value(lang);
    UiConfig().Set(CTTUi::WalletSettings, CTTUi::Lang, value);
    if (language.isEmpty() && lang.compare("CN") == 0)
        return;

    // translate
    if (lang.compare("EN") == 0)
        m_Translator->load(":/lang/en_US.qm");
    else if(lang.compare("CN") == 0)
        m_Translator->load("");
    //translator->load(":/zh_CN.qm");

    m_App->installTranslator(m_Translator);
    m_Engine->retranslate();
    m_GuiString->SetLanguage(lang);
}

QString MainLoop::getLanguage()
{
    QString lang = UiConfig().Get(CTTUi::WalletSettings, CTTUi::Lang).toString();
    if (lang.isEmpty())
        lang = "CN";
    return lang;
}

void MainLoop::setDataDir(QString dir)
{
    m_DataDir = dir;
    if (m_DataDir.isEmpty())
        m_DataDir = QCoreApplication::applicationDirPath() + "/data";

    QVariant value(m_DataDir);
    UiConfig().Set(CTTUi::WalletSettings, CTTUi::DataDir, value);
    m_BackRPC->SendOperationCmd(GUIString::SetDataDirCmd, std::vector<QString>{m_DataDir});

    emit dataDirChanged(dir);
}

void MainLoop::setLockState(bool state)
{
    m_LockFlag = state;
    emit lockChanged(state);
}

void MainLoop::showMsgBox(QString msg, int dlgType, bool copyID)
{
    if (m_MsgDialog == nullptr)
        return;
    //QMetaObject::invokeMethod(msgDialog, "openMsg", Q_ARG(QString, "world hello"));
    m_MsgDialog->setProperty("visible", true);
    QObject *textObject = m_MsgDialog->findChild<QObject*>("content");
    textObject->setProperty("text", msg);
    QObject *btnObject = m_MsgDialog->findChild<QObject*>("copyBtn");
    btnObject->setProperty("visible", copyID);

    if (dlgType == GUIString::HintDlg)
        QMetaObject::invokeMethod(m_MsgDialog, "openHintBox");
    else if(dlgType == GUIString::SelectionDlg)
        QMetaObject::invokeMethod(m_MsgDialog, "openSelectionBox");
    else if(dlgType == GUIString::WaitingDlg)
        QMetaObject::invokeMethod(m_MsgDialog, "openWaitingBox");
}

void MainLoop::showYesNoBox(QString msg)
{
    if (m_MsgDialog == nullptr)
        return;

    m_MsgDialog->setProperty("visible", true);
    QObject *textObject = m_MsgDialog->findChild<QObject*>("contentYesNo");
    textObject->setProperty("text", msg);
    QMetaObject::invokeMethod(m_MsgDialog, "openYesNoBox");
}

void MainLoop::closeMsgBox()
{
    if (m_MsgDialog == nullptr)
        return;
    //QMetaObject::invokeMethod(msgDialog, "openMsg", Q_ARG(QString, "world hello"));
    m_MsgDialog->setProperty("visible", false);
    QMetaObject::invokeMethod(m_MsgDialog, "closeWaitingBox");
}

void MainLoop::showAgentConnWindow()
{
    if (m_AgentWin == nullptr)
        return;
    //QMetaObject::invokeMethod(msgDialog, "openMsg", Q_ARG(QString, "world hello"));
    m_AgentWin->setProperty("visible", true);
}

void MainLoop::autoConnectAgent(QString ip)
{
    if (m_AgentWin == nullptr)
        return;
    QMetaObject::invokeMethod(m_AgentWin, "connectAgentServ", Q_ARG(QVariant, ip));
}

void MainLoop::showMainWindow()
{
    if (m_MainWin == nullptr)
        return;

    m_MainWin->setProperty("visible", true);

    m_MainWindowShowedFlag = true;
}

QString MainLoop::initAccounts()
{
    return InitAccounts();
}

QString MainLoop::newAccount(QString passwd)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::NewAccountCmd, vector<QString>{passwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) != 0 &&
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) != 0)
    {
        addAccount(QString::fromStdString(ret));
        return QString::fromStdString(ret);
    }

    return "";
}

QString MainLoop::InitAccounts()
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::ShowAccountsCmd, vector<QString>());
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) != 0  &&
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) != 0 &&
            ret.compare(m_GuiString->sysMsgs(GUIString::NoAnyAccountMsg).toStdString()) != 0)
    {
        // Get default
        QString defAccount = getDefaultAccount();

        // Set account list
        setAccountList(QString::fromStdString(ret), defAccount);
    }
    else
        UiConfig().Set(CTTUi::AccountSettings, CTTUi::DefaultAccount, QString(""));

    return QString::fromStdString(ret);
}

void MainLoop::timerEvent(QTimerEvent*)
{
    if (!m_PreExitFlag)
    {
        bool atService = m_AtServiceFlag;
        doGetSignedAgentDetailOp();
        setAgentIPAddress(m_AgentDetails);
        if (atService != (m_AgentDetails[m_AgentConnectedAccount].IsAtService.compare("Yes") == 0))
            setServiceState();
        //RefreshAgentDetails();

        setAccountBalance(doGetAccBalanceOp(m_AccountInUse));
    }
}

void MainLoop::closeMshBoxAndWaitForShowMsgBox()
{
    emit closeMsgBoxSig();
    QApplication::processEvents(QEventLoop::AllEvents, 100);
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
}

void MainLoop::setAccountList(QString accountsStr, QString defAccount)
{
    if (!accountsStr.isEmpty())
    {
        m_QAccounts = accountsStr.split(GUIString::Spec);
        if (!defAccount.isEmpty())
        {
            for(QStringList::iterator iter = m_QAccounts.begin(); iter != m_QAccounts.end(); iter++)
            {
                if (iter->compare(defAccount) == 0)
                {
                    m_QAccounts.erase(iter);
                    m_QAccounts.insert(0, defAccount);
                    break;
                }
            }
        }
    }

    emit accountsChanged(m_QAccounts.size());
//    ui->accountList->setModel(m_QAccountsModel);
//    for (int i = 0; i < m_QAccounts.length(); i++)
//        ui->accountComboBox->addItem(m_QAccounts.at(i));
}

QString MainLoop::getDefaultAccount()
{
    QString defaultAccount = UiConfig().Get(CTTUi::AccountSettings, CTTUi::DefaultAccount).toString();
    if (defaultAccount.isEmpty())
    {
        if (m_QAccounts.size() > 0)
        {
            defaultAccount = static_cast<QString>(m_QAccounts.at(0));
            UiConfig().Set(CTTUi::AccountSettings, CTTUi::DefaultAccount, defaultAccount);
        }
    }

    if (!defaultAccount.isEmpty())
        return defaultAccount;
    else
        return "";
//    ui->defAccLineEdit->setText(defaultAccount);
//    ui->accountComboBox->setCurrentText(defaultAccount);
//    m_AccountInUse = defaultAccount;
}

void MainLoop::setAccountInUse(QString account)
{
    m_AccountInUse = account;
    emit accountChanged(account);
}

void MainLoop::addAccount(QString account)
{
    m_QAccounts.insert(0, account);

    setAccountList();
//    ui->accountList->setModel(m_QAccountsModel);
//    ui->accountComboBox->addItem(account);
}

void MainLoop::initAgentTestInfoModel()
{
    if (!m_AgentTestInitedFlag)
    {
        QString agnetInfo = "{\"AgentList\":[{\"AgentIP\":\"111.229.236.194\",\"AgentAddr\":\"Unkown(未知)\",\"PaymentMethod\":\"0\",\"Price\":\"0/0\"}]}";
        //agnetInfo = "{\"AgentList\":[{\"AgentIP\":\"118.89.165.39\",\"AgentAddr\":\"未知\",\"PaymentMethod\":\"0\",\"Price\":\"1/1\"},{\"AgentIP\":\"212.129.144.237\",\"AgentAddr\":\"未知 \",\"PaymentMethod\":\"0\",\"Price\":\"1/1\"},{\"AgentIP\":\"127.0.0.1\",\"AgentAddr\":\"未知  \",\"PaymentMethod\":\"0\",\"Price\":\"1/1\"},{\"AgentIP\":\"111.229.236.194\",\"AgentAddr\":\"未知   \",\"PaymentMethod\":\"0\",\"Price\":\"1/1\"},{\"AgentIP\":\"10.0.215.52\",\"AgentAddr\":\"未知    \",\"PaymentMethod\":\"0\",\"Price\":\"1/1\"}]}";
        AgentDetailsFunc::ParseAgentInfo(agnetInfo, m_AgentDetails, m_AgentTestInfo);
        m_QAgentsModel->SetModel(m_AgentTestInfo);
        m_AgentTestInitedFlag = true;
    }
}

void MainLoop::setAgentInfoModel(const std::map<QString, AgentInfo>& agentInfo)
{
    m_QAgentsModel->SetModel(agentInfo);
}

QString MainLoop::doGetSignedAgentDetailOp()
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::GetSignedAgentInfoCmd, vector<QString>{m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return m_GuiString->uiHints(GUIString::GetAgentDetailsErrHint);

    m_AgentDetailsRawData = QString::fromStdString(ret);
    if (!AgentDetailsFunc::ParseAgentDetails(m_AgentDetailsRawData, m_AgentDetails))
        return m_GuiString->uiHints(GUIString::ParseAgentDetailsErrHint);

//    for(map<QString, AgentDetails>::const_iterator it = m_AgentDetails.begin(); it != m_AgentDetails.end(); it++)
//    {
//        if (it->second.CsAddr.compare(m_AgentConnectedAccount) == 0)
//        {
//            cout<<"IP: "<<it->second.IPAddress.toStdString()<<endl;
//            cout<<"CsAddr: "<<it->second.CsAddr.toStdString()<<endl;
//            cout<<"IsAtService: "<<it->second.IsAtService.toStdString()<<endl;
//            cout<<"PaymentMeth: "<<it->second.PaymentMeth.toStdString()<<endl;
//            cout<<"TStart: "<<it->second.TStart.toStdString()<<endl;
//            cout<<"TEnd: "<<it->second.TEnd.toStdString()<<endl;
//            cout<<"FlowAmount: "<<it->second.FlowAmount.toStdString()<<endl;
//            cout<<"FlowUsed: "<<it->second.FlowUsed.toStdString()<<endl;
//        }
//    }

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::getAgentIPByAcc(const QString& account)
{
    for(map<QString, AgentInfo>::const_iterator it = m_AgentInfo.begin(); it != m_AgentInfo.end(); it++)
    {
        if (it->second.CsAddr.compare(account) == 0)
            return it->first;
    }
    for(map<QString, AgentInfo>::const_iterator it = m_AgentTestInfo.begin(); it != m_AgentTestInfo.end(); it++)
    {
        if (it->second.CsAddr.compare(account) == 0)
            return it->first;
    }
    return "";
}

void MainLoop::setAgentIPAddress(map<QString, AgentDetails>& agentDetails)
{
    for(map<QString, AgentDetails>::iterator it = agentDetails.begin(); it != agentDetails.end(); it++)
    {
        if (it->second.IPAddress.isEmpty())
            it->second.IPAddress = getAgentIPByAcc(it->first);
        if (it->second.CsAddr.compare(m_AgentConnectedAccount) == 0)
            it->second.IPAddress = m_AgentConnectedIP;
    }
}

void MainLoop::setAgentIPAddress(map<QString, AgentDetails>& agentDetails, QString ip, QString account)
{
    for(map<QString, AgentDetails>::iterator it = agentDetails.begin(); it != agentDetails.end(); it++)
    {
        if (it->second.IPAddress.compare(ip) == 0)
            it->second.CsAddr = account;
    }
}

void MainLoop::updateAgentInfoByIP(const map<QString, AgentDetails>& agentDetails, QString ipAddress)
{
    for(map<QString, AgentDetails>::const_iterator it = agentDetails.begin(); it != agentDetails.end(); it++)
    {
        if (it->second.IPAddress.compare(ipAddress) == 0)
        {
            if (m_AgentInfo.size() > 0)
            {
                m_AgentInfo[ipAddress].CsAddr = it->second.CsAddr;
                m_QAgentsModel->SetModel(m_AgentInfo);
            }
            else
            {
                m_AgentTestInfo[ipAddress].CsAddr = it->second.CsAddr;
                m_QAgentsModel->SetModel(m_AgentTestInfo);
            }
            return;
        }
    }
}

AgentDetails MainLoop::getAgentDetailsByIP(QString IP)
{
    QString agentAcc;
    if (IP.compare(m_AgentConnectedIP) == 0)
    {
        agentAcc = m_AgentConnectedAccount;
//        string ret = m_BackRPC->SendOperationCmd(GUIString::GetCurAgentAddrCmd, vector<QString>());
//        if (ret != m_GuiString->SysMsgs(GUIString::ErrorMsg) && !QString::fromStdString(ret).isEmpty())
//            agentAcc = QString::fromStdString(ret);
//        else
//            ShowOpErrorMsgBox();
    }
    else
        agentAcc = doGetAgentAccByIPOp(IP);
    if (agentAcc.isEmpty())
        return AgentDetails();
    AgentDetails details = getAgentDetailsByAcc(agentAcc);
    details.IPAddress = IP;
    return details;
}

QString MainLoop::getMyAgentNoneID()
{
    QString currentAgent = m_AgentConnectedAccount;
    std::map<QString, AgentDetails>::iterator iter = m_AgentDetails.find(currentAgent);
    if (iter == m_AgentDetails.end())
    {
        return "";
    }
    return iter->second.AgentNodeID;
}

QString MainLoop::doGetAgentAccByIPOp(QString IPAddress)
{
    if (m_AgentInfo.find(IPAddress) != m_AgentInfo.end())
        return m_AgentInfo[IPAddress].CsAddr;
    return "";
}

AgentDetails MainLoop::getAgentDetailsByAcc(const QString& account)
{
    AgentDetails details;
    details.CsAddr = account;
    if (m_AgentDetails.find(account) != m_AgentDetails.end())
        details = m_AgentDetails[account];
    else
    {
        details.TStart = QString("N/A");
        details.TEnd = QString("N/A");
        details.FlowAmount = QString("N/A");
        details.FlowUsed = QString("N/A");
        details.PaymentMeth = QString("N/A");
        details.Price = QString("N/A");
        details.IsAtService = QString("No");
    }
    return details;
}

QString MainLoop::getCurrAgentPayment()
{
    return m_AgentDetails[m_AgentConnectedAccount].PaymentMeth;
}

QString MainLoop::getCurrAgentTStart()
{
    QString tstart = m_AgentDetails[m_AgentConnectedAccount].TStart;
    return tstart.remove(10, tstart.length()-10);
}

QString MainLoop::getCurrAgentTStop()
{
    QString tstop = m_AgentDetails[m_AgentConnectedAccount].TEnd;
    return tstop.remove(10, tstop.length()-10);
}

QString MainLoop::getCurrAgentFlow()
{
    qint64 flow = m_AgentDetails[m_AgentConnectedAccount].FlowAmount.toLongLong();
    return QString::number((double(flow / 1000000) / double(GB_Bytes / 1000000)), 'f', 2);
}

QString MainLoop::getCurrAgentFlowRemain()
{
    qint64 flow = m_AgentDetails[m_AgentConnectedAccount].FlowAmount.toLongLong();
    qint64 used = m_AgentDetails[m_AgentConnectedAccount].FlowUsed.toLongLong();
    return QString::number((double((flow - used) / 1000000) / double(GB_Bytes / 1000000)), 'f', 2);
}

void MainLoop::setServiceState()
{
    m_AtServiceFlag = m_AgentDetails[m_AgentConnectedAccount].IsAtService.compare("Yes") == 0;
    if (!m_AtServiceFlag)
        emit uiStatusChanged(m_GuiString->uiHints(GUIString::AgentInvalidHint));
    else
        emit uiStatusChanged("");
    emit serviceStateChanged();
}

void MainLoop::setIsCreatingSpace(bool flag)
{
    m_IsCreatingSpaceFlag = flag;
    emit isCreaetingSpaceChanged();
}

void MainLoop::setIsRefreshingFiles(bool flag)
{
    m_IsRefreshingFilesFlag = flag;
    emit isRefreshingFilesChanged();
    if (flag)
        emit uiStatusChanged(m_GuiString->uiStatus(GUIString::RefreshingFilesStatus));
    else
        emit uiStatusChanged("");
}

void MainLoop::setBusyingFlag(bool state, QString status)
{
    m_Busying = state;
    emit uiStatusChanged(status);
}

bool MainLoop::doSignAgentOp(QString csAddr, QString expireTime, QString flowAmount, QString password, QString& result)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::SignAgentCmd, vector<QString>{csAddr,
                expireTime, flowAmount, password});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        result = m_GuiString->uiHints(GUIString::OperationFailedHint);
        return false;
    }

    result = QString::fromStdString(ret);
    return true;
}

bool MainLoop::doCancelAgentOp(QString agentAcc, QString password, QString& result)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::CancelAgentCmd, vector<QString>{agentAcc, password});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        result = m_GuiString->uiHints(GUIString::OperationFailedHint);
        return false;
    }

    result = QString::fromStdString(ret);
    return true;
}

void MainLoop::updateAgentDetails(QString agentAccount, QString TXtoMonitor, GUIString::RpcCmds cmd, bool newState)
{
    emit showMsgBoxSig(m_GuiString->uiHints(GUIString::WaitingTXCompleteHint), GUIString::WaitingDlg);
    m_AgentDetails[agentAccount].PaymentMeth = m_GuiString->uiHints(GUIString::WaitingForCompleteHint);
    m_AgentDetails[agentAccount].TStart = "";
    m_AgentDetails[agentAccount].TEnd = "";
    m_AgentDetails[agentAccount].FlowUsed = "";
    m_AgentDetails[agentAccount].FlowAmount = "";
    emit serviceStateChanged();

    if (!doWaitTXCompleteOp(TXtoMonitor, cmd))
        return;
    else
    {
        closeMshBoxAndWaitForShowMsgBox();

        QString success = "";
        if (newState)
            success = m_GuiString->uiHints(GUIString::AgentApplySucceededHint) + ", " +
                m_GuiString->uiHints(GUIString::TXIDHint) + TXtoMonitor;
        else
            success = m_GuiString->uiHints(GUIString::AgentCancelSucceededHint) + ", " +
                m_GuiString->uiHints(GUIString::TXIDHint) + TXtoMonitor;

        emit showMsgBoxSig(success, GUIString::HintDlg, true);
    }

    if (newState)
        doGetSignedAgentDetailOp();
    else
    {
        m_AgentDetails[agentAccount].PaymentMeth = "";
        m_AgentDetails[agentAccount].TStart = "";
        m_AgentDetails[agentAccount].TEnd = "";
        m_AgentDetails[agentAccount].FlowUsed = "";
        m_AgentDetails[agentAccount].FlowAmount = "";
        m_AgentDetails[agentAccount].IsAtService = "No";
    }
    if (m_AgentConnectedAccount.compare(agentAccount) == 0)
    {
        setServiceState();
        if (newState)
            updateFileList();
    }
}

bool MainLoop::doWaitTXCompleteOp(QString TXtoMonitor, GUIString::RpcCmds cmdType)
{
    if (cmdType == GUIString::SignAgentCmd || cmdType == GUIString::CancelAgentCmd)
    {
        string ret = m_BackRPC->SendOperationCmd(GUIString::WaitTXCompleteCmd, vector<QString>{TXtoMonitor});
        if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
                ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        {
            closeMshBoxAndWaitForShowMsgBox();
            emit showMsgBoxSig(m_GuiString->uiHints(GUIString::TXTimedoutHint), GUIString::HintDlg);
            return false;
        }

        return true;
    }
    else if(cmdType == GUIString::CreateObjCmd)
    {
        string ret = m_BackRPC->SendOperationCmd(GUIString::WaitTXCompleteCmd, vector<QString>{TXtoMonitor});
        if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
                ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        {
            closeMshBoxAndWaitForShowMsgBox();
            setIsCreatingSpace(false);
            setBusyingFlag(false, "");
            emit showMsgBoxSig(m_GuiString->uiHints(GUIString::TXTimedoutHint), GUIString::HintDlg);
            return false;
        }

        QString newMetaData = getNewMetaData(m_NewSpaceLabel, m_NewSpaceSize);
        QString currentReformatMetaDatas = UiConfig().Get(CTTUi::DiskMetaDataBackup, CTTUi::AllReformatMetaData).toString();
        QVariant value1(Utils::AddReformatMetaDataJson(m_AccountInUse, m_NewObjID, currentReformatMetaDatas, newMetaData));
        UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::AllReformatMetaData, value1);

        QVariant value2(Utils::GenMetaDataJson(m_AccountInUse, m_NewObjID, newMetaData));
        UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::IncompletedMetaData, value2);

        m_CreateSpaceTX.clear();

        waitingForNewSpaceCreating(TXtoMonitor);
        return true;
    }
    return true;
}

QString MainLoop::doGetAccBalanceOp(QString account)
{
    vector<QString> accParam;
    GUIString::RpcCmds cmd;
    QString errHint;
    accParam.push_back(account);
    cmd = GUIString::GetAccBalanceCmd;
    errHint = m_GuiString->uiHints(GUIString::GetAccBalanceErrHint);

    string ret = m_BackRPC->SendOperationCmd(cmd, accParam);
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return errHint;

    return QString::fromStdString(ret);
}

void MainLoop::setAccountBalance(QString bal)
{
    if (bal.compare(m_GuiString->uiHints(GUIString::GetAccBalanceErrHint)) == 0)
        return;

    m_AccBalance = bal;
    emit balanceChanged(bal);
}

void MainLoop::setDefaultAccount(QString account)
{
//    ui->defAccLineEdit->setText(m_AccountInUse);
//    ui->accountComboBox->setCurrentText(m_AccountInUse);

    UiConfig().Set(CTTUi::AccountSettings, CTTUi::DefaultAccount, account);
}

void MainLoop::copyToClipboard(QString text)
{
    QClipboard* clipboard = QApplication::clipboard();
    clipboard->setText(text);
}

void MainLoop::copyTxIDToClipboard(QString text)
{
    QStringList list = text.split("0x");
    if (list.size() == 1)
    {
        QClipboard* clipboard = QApplication::clipboard();
        clipboard->setText(text);
    }
    else if(list.size() == 2)
    {
        QClipboard* clipboard = QApplication::clipboard();
        clipboard->setText("0x" + list.at(1));
    }
}

QString MainLoop::getAgentInfo()
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::GetAgentsInfoCmd, vector<QString>{});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        initAgentTestInfoModel();
        return QString::fromStdString(ret);
    }

    if (!AgentDetailsFunc::ParseAgentInfo(QString::fromStdString(ret), m_AgentDetails, m_AgentInfo))
        return m_GuiString->sysMsgs(GUIString::ParseAgentsFailedMsg);

    setAgentInfoModel(m_AgentInfo);

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

bool MainLoop::verifyPassword(QString account, QString password)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::LoginAccountCmd, vector<QString>{account, password});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::OkMsg).toStdString()) == 0)
    {
        setAccountInUse(account);
        m_AccountPwd = password;
        setDefaultAccount(account);
        return true;
    }
    else
        return false;
}

QString MainLoop::getAutoConnectAgentServ()
{
    return UiConfig().Get(CTTUi::AutoConnectSettings, CTTUi::AgentServerIP).toString();
}

void MainLoop::setAutoConnectAgentServ(QString ip)
{
    UiConfig().Set(CTTUi::AutoConnectSettings, CTTUi::AgentServerIP, ip);
}

void MainLoop::unsetAutoConnectAgentServ()
{
    UiConfig().Set(CTTUi::AutoConnectSettings, CTTUi::AgentServerIP, QString(""));
}


QString MainLoop::connectAgentServ(QString agentIP)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::RemoteIPCmd, vector<QString>{agentIP});
    if (ret != m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString() &&
            ret != m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString())
    {
        m_HasConnFlag = true;
        m_AgentConnectedAccount = QString::fromStdString(ret);
        m_AgentConnectedIP = agentIP;
        QString returnStr = m_GuiString->sysMsgs(GUIString::OkMsg);

        getSignedAgentDetail(m_AgentConnectedAccount);
        updateAgentInfoByIP(m_AgentDetails, m_AgentConnectedIP);
//        for(map<QString, AgentDetails>::const_iterator it = m_AgentDetails.begin(); it != m_AgentDetails.end(); it++)
//        {
//            if (it->second.CsAddr.compare(m_AgentConnectedAccount) == 0)
//            {
//                cout<<"IP: "<<it->second.IPAddress.toStdString()<<endl;
//                cout<<"CsAddr: "<<it->second.CsAddr.toStdString()<<endl;
//                cout<<"IsAtService: "<<it->second.IsAtService.toStdString()<<endl;
//                cout<<"PaymentMeth: "<<it->second.PaymentMeth.toStdString()<<endl;
//                cout<<"TStart: "<<it->second.TStart.toStdString()<<endl;
//                cout<<"TEnd: "<<it->second.TEnd.toStdString()<<endl;
//                cout<<"FlowAmount: "<<it->second.FlowAmount.toStdString()<<endl;
//                cout<<"FlowUsed: "<<it->second.FlowUsed.toStdString()<<endl;
//            }
//        }
        if (m_AgentDetails.find(QString::fromStdString(ret)) != m_AgentDetails.end() &&
            m_AgentDetails[QString::fromStdString(ret)].IsAtService.compare("Yes") == 0)
            setServiceState();
        else
        {
            returnStr = m_GuiString->uiHints(GUIString::AgentInvalidHint);
            emit uiStatusChanged(m_GuiString->uiHints(GUIString::AgentInvalidHint));
        }

        setAccountBalance(doGetAccBalanceOp(m_AccountInUse));
        m_RefreshTimerId = startTimer(timeRefreshInterv);

        std::thread t(bind(&MainLoop::progressUpdatingLoop, this));
        t.detach();

        return returnStr;
    }

    return m_GuiString->uiHints(GUIString::ConnFailedHint);
}

QString MainLoop::getAgentIP(int index)
{
    return m_QAgentsModel->GetAgentIP(index);
}

QString MainLoop::getAgentAccount(int index, bool shortMode)
{
    QString account = m_QAgentsModel->GetAgentAccount(index);
    if (shortMode)
        account = account.replace(10, account.length()-20, "‧‧‧");

    return account;
}

QString MainLoop::getAgentAvailablePayment(int index)
{
    return m_QAgentsModel->GetAgentAvailablePayment(index);
}

QString MainLoop::getCurrAgentAccount()
{
    return m_AgentConnectedAccount;
}

QString MainLoop::getSignedAgentDetail(QString agentConnectedAccount)
{
    QString ret = doGetSignedAgentDetailOp();
    if (ret.compare(m_GuiString->sysMsgs(GUIString::OkMsg)) != 0)
        return ret;

    setAgentIPAddress(m_AgentDetails);
    if (!agentConnectedAccount.isEmpty())
        setAgentIPAddress(m_AgentDetails, m_AgentConnectedIP, agentConnectedAccount);
    return ret;
}

QString MainLoop::getSpaceLabelByIndex(int index)
{
    if (index < 0)
        return m_GuiString->sysMsgs(GUIString::SharingSpaceLabel);
    return m_QSapceListModel->GetSpaceLabel(index);
}

QString MainLoop::showFileList(QString spaceLabel)
{
    if (spaceLabel.isEmpty())
        return refreshFiles(spaceLabel);
    //else
    //    m_CurrentSpace = spaceLabel;
    if (m_SpaceReformatInfos.find(spaceLabel) != m_SpaceReformatInfos.end())
    {
        QString hint = m_GuiString->uiHints(GUIString::ParseMetaErrHint) + ". " +
                m_GuiString->uiHints(GUIString::NeedToBeReformattedHint);
        m_ReformatMetaJson = m_SpaceReformatInfos.find(spaceLabel)->second;
        emit showYesNoBoxSig(hint);
    }

    showFileTableAndInfo(spaceLabel, m_SpaceInfos[spaceLabel].second);
    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::updateFileList(QString spaceLabel)
{
//    QString abc("{\"Sharing\":false,\"ObjId\":\"1\",\"Size\":\"256\"}/{\"Sharing\":false,\"ObjId\":\"2\",\"Size\":\"128\"}/{\"Sharing\":true,\"Owner\":\"0x5f663f10F12503Cb126Eb5789A9B5381f594A0eB\",\"ObjId\":\"3\",\"Offset\":\"64\",\"Length\":\"128\",\"StartTime\":\"2019-01-01 00:00:00\",\"StopTime\":\"2020-01-01 00:00:00\",\"FileName\":\"123.txt\",\"SharerCSNodeID\":\"...\"}");
//    QStringList list = abc.split(CTTUi::spec);
//    for(int i = 0; i < list.length(); ++i)
//    {
//        ObjInfo info;
//        info.DecodeQJson(list.at(i));
//        if (info.isSharing)
//            cout<<"Sharing: true"<<endl;
//        else
//            cout<<"Sharing: false"<<endl;
//        cout<<"ObjId: "<<info.objectID.toStdString()<<endl;
//        cout<<"Size: "<<info.objectSize.toStdString()<<endl;
//        cout<<"Owner: "<<info.owner.toStdString()<<endl;
//        cout<<"SharerCSNodeID: "<<info.sharerCSNodeID.toStdString()<<endl;
//        cout<<"==============================="<<endl;
//    }

    return refreshFiles(spaceLabel);

    //EnableElements();
}

QString MainLoop::reformatSpace()
{
    QString account = "";
    QString metaData = "";
    qint64 objID = 0;
    if (!m_ReformatMetaJson.isEmpty() && Utils::ParseMetaDataJson(m_ReformatMetaJson, account, objID, metaData))
    {
        if (account.compare(m_AccountInUse) == 0)
        {
            if (writeMetaDataOp(objID, metaData))
            {
                m_ReformatMetaJson = "";
                return m_GuiString->sysMsgs(GUIString::OkMsg);
            }
        }
    }

    return m_GuiString->uiHints(GUIString::WriteMetaDataErrHint);
}

QString MainLoop::sendTransaction(QString receiver, QString amount, QString passwd)
{
    if (passwd.compare(m_AccountPwd) != 0)
        return m_GuiString->uiHints(GUIString::IncorrecctPasswdHint);

    double number = amount.toDouble();
    if (number > m_AccBalance.toDouble())
        return m_GuiString->uiHints(GUIString::TxAmountExceededHint);

//    QMessageBox::StandardButton button;
//    button = QMessageBox::question(this, QString::fromStdString(m_StringsTable[Ui::programTitle]),
//            QString::fromStdString(m_UIHints[Ui::sendTransactionHint]),
//            QMessageBox::Yes|QMessageBox::No);
//    if(button == QMessageBox::No)
//        return;

    string ret = m_BackRPC->SendOperationCmd(GUIString::SendTransactionCmd, vector<QString>{m_AccountInUse,
                receiver, amount, m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return m_GuiString->uiHints(GUIString::TxSentFailedHint);
    else
        return (m_GuiString->uiHints(GUIString::TxSuccessfullySentHint) + ", " +
                m_GuiString->uiHints(GUIString::TXIDHint) + QString::fromStdString(ret));
}

QString MainLoop::preApplyForAgentService(QString agentAccount, bool paymentTimeFlag,
                                          QString timePeriod, QString flowAmount, bool isNotCurrtAgent)
{
    if (agentAccount.isEmpty())
        return m_GuiString->uiHints(GUIString::OperationFailedHint);

    if (!paymentTimeFlag)
        timePeriod = "0000-00-00 00:00:00~0000-00-00 00:00:00";

    if(!paymentTimeFlag && flowAmount.isEmpty())
        return m_GuiString->uiHints(GUIString::OperationFailedHint);

    m_PreServApplyAccount = agentAccount;
    m_PreServApplyTimePeriod = timePeriod;
    m_PreServApplyFlowAmount = flowAmount;

    if (isNotCurrtAgent)
    {
        emit showYesNoBox(m_GuiString->uiHints(GUIString::AgentApplyNotCurrentHint));
        return m_GuiString->sysMsgs(GUIString::OkMsg);
    }
    else
        return applyForAgentService();
}

QString MainLoop::applyForAgentService()
{
    QString ret;
    //flowAmount = "0"; //QString::number(flow.toLongLong() * GB_Bytes);
    //timePeriod = "2020-01-01 00:00:00~2020-12-31 23:59:59";

    if (m_AccBalance.toDouble() <= 0)
        return m_GuiString->uiHints(GUIString::TxAmountTooLowHint);

    showMsgBoxSig(m_GuiString->uiHints(GUIString::AgentApplyStartedHint), GUIString::WaitingDlg);
    if (!doSignAgentOp(m_PreServApplyAccount, m_PreServApplyTimePeriod,
                       QString::number(m_PreServApplyFlowAmount.toLongLong() * GB_Bytes), m_AccountPwd, ret))
    {
        closeMsgBox();
        return m_GuiString->uiHints(GUIString::AgentApplyFailedHint);
    }
    else
    {
        std::thread t(bind(&MainLoop::updateAgentDetails, this, m_PreServApplyAccount, ret, GUIString::SignAgentCmd, true));
        t.detach();
        ret = m_GuiString->sysMsgs(GUIString::OkMsg);
        return ret;
    }
}

QString MainLoop::cancelAgentService(QString agentAccount)
{
    QString IP = m_AgentDetails[agentAccount].IPAddress;
    QString ret;
    showMsgBoxSig(m_GuiString->uiHints(GUIString::AgentCancelStartedHint), GUIString::WaitingDlg);
    if (!doCancelAgentOp(agentAccount, m_AccountPwd, ret))
    {
        closeMsgBox();
        return m_GuiString->uiHints(GUIString::AgentCancelFailedHint);
    }
    else
    {
        std::thread t(bind(&MainLoop::updateAgentDetails, this, agentAccount, ret, GUIString::CancelAgentCmd, false));
        t.detach();
        ret = m_GuiString->sysMsgs(GUIString::OkMsg);
        return ret;
    }
}

QString MainLoop::createSpace(QString spaceLabel, QString spaceSize)
{
    if (m_SpaceInfos.find(spaceLabel) != m_SpaceInfos.end())
        return m_GuiString->uiHints(GUIString::DuplicatedSpaceNameHint);

    if (double(spaceSize.toLongLong() * 1) >= m_AccBalance.toDouble()) //TODO: price is 1
        return m_GuiString->uiHints(GUIString::TxAmountTooLowHint);

    setIsCreatingSpace(true);
    m_NewSpaceSize = spaceSize.toLongLong() * MB_Bytes;
    string ret = m_BackRPC->SendOperationCmd(GUIString::CreateObjCmd, vector<QString>{QString::number(m_ObjInfos.size()+1),
                QString::number(m_NewSpaceSize), m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        setIsCreatingSpace(false);
        return m_GuiString->uiHints(GUIString::OperationFailedHint);
    }
    else
    {
        //cout<<ret<<endl;

        QString qRet = QString::fromStdString(ret);
        QStringList list = qRet.split(CTTUi::spec);
        if (list.length() > 0)
        {
            m_CreateSpaceTX = list.at(0);
            setBusyingFlag(true, m_GuiString->uiStatus(GUIString::CreatingSpaceText) +
                           m_GuiString->uiStatus(GUIString::TxSendedStatus) + ": " + m_CreateSpaceTX);

            GUIString::RpcCmds cmd = GUIString::CreateObjCmd;
            std::thread t(bind(&MainLoop::doWaitTXCompleteOp, this, m_CreateSpaceTX, cmd));
            t.detach();
        }
        else
        {
            setIsCreatingSpace(false);
            return m_GuiString->uiHints(GUIString::TxReturnErrorHint);
        }

        m_WaitingSpaceFlag = true;
        m_NewSpaceLabel = spaceLabel;
        m_NewObjID = int(m_ObjInfos.size()) + 1;
    }

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

void MainLoop::showDownloadDlg(bool isShare)
{
    if (m_DownloadDlg == nullptr)
        return;

    m_DownloadDlg->setProperty("visible", true);
    int spaceIdx = m_CurrentSpaceIdx;
    if (isShare)
        spaceIdx = -1;
    m_DownloadDlg->setProperty("currentSpaceIdx", spaceIdx);
    m_DownloadDlg->setProperty("currentFileIdx", m_CurrentFileIdx);
    QObject *fileNameText = m_DownloadDlg->findChild<QObject*>("fileNameText");
    fileNameText->setProperty("text", getFileNameByIndex(m_CurrentFileIdx));
    QObject *fileSizeText = m_DownloadDlg->findChild<QObject*>("fileSizeText");
    fileSizeText->setProperty("text", getFileSizeByIndex(spaceIdx, m_CurrentFileIdx));
}

int MainLoop::getSpaceIdx(QString spaceLabel)
{
    if (spaceLabel.isEmpty())
        return m_CurrentSpaceIdx;
    else if(spaceLabel.compare(m_GuiString->sysMsgs(GUIString::SharingSpaceLabel)) == 0)
        return -1;
        //m_QSapceListModel->GetSpaceIndex(m_CurrentSpace);
    return m_QSapceListModel->GetSpaceIndex(spaceLabel);
}

QString MainLoop::getFileNameByIndex(int fileIndex)
{
    return m_QFileListModel->GetFileName(fileIndex);
}

QString MainLoop::getFileSizeByIndex(int spaceIndex, int fileIndex)
{
    QString fName = getFileNameByIndex(fileIndex);
    QString spaceLabel = getSpaceLabelByIndex(spaceIndex);
    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr = "0";
    QString sharerAgentNodeID = "0";
    QString key = "0";
    Utils::GetFileCompleteInfo(&m_VecFileList, &m_ObjInfos, fName, spaceLabel,
                               offset, length, sharerAddr, sharerAgentNodeID, key,
                               spaceLabel.compare(m_GuiString->sysMsgs(GUIString::SharingSpaceLabel)) == 0);
    qint64 unit = 0;
    return Utils::AutoSizeString(length, unit);
}

QString MainLoop::downloadFileByIndex(QString filePath, int spaceIndex, int fileIndex)
{
    QString fName = getFileNameByIndex(fileIndex);
    QString spaceLabel = getSpaceLabelByIndex(spaceIndex);

    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr = "0";
    QString sharerAgentNodeID = "0";
    QString key = "0";
    qint64 objID = Utils::GetFileCompleteInfo(&m_VecFileList, &m_ObjInfos, fName, spaceLabel,
                                              offset, length, sharerAddr, sharerAgentNodeID, key,
                                              spaceLabel.compare(m_GuiString->sysMsgs(GUIString::SharingSpaceLabel)) == 0);
    if (objID == 0 && sharerAddr == "0")
        return m_GuiString->uiHints(GUIString::FileInfoNotFoundHint);
    if (offset == 0 || length == 0)
        return m_GuiString->uiHints(GUIString::FileInfoNotFoundHint);
    if (!AgentDetailsFunc::CheckIfAtService(m_AgentDetails[m_AgentConnectedAccount]))
        return m_GuiString->uiHints(GUIString::AgentInvalidHint);
    if (!AgentDetailsFunc::CheckFlow(m_AgentDetails[m_AgentConnectedAccount], length))
        return m_GuiString->uiHints(GUIString::NeedMoreFlowHint);

    setBusyingFlag(true, m_GuiString->uiStatus(GUIString::DownloadingStatus));
    std::thread t(bind(&MainLoop::doDownloadFileOp, this, filePath, fName, objID, offset, length, sharerAddr, sharerAgentNodeID, key));
    t.detach();

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::getDirPathFromUrl(QString url)
{
    return url.replace("file:///", "");
}

QString MainLoop::uploadFileByIndex(QString filePath, int spaceIndex)
{
    qint64 objID = Utils::GetSpaceObjID(&m_ObjInfos, m_QSapceListModel->GetSpaceLabel(spaceIndex));
    if (objID == 0)
        return m_GuiString->uiHints(GUIString::OperationFailedHint);

    QFileInfo info(filePath);
    qint64 offset = 0;
    if (!AgentDetailsFunc::CheckFlow(m_AgentDetails[m_AgentConnectedAccount], info.size()))
        return m_GuiString->uiHints(GUIString::NeedMoreFlowHint);
    if (info.size() >= getRemainingSpaceSizeByIndex(spaceIndex))
        return m_GuiString->uiHints(GUIString::NotEnoughSpaceToUpHint);
    if (!checkFileName(spaceIndex, info.fileName()))
        return m_GuiString->uiHints(GUIString::DuplicatedFileNameHint);


    std::thread t(bind(&MainLoop::doUploadFileOp, this, objID, offset, info.fileName(), info.size(), filePath));
    t.detach();

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::shareFileToAccount(int spaceIndex, int fileIndex, QString receiver,
                                     QString startTime, QString stopTime, QString price)
{
    QString fName = getFileNameByIndex(fileIndex);
    QString spaceLabel = getSpaceLabelByIndex(spaceIndex);

    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr = "0";
    QString sharerAgentNodeID = "0";
    QString key = "0";
    qint64 objID = Utils::GetFileCompleteInfo(&m_VecFileList, &m_ObjInfos, fName, spaceLabel,
                                              offset, length, sharerAddr, sharerAgentNodeID, key,
                                              spaceLabel.compare(m_GuiString->sysMsgs(GUIString::SharingSpaceLabel)) == 0);
    if (objID == 0 && sharerAddr == "0")
    {
        //emit ShowDownFileOpRetSign(m_GuiString->sysMsgs(GUIString::ErrorMsg));
        return m_GuiString->uiHints(GUIString::FileInfoNotFoundHint);
    }
    if (offset == 0 || length == 0)
    {
        //emit ShowDownFileOpRetSign(m_GuiString->sysMsgs(GUIString::ErrorMsg));
        return m_GuiString->uiHints(GUIString::FileInfoNotFoundHint);
    }
    sharerAgentNodeID = getMyAgentNoneID();

    QString sharingCode = generateSharingCode(objID, fName, offset, length, receiver,
                                              startTime, stopTime, price, sharerAgentNodeID);
    if (sharingCode.compare("0") == 0)
        return m_GuiString->uiHints(GUIString::GenSharingCodeFailedHint);

    setSharingCode(sharingCode);

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::getSharingInfo(QString sharingCode)
{
    QString fName;
    QString spaceLabel;
    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr, receiver;
    QString sharerAgentNodeID;
    QString key;
    qint64 objID;
    QString price;
    QString startTime, stopTime, signature;

    FileSharingInfo fileJson;
    fileJson.DecodeQJson(sharingCode);
    fileJson.GetValues(objID, fName, offset, length, receiver, startTime, stopTime, price,
                       sharerAgentNodeID, key, signature);

    std::string sharer = m_BackRPC->SendOperationCmd(GUIString::CheckSignatureCmd,
            std::vector<QString>{sharingCode});
    if (sharer.compare("") == 0 || sharer.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0)
        return m_GuiString->sysMsgs(GUIString::ErrorMsg);

    setSharingFileInfo(QString::fromStdString(sharer), fName, startTime, stopTime, price);

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::payforSharing(QString sharingCode)
{
    FileSharingInfo fileJson;
    fileJson.DecodeQJson(sharingCode);
    std::string sharer = m_BackRPC->SendOperationCmd(GUIString::CheckSignatureCmd,
            std::vector<QString>{sharingCode});
//    if (sharer.compare(m_AccountInUse.toStdString()) == 0)
//        return m_GuiString->uiHints(GUIString::PayForSelfSharingHint);

    std::string ret = m_BackRPC->SendOperationCmd(GUIString::PayForSharedFileCmd,
            std::vector<QString>{sharingCode, m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return m_GuiString->uiHints(GUIString::OperationFailedHint);
    else
        return (m_GuiString->uiHints(GUIString::PayForSharingSuccHint) + ". " +
                m_GuiString->uiHints(GUIString::TXIDHint) + QString::fromStdString(ret));
}

QString MainLoop::refreshMails()
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::GetMailsCmd, vector<QString>{m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return m_GuiString->uiHints(GUIString::GetMailsErrorHint);

    //QString abc("{\"Title\":\"title_1\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"1...1\",\"TimeStamp\":\"2019-01-01 00:05:00\",\"FileSharing\":\"\",\"Signature\":\"\"}/{\"Title\":\"title_2\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"2...2\",\"TimeStamp\":\"2019-01-02 00:00:00\",\"FileSharing\":\"\",\"Signature\":\"...\"}/{\"Title\":\"title_3\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"3...3\",\"TimeStamp\":\"2019-01-03 00:05:00\",\"FileSharing\":\"{\\\"FileName\\\":\\\"Qt5Networkd.dll\\\",\\\"FileOffset\\\":\\\"16352632\\\",\\\"FileSize\\\":\\\"94562078\\\",\\\"ObjectID\\\":1,\\\"Price\\\":\\\"0.12\\\",\\\"Receiver\\\":\\\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\\\",\\\"SharerCSNodeID\\\":\\\"\\\",\\\"Signature\\\":\\\"0x51eb9f01c56e9be964a2885187e97135336667c1ae511f3961550b2ee4a486f91b990700ffbfb524094b32562fdae6dde6e302f35fd8f0aaf63697ec49e439dc00\\\",\\\"TimeStart\\\":\\\"2019-01-01 00:00:00\\\",\\\"TimeStop\\\":\\\"2019-01-01 00:00:00\\\"}\",\"Signature\":\"...\"}");
    //ret = abc.toStdString();
    QString mailDetals = QString::fromStdString(ret);
    if (!mailDetals.isEmpty() && mailDetals.compare("null") != 0)
    {
        parseMailList(mailDetals);
        setMailListModel(m_MailList);
    }
    else
    {
        QList<MailDetails> emptyList;
        setMailListModel(emptyList);
    }

    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

QString MainLoop::getMailTitle(int index)
{
    return m_QMailListModel->GetMailTitle(index);
}

QString MainLoop::getMailSender(int index)
{
    return m_QMailListModel->GetMailSender(index);
}

QString MainLoop::getMailTimeStamp(int index)
{
    return m_QMailListModel->GetMailTimeStamp(index);
}

QString MainLoop::getMailContent(int index)
{
    return m_QMailListModel->GetMailContent(index);
}

QString MainLoop::sendNewMail(QString title, QString receiver, QString content, QString fileSharingCode)
{
    MailDetails mail;
    QDateTime time = QDateTime::currentDateTime();
    QString timeStr = time.toUTC().toString("yyyy-MM-dd hh:mm:ss");
    mail.SetValues(title, m_AccountInUse, receiver, content, timeStr, fileSharingCode);
    QString mailJson = mail.EncodeQJson();

    emit showMsgBoxSig(m_GuiString->uiHints(GUIString::MailSendingHint), GUIString::WaitingDlg);

    string ret = m_BackRPC->SendOperationCmd(GUIString::GetSignatureCmd, std::vector<QString>{mailJson, m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        closeMshBoxAndWaitForShowMsgBox();
        return m_GuiString->uiHints(GUIString::GetSignatureFailedHint);
    }

    MailDetails mailSigned;
    mailSigned.DecodeQJson(mailJson);
    mailSigned.SetSignature(QString::fromStdString(ret));

    QString mailSignedStr = mailSigned.EncodeQJson();
    ret = m_BackRPC->SendOperationCmd(GUIString::SendMailCmd, vector<QString>{mailSignedStr, m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
    {
        closeMshBoxAndWaitForShowMsgBox();
        return m_GuiString->uiHints(GUIString::OperationFailedHint);
    }
    else
    {
        closeMshBoxAndWaitForShowMsgBox();
        return (m_GuiString->uiHints(GUIString::MailSuccessfullySentHint) + ", " +
                m_GuiString->uiHints(GUIString::TXIDHint) + QString::fromStdString(ret));
    }
}

void MainLoop::preExit()
{
    m_PreExitFlag = true;
    emit showYesNoBox(m_GuiString->uiHints(GUIString::MakeSureToLeaveHint));
}

qint64 MainLoop::getRemainingSpaceSizeByIndex(int spaceIndex)
{
    QString label = getSpaceLabelByIndex(spaceIndex);
    if (!label.isEmpty())
    {
        if (m_SpaceInfos.find(label) != m_SpaceInfos.end())
            return m_SpaceInfos[label].first;
    }

    return -1;
}

bool MainLoop::getCorruptedSpaceInfo(QString account, qint64 objId, map<qint64, pair<qint64, QString> >& objInfos,
                                     map<QString, pair<qint64, qint64> >& spaceInfos,
                                     map<QString, QString>& spaceReformatInfos)
{
    std::map<qint64, QString> idMetaData;
    if (Utils::ParseReformatMetaDataJsons(UiConfig().Get(CTTUi::DiskMetaDataBackup, CTTUi::AllReformatMetaData).toString(),
                                          account, idMetaData))
    {
        if (idMetaData.find(objId) != idMetaData.end())
        {
            QString label; qint64 spaceTotal, spaceRemain;
            if (FileManager::CheckHead4K(idMetaData.find(objId)->second, label, spaceTotal, spaceRemain))
            {
                label = label + m_GuiString->uiHints(GUIString::IncorrectSpaceMetaHint);
                objInfos[objId].second = label;
                spaceInfos[label] = std::pair<qint64, qint64>(spaceRemain, spaceTotal);
                spaceReformatInfos[label] = Utils::GenMetaDataJson(account, objId, idMetaData.find(objId)->second);
            }
            else
                return false;
        }
        else
            return false;
    }
    else
        return false;

    return true;
}

QString MainLoop::refreshFiles(QString spaceLabel)
{
    m_ObjInfos.clear();
    m_VecFileList.clear();
    //ui->curSpaceComboBox->clear();

    return getFileList(spaceLabel);
}

QString MainLoop::getFileList(QString spaceLabel)
{
    setIsRefreshingFiles(true);
    string ret = m_BackRPC->SendOperationCmd(GUIString::GetObjInfoCmd, vector<QString>{m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return m_GuiString->uiHints(GUIString::GetObjInfoErrHint);
    else
    {
        //cout<<ret<<endl; ///////////////////////////////////////debug
        QString qRet = QString::fromStdString(ret);
        //qRet = "{\"Sharing\":true,\"ObjId\":\"1\",\"Owner\":\"0xA4CF24068A18ac6C98D4Aa8D7CC8E78DBc890005\",\"Offset\":\"4096\",\"Length\":\"3\",\"StartTime\":\"2020-01-01 00:00:00\",\"StopTime\":\"2020-03-01 23:59:59\",\"FileName\":\"1.txt\",\"SharerCSNodeID\":\"0c627764bbfaedba641f4a667107a66bc4703c7f03f5f62efbe67e5970fad9259b8681d3938d9cb3f22ae60d6f72ed927a58be399597d0837905fbaeafc6c9fc\",\"Key\":\"3a860e2d\"}";
        QList<FileInfo> sharingList;
        if (!FileManager::ParseObjList(qRet, sharingList, m_ObjInfos))
            return m_GuiString->uiHints(GUIString::ParseObjErrHint);

        if (sharingList.size() > 0)
            m_VecFileList.push_back(pair<QList<FileInfo>, qint64>(sharingList, 0));
    }

    qint64 remainTemp = 0;
    QString readMetaDataRet = "";
    map<qint64, pair<qint64, QString> >::iterator iter;
    for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); iter++)
    {
        DelayMS(delayTimeInterv);
        qint64 totalSpaceTemp = iter->second.first;
        readMetaDataByObjID(iter->first, totalSpaceTemp, remainTemp, readMetaDataRet);

        //if (!readMetaDataByObjID(iter->first, totalSpaceTemp, remainTemp, readMetaDataRet))
        //{
        //    if (!getCorruptedSpaceInfo(m_AccountInUse, iter->first, m_ObjInfos, m_SpaceInfos))
        //        return readMetaDataRet;
        //}
        //return readMetaDataRet;
        //cout<<"objID: "<<iter->first<<", objName: "<<iter->second.second.toStdString()<<endl; //debug
    }
    setSpaceListModel(m_ObjInfos);
    if (m_VecFileList.size() > 0)
        showFileTableAndInfo(spaceLabel, m_SpaceInfos[spaceLabel].second);

    emit currentSpaceChanged(getSpaceIdx(spaceLabel));
    setIsRefreshingFiles(false);
    return m_GuiString->sysMsgs(GUIString::OkMsg);
}

void MainLoop::setSpaceListModel(const std::map<qint64, std::pair<qint64, QString> >& objInfos)
{
    m_QSapceListModel->SetModel(objInfos);
}

void MainLoop::clearFileListModel()
{
    m_QFileListModel->ClearData();
}

void MainLoop::addFileListModel(const QList<FileInfo>& files)
{
    m_QFileListModel->AddModel(files);
}

void MainLoop::setFileListModel(const QList<FileInfo>& files)
{
    m_QFileListModel->SetModel(files);
}

bool MainLoop::doReadMetaDataOp(qint64 objID, QString& metaData)
{
    string ret = m_BackRPC->SendOperationCmd(GUIString::BlizReadCmd,
            vector<QString>{QString::number(objID), "0", QString::fromStdString(fHeadLengthStr), m_AccountPwd});
    if (ret.length() <= 128 && (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0))
    {
        metaData = m_GuiString->uiHints(GUIString::ReadMetaDataErrHint);
        return false;
    }

    //cout<<ret<<endl; ///////////////////////////////////////debug
    metaData = QString::fromStdString(ret);
    return true;
}

void MainLoop::showFileTableAndInfo(QString spaceLabel, qint64 totalSpace)
{
    bool isSharing = (spaceLabel.compare(m_GuiString->sysMsgs(GUIString::SharingSpaceLabel)) == 0);
    if (spaceLabel.isEmpty())
    {
        spaceLabel = m_CurrentSpace.isEmpty() ? getSpaceLabelByIndex(0) : m_CurrentSpace;
        if (spaceLabel.isEmpty())
            return;

        setCurrentSpaceIdx(m_QSapceListModel->GetSpaceIndex(spaceLabel));
        totalSpace = m_SpaceInfos[spaceLabel].second;
        //ShowSpaceAmount(0, 0);
        clearFileListModel();
//        for(size_t i = 0; i < m_VecFileList.size(); i++)
//            addFileListModel(m_VecFileList[i].first);
    }

    // Show space info
    if (isSharing)
        setSpaceInfo(0, 0);
    else
    {
        map<qint64, pair<qint64, QString> >::iterator iter;
        for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); iter++)
        {
            if (iter->second.second.compare(spaceLabel) == 0)
                setSpaceInfo(totalSpace, m_SpaceInfos[spaceLabel].first);
        }
    }

    // show file table
    QList<FileInfo> emptyFilelist;
    for(size_t i = 0; i < m_VecFileList.size(); i++)
    {
        for(int j = 0; j < m_VecFileList[i].first.length(); j++)
        {
            if (m_VecFileList[i].first.at(j).fileSpaceLabel.compare(spaceLabel) == 0)
            {
                setFileListModel(m_VecFileList[i].first);
                return;
            }
        }
    }

    setFileListModel(emptyFilelist);
}

void MainLoop::setSpaceInfo(qint64 totalSpace, qint64 remainSpace)
{
    qint64 uint = 0;
    m_TotalSpace = Utils::AutoSizeString(totalSpace, uint);
    m_UsedSpace = Utils::AutoSizeString(totalSpace - remainSpace, uint);
    if (totalSpace == 0 && remainSpace == 0)
    {
         m_TotalSpace = m_GuiString->uiHints(GUIString::UnavailableHint);
         m_UsedSpace = m_GuiString->uiHints(GUIString::UnavailableHint);
    }
    emit totalSpaceChanged(totalSpace);
    emit usedSpaceChanged(totalSpace - remainSpace);
}

void MainLoop::waitingForNewSpaceCreating(QString TXtoMonitor)
{
    const int timeOutLimit = 20; // 20s for time out
    int timeoutCnt = 0;
    setBusyingFlag(true, m_GuiString->uiStatus(GUIString::CreatingSpaceStatus));
    while(m_WaitingSpaceFlag)
    {
        QApplication::processEvents(QEventLoop::AllEvents, 500);
        std::this_thread::sleep_for(std::chrono::milliseconds(500));

        if (!m_CreateSpaceTX.isEmpty())
            continue;

        string ret = m_BackRPC->SendOperationCmd(GUIString::GetObjInfoCmd, vector<QString>{m_AccountPwd});
        if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) != 0 &&
                ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) != 0)
        {
            QString qRet = QString::fromStdString(ret);
            QList<FileInfo> sharingList;
            if (!FileManager::ParseObjList(qRet, sharingList, m_ObjInfos))
            {
                //ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseObjErrHint]));
                return;//////////////////////////// TODO: recover 4096 data
            }
        }

        map<qint64, pair<qint64, QString> >::iterator iter = m_ObjInfos.find(m_NewObjID);
        if (iter != m_ObjInfos.end())
        {
            //m_SpaceOpTimer = false; // to wait TryNewMetaData() to return by stopping function in timer
            //QApplication::processEvents();
            tryNewMetaData(iter->second.first);
            if (!m_WaitingSpaceFlag)
                break;
        }

        timeoutCnt++;
        if (timeoutCnt >= timeOutLimit)
            break;
    }

    emit refreshFilesSig(m_CurrentSpace);
    //m_BusyFlag = false;
    //if (m_MsgBox != nullptr)
    //    m_MsgBox->close();
    setIsCreatingSpace(false);
    setBusyingFlag(false, "");
    if (timeoutCnt >= timeOutLimit)
        emit showMsgBoxSig(m_GuiString->uiHints(GUIString::CreateSpaceFailedHint), GUIString::HintDlg);
    else
    {
        QString success = m_GuiString->uiHints(GUIString::CreateSpaceSuccessHint) + ", " +
                m_GuiString->uiHints(GUIString::TXIDHint) + TXtoMonitor;
        emit showMsgBoxSig(success, GUIString::HintDlg, true);
    }
}

bool MainLoop::readMetaDataByObjID(qint64 objID, qint64& spaceTotal, qint64& spaceRemain, QString& result)
{
    QString readMetaDataRet = "";
    if (!doReadMetaDataOp(objID, readMetaDataRet))
    {
        result = readMetaDataRet;
        return false;
    }

    bool parseFailed = false;
    QString err = "";
    qint64 spaceTotalTemp = spaceTotal;
    if (!FileManager::ParseMetaData(readMetaDataRet, objID, spaceTotal, spaceRemain,
                                    m_ObjInfos, m_VecFileList, m_SpaceInfos))
    {
        if (!recoverNewSpaceMetaData(objID, readMetaDataRet) && !restoreLastMetaData(objID, readMetaDataRet))
            parseFailed = true;
//        {
//            err = m_GuiString->uiHints(GUIString::NoMetaBakToRecoverHint) + ". " +
//                    m_GuiString->uiHints(GUIString::NeedToBeReformattedHint);
//            QString spaceName = "Reformat-" + QString::number(spaceTotalTemp/MB_Bytes);
//            m_ReformatMetaJson = Utils::GenMetaDataJson(m_AccountInUse, objID,
//                                                        getNewMetaData(spaceName, spaceTotalTemp));
//        }
        else if(!FileManager::ParseMetaData(readMetaDataRet, objID, spaceTotal, spaceRemain,
                                            m_ObjInfos, m_VecFileList, m_SpaceInfos))
            parseFailed = true;
//        {
//            parseFailed = true;
//            err = m_GuiString->uiHints(GUIString::NeedToBeReformattedHint);
//        }
    }
    else if(spaceTotalTemp != 0 && spaceTotalTemp != spaceTotal)
        parseFailed = true;

    if (parseFailed)
    {
        if (getCorruptedSpaceInfo(m_AccountInUse, objID, m_ObjInfos, m_SpaceInfos, m_SpaceReformatInfos))
            err = m_GuiString->uiHints(GUIString::NeedToBeReformattedHint);
        else
            err = m_GuiString->uiHints(GUIString::CriticalErrorHint);
        result = m_GuiString->uiHints(GUIString::ParseMetaErrHint) + (err.isEmpty() ? "" : (". " + err));
        return false;
    }

    result = readMetaDataRet;
    return true;
}

bool MainLoop::writeMetaDataOp(qint64 id, QString& metaData)
{
    if (metaData.length() > fHeadLength)
        return false;
    for(int i = 0; metaData.length() < fHeadLength; i++)
        metaData += " ";
    string ret = m_BackRPC->SendOperationCmd(GUIString::BlizWriteCmd, vector<QString>{QString::number(id), "0",
                metaData, m_AccountPwd});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0 ||
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) == 0)
        return false;
    else
        return true;
}

QString MainLoop::getNewMetaData(QString label, qint64 size)
{
    return (label + CTTUi::spec + QString::number(size) + CTTUi::spec + QString::number(size - fHeadLength));
}

bool MainLoop::newMetaDataOp(qint64 id, QString label, qint64 size)
{
    QString data = getNewMetaData(label, size);
    return writeMetaDataOp(id, data);
}

void MainLoop::tryNewMetaData(qint64 totalSize)
{
    qint64 remainTemp = 0;
    qint64 totalTemp = 0;
    QString readMetaDataRet = "";

    if (!readMetaDataByObjID(m_NewObjID, totalTemp, remainTemp, readMetaDataRet))
    {
        DelayMS(delayTimeInterv*5);
        if (!newMetaDataOp(m_NewObjID, m_NewSpaceLabel, totalSize))
        {
            //m_SpaceOpTimer = true; // function NewMetaDataOp() have had a negative result and restart timer to handle it
            return;
        }
        //DelayMS(timeInterval*3);
    }

    QVariant value = QVariant("");
    UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::IncompletedMetaData, value);
    m_WaitingSpaceFlag = false;
}

bool MainLoop::recoverNewSpaceMetaData(qint64 newSpaceObjID, QString& lastMetaData)
{
    QString account = "";
    QString metaData = "";
    qint64 objID = 0;
    QString newSpaceMetaDataJson = UiConfig().Get(CTTUi::DiskMetaDataBackup, CTTUi::IncompletedMetaData).toString();
    if (!newSpaceMetaDataJson.isEmpty() && Utils::ParseMetaDataJson(newSpaceMetaDataJson, account, objID, metaData))
    {
        if (account.compare(m_AccountInUse) == 0 && objID == newSpaceObjID)
        {
            if (writeMetaDataOp(objID, metaData))
            {
                UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::IncompletedMetaData, QVariant(""));
                lastMetaData = metaData;
                return true;
            }
        }
    }

    return false;
}

void MainLoop::setCurrentFileIdx(int index)
{
    m_CurrentFileIdx = index;
}

void MainLoop::setCurrentSpaceIdx(int index)
{
    m_CurrentSpaceIdx = index;
    m_CurrentSpace = m_QSapceListModel->GetSpaceLabel(index);
    emit currentSpaceChanged(getSpaceIdx(m_CurrentSpace));
}

QString MainLoop::doDownloadFileOp(QString filePath, QString fName, qint64 objID, qint64 offset, qint64 length,
                                   QString sharerAddr, QString sharerAgentNodeID, QString key)
{
    filePath += CTTUi::spec + fName;
    setTransmittingFlag(true, 0);
    string ret = m_BackRPC->SendOperationCmd(GUIString::BlizReadFileCmd, vector<QString>{QString::number(objID),
                QString::number(offset), QString::number(length), sharerAddr, sharerAgentNodeID, key,
                filePath, m_AccountPwd});

    setTransmittingFlag(false, 2);
    emit transmitRateChanged(100);
    setBusyingFlag(false, "");

    return getDownFileOpRet(QString::fromStdString(ret));
    //emit ShowDownFileOpRetSign(QString::fromStdString(ret));
}

QString MainLoop::getDownFileOpRet(QString qRet)
{
//    if (m_MsgBox != nullptr)
//        m_MsgBox->close();

    if (qRet.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg)) == 0 ||
            qRet.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg)) == 0)
    {
        emit showMsgBoxSig(m_GuiString->uiHints(GUIString::DownloadFileFailedHint), GUIString::HintDlg);
        return m_GuiString->uiHints(GUIString::DownloadFileFailedHint);
    }

    if (qRet.compare(m_GuiString->sysMsgs(GUIString::NeedMoreFlow)) == 0)
    {
        emit showMsgBoxSig(m_GuiString->uiHints(GUIString::NeedMoreFlowHint), GUIString::HintDlg);
        return m_GuiString->uiHints(GUIString::NeedMoreFlowHint);
    }
    else
        return m_GuiString->sysMsgs(GUIString::OkMsg);
}

bool MainLoop::updateMetaDataOp(qint64 objID, QString fName, qint64 fSize, qint64& posStart, QString& updateMetaData)
{
    qint64 remainSpace = 0;
    qint64 totalSpace = 0;
    if (!readMetaDataByObjID(objID, totalSpace, remainSpace, updateMetaData))
        return false;

//    if (!doReadMetaDataOp(objID, updateMetaData))
//        return false;

//    QString label;
//    if (!FileManager::CheckHead4K(updateMetaData, label, totalSpace, remainSpace))
//    {
//        updateMetaData = m_GuiString->uiHints(GUIString::ParseMetaErrHint);
//        return false;
//    }

    if (!FileManager::CheckSpace(remainSpace, fSize))
    {
        updateMetaData = m_GuiString->uiHints(GUIString::NotEnoughSpaceToUpHint);
        return false;
    }

    QVariant value(Utils::GenMetaDataJson(m_AccountInUse, objID, updateMetaData));
    UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::LastMetaDataCache, value);
    QString newData = FileManager::UpdateMetaData(updateMetaData, fHeadLength, fName, fSize, posStart);
    if (newData.compare(m_GuiString->sysMsgs(GUIString::FNameduplicatedMsg)) == 0)
    {
        updateMetaData = newData;
        return false;
    }

    if (!writeMetaDataOp(objID, newData))
    {
        updateMetaData = m_GuiString->uiHints(GUIString::WriteMetaDataErrHint);
        return false;
    }
    return true;
}

bool MainLoop::checkFileName(int spaceIndex, QString fileName)
{
    QString space = m_QSapceListModel->GetSpaceLabel(spaceIndex);
    for(size_t i = 0; i < m_VecFileList.size(); i++)
    {
        if (m_VecFileList[i].first.size() > 0 && m_VecFileList[i].first[0].fileSpaceLabel.compare(space) == 0)
        {
            for(int j = 0; j < m_VecFileList[i].first.size(); j++)
            {
                if (m_VecFileList[i].first[j].fileName.compare(fileName) == 0)
                    return false;
            }
        }
        else
            continue;

    }

    return true;
}

void MainLoop::doUploadFileOp(qint64 objID, qint64 offset, QString fileName, qint64 fileSize, QString filePath)
{
    QString updateMetaDataOpRet = "";
    if (!updateMetaDataOp(objID, fileName, fileSize, offset, updateMetaDataOpRet))
    {
        emit showMsgBoxSig(updateMetaDataOpRet, GUIString::HintDlg);
        return;
    }

    setBusyingFlag(true, m_GuiString->uiStatus(GUIString::UploadingStatus));
    setTransmittingFlag(true, 1);
    string ret = m_BackRPC->SendOperationCmd(GUIString::BlizWriteFileCmd, vector<QString>{QString::number(objID),
                QString::number(offset), fileName, m_AccountPwd,
                QString::number(fileSize), filePath});
    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) != 0 &&
            ret.compare(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString()) != 0)
    {
        emit refreshFilesSig(m_CurrentSpace);
        UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::LastMetaDataCache, QVariant(""));
    }
    else
    {
        QString temp = "";
        restoreLastMetaData(objID, temp);
    }
    setTransmittingFlag(false, 2);
    emit transmitRateChanged(100);
    setBusyingFlag(false, "");
}

bool MainLoop::restoreLastMetaData(qint64 targetObjID, QString& lastMetaData)
{
    QString account = "";
    QString metaData = "";
    qint64 objID = 0;
    QString lastMetaDataJson = UiConfig().Get(CTTUi::DiskMetaDataBackup, CTTUi::LastMetaDataCache).toString();
    if (!lastMetaDataJson.isEmpty() && Utils::ParseMetaDataJson(lastMetaDataJson, account, objID, metaData))
    {
        if (account.compare(m_AccountInUse) == 0 && objID == targetObjID)
        {
            if (writeMetaDataOp(objID, metaData))
            {
                UiConfig().Set(CTTUi::DiskMetaDataBackup, CTTUi::LastMetaDataCache, QVariant(""));
                lastMetaData = metaData;
                return true;
            }
        }
    }

    return false;
}

void MainLoop::setTransmittingFlag(bool state, int category)
{
    m_TransmittingFlag = state;
    emit transmittingChanged(category);
}

int MainLoop::getTransmitRate() const
{
    return *m_TranmitRate;
}

void MainLoop::progressUpdatingLoop()
{
    int oldRate = 0;
    while(!m_ExitFlag)
    {
        if (*m_TranmitRate != oldRate)
        {
            emit showTranmitRate(*m_TranmitRate);
            oldRate = *m_TranmitRate;
        }

        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
}

QString MainLoop::generateSharingCode(qint64 objID, QString fileName, qint64 offset, qint64 size, QString receiver,
                                      QString startTime, QString stopTime, QString price, QString sharerAgentNodeID)
{
    std::string ret = m_BackRPC->SendOperationCmd(GUIString::GetSharingCodeCmd,
            std::vector<QString>{QString::number(objID), fileName,
                                 QString::number(offset), QString::number(size),
                                 receiver, startTime, stopTime, price,
                                 sharerAgentNodeID, m_AccountPwd});

    if (ret.compare(m_GuiString->sysMsgs(GUIString::ErrorMsg).toStdString()) == 0)
        return QString("0");

    return QString::fromStdString(ret);
}

void MainLoop::setSharingCode(QString code)
{
    m_SharingCode = code;
    emit sharingCodeChanged();
}

void MainLoop::setSharingFileInfo(QString sharer, QString fName, QString startTime, QString stopTime, QString price)
{
    m_SharingInfoSharer = sharer.replace(10, sharer.length()-20, "‧‧‧");
    m_SharingInfoFileName = fName;
    m_SharingInfoStartTime = startTime;
    m_SharingInfoStopTime = stopTime;
    m_SharingInfoPrice = price;
    emit sharingInfoSharerChanged();
    emit sharingInfoFileNameChanged();
    emit sharingInfoStartTimeChanged();
    emit sharingInfoStopTimeChanged();
    emit sharingInfoPriceChanged();
}

bool MainLoop::parseMailList(QString& mailListStr)
{
    const QString noObjId("mails not found");
    if (mailListStr.compare(noObjId) == 0)
        return true;

    QStringList list = mailListStr.split(CTTUi::spec);
    m_MailList.clear();
    for(int i = 0; i < list.length(); ++i)
    {
        MailDetails mail;
        mail.DecodeQJson(list.at(i));
        m_MailList.push_back(mail);
    }

    return true;
}

void MainLoop::setMailListModel(const QList<MailDetails>& mails)
{
    m_QMailListModel->SetModel(mails);
}

void MainLoop::setCurrentMailIdx(int index)
{
    m_CurrentMailIdx = index;
    setSharingCode(m_QMailListModel->GetMailAttachCode(index));
    if (!m_QMailListModel->GetMailAttachCode(index).isEmpty())
        getSharingInfo(m_QMailListModel->GetMailAttachCode(index));
}

void MainLoop::InitAgentsInfo()
{
    if (m_LoginWin == nullptr)
        return;
    QMetaObject::invokeMethod(m_LoginWin, "init");
}

void MainLoop::SetLoginWindow(QObject* qmlObj)
{
    m_LoginWin = qmlObj;
}

void MainLoop::HideLoginWindow()
{
    if (m_LoginWin == nullptr)
        return;
    m_LoginWin->setProperty("visible", false);
}

void MainLoop::SetAgentWindow(QObject* qmlObj)
{
    m_AgentWin = qmlObj;
}

void MainLoop::SetMainWindow(QObject* qmlObj)
{
    m_MainWin = qmlObj;
}

void MainLoop::SetDownloadDlg(QObject* qmlObj)
{
    m_DownloadDlg = qmlObj;
}

void MainLoop::SetMsgDialog(QObject* qmlObj)
{
    m_MsgDialog = qmlObj;
}

QAgentModel* MainLoop::GetAgentsModel() const
{
    return m_QAgentsModel;
}

QSpaceListModel* MainLoop::GetSpaceListModel() const
{
    return m_QSapceListModel;
}

QFileListModel* MainLoop::GetFileListModel() const
{
    return m_QFileListModel;
}

QMailListModel* MainLoop::GetMailListModel() const
{
    return m_QMailListModel;
}

QMailListModel::QMailListModel(QObject* parent)
{
    Q_UNUSED(parent);
}

QMailListModel::~QMailListModel()
{

}

int QMailListModel::rowCount(const QModelIndex&) const
{
    return m_MailList.count();
}

QVariant QMailListModel::data(const QModelIndex& index, int role) const
{
    if (!index.isValid())
        return QVariant();

    if (index.row() >= m_MailList.size())
        return QVariant();

    if (role == SenderRole)
        return m_MailList.at(index.row()).mailSender;
    else if(role == TitleRole)
        return m_MailList.at(index.row()).mailTitle;
    else if(role == TimeStampRole)
        return m_MailList.at(index.row()).mailTimeStamp;
    else if(role == AttachmentRole)
    {
        if (m_MailList.at(index.row()).mailFileSharingCode.isEmpty())
            return "";
        else
            return "attachment";
    }
    else
        return QVariant();
}

Qt::ItemFlags QMailListModel::flags(const QModelIndex& index) const
{
    if (!index.isValid())
        return Qt::ItemIsEnabled;

    return QAbstractItemModel::flags(index) | Qt::ItemIsEditable;
}

//bool QMailListModel::setData(const QModelIndex& index, const QVariant& value, int role)
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

QHash<int, QByteArray> QMailListModel::roleNames() const
{
    QHash<int, QByteArray> names;
    names[SenderRole] = "sender";
    names[TitleRole] = "title";
    names[TimeStampRole] = "timeStamp";
    names[AttachmentRole] = "attached";
    return names;
}

bool QMailListModel::insertRows(int position, int rows, const QModelIndex& parent)
{
    if (position < 0 || position > m_MailList.size() || rows <= 0)
        return false;

    beginInsertRows(parent, position, position+rows-1);
//    for (int row = 0; row < rows; ++row)
//    {
//        m_FileList.insert(position, "");
//    }
    endInsertRows();
    return true;
}

//bool QMailListModel::removeRows(int position, int rows, const QModelIndex& parent)
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

bool QMailListModel::ClearData(const QModelIndex& parent)
{
    beginRemoveRows(parent, 0, m_MailList.size()-1);
    m_MailList.clear();
    endRemoveRows();
    return true;
}

void QMailListModel::AddModel(const QList<MailDetails>& mails)
{
    for (int i = 0; i < mails.size(); i++)
        m_MailList.append(mails.at(i));
    insertRows(0, mails.size());
}

void QMailListModel::SetModel(const QList<MailDetails>& mails)
{
    ClearData();
    m_MailList = mails;
    insertRows(0, mails.size());
//    for(int i = 0; i < files.size(); i++)
//        setData(createIndex(i, 0), it->second.second, SpaceLabelRole);

    //emit dataChanged(createIndex(0, 0), createIndex(m_SpaceList.count()-1, 0));
}

QString QMailListModel::GetMailAttachCode(int index) const
{
    if (index >= m_MailList.size())
        return "";

    return m_MailList.at(index).mailFileSharingCode;
}

QString QMailListModel::GetMailTitle(int index) const
{
    if (index >= m_MailList.size())
        return "";

    return m_MailList.at(index).mailTitle;
}

QString QMailListModel::GetMailSender(int index) const
{
    if (index >= m_MailList.size())
        return "";

    return m_MailList.at(index).mailSender;
}

QString QMailListModel::GetMailTimeStamp(int index) const
{
    if (index >= m_MailList.size())
        return "";

    return m_MailList.at(index).mailTimeStamp;
}

QString QMailListModel::GetMailContent(int index) const
{
    if (index >= m_MailList.size())
        return "";

    return m_MailList.at(index).mailContent;
}
