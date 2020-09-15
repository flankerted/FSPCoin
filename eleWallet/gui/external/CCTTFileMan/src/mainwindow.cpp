//#if (defined WIN32) || (defined WIN64)
//    #include <windows.h>
//#else
//    #include <unistd.h>
//#endif
#include <iostream>
#include <thread>
#include <QCloseEvent>
#include <QClipboard>
#include "mainwindow.h"
#include "dataDirDlg.h"
#include "signAgentDlg.h"
#include "sharingFileDlg.h"
#include "newMailDlg.h"
#include "ui_mainwindow.h"
#include "config.h"

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

string DoubleToString(double value)
{
    const std::string& new_val = std::to_string(value);
    return new_val;
}

QStringList ParseAgents(QString agentsQStr)
{
    QStringList list = agentsQStr.split(Ui::spec);
    return list;
}

QStringList ParseAccounts(QString accountsQStr)
{
    QStringList list = accountsQStr.split(Ui::spec);
    return list;
}

//QString AutoSizeString(qint64 size)
//{
//    QString sizedStr;
//    if (size / MB_Bytes > 0)
//        sizedStr = QString::number(size / MB_Bytes) + QString(" MB");
//    else if(size / KB_Bytes > 0)
//        sizedStr = QString::number(size / KB_Bytes) + QString(" KB");
//    else
//        sizedStr = QString::number(size) + QString(" B");
//    return sizedStr;
//}

MainWindow::MainWindow(QWidget *parent, quint16 port) :
    QMainWindow(parent),
    ui(new Ui::MainWindow),
    pwdEdit(new QLineEdit),
    newSpaceLabelEdit(new QLineEdit),
    newSpaceSizeEdit(new QLineEdit),
    m_MsgBox(nullptr),
    m_TranferRate(new(int)),
    m_BackRPC("127.0.0.1", port, m_TranferRate),
    m_AccountInUse(QString("")),
    m_AgentIPInUse(QString("")),
    m_AgentConnectedAccount(QString("")),
    m_AgentDetailsRawData(QString("")),
    m_Cmds(Ui::RPCCommands),
    m_SysMsgs(Ui::SysMessages),
    m_UIHints(Ui::UIHintsEN),
    m_CreateSpaceTX(""),
    m_QAgentsModel(nullptr),
    m_QAccountsModel(nullptr),
    m_BusyFlag(false),
    m_LangENFlag(true),
    m_IsLoginFlag(false),
    m_HasAgentFlag(false),
    m_HasConnFlag(false),
    m_AtServiceFlag(false),
    m_WaitingSpaceFlag(false),
    m_CloseFlag(false),
    m_nTimerId(0),
    m_RefreshTimerId(0)
{
    ui->setupUi(this);
    ui->tabWidget->setCurrentIndex(0);
    SetLang();

    HasConnection = false;
    m_BackRPC.ConnectBackend(&HasConnection);
    clock_t now = clock();
    while(clock() - now < 1000)
    {
        if (HasConnection)
            break;
        QApplication::processEvents(QEventLoop::AllEvents, 100);
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    if (!HasConnection)
        return;

    *m_TranferRate = 0;
    InitUIElements();

    if (!SetDataDir())
        return;
    //DoGetAgentInfoOp(true);
    m_AgentDetails.clear();
    InitAgents();
    string errMessage = "";
    InitAccounts(errMessage);
    EnableElements();

    DoGetAgentInfoOp(true);

    connect(this, SIGNAL(ShowTransferRate(int)), SLOT(UpdateTransferRate(int)));
    connect(this, SIGNAL(ShowUpFileOpRetSign(QString)), SLOT(ShowUpFileOpRet(QString)), Qt::QueuedConnection);
    connect(this, SIGNAL(ShowDownFileOpRetSign(QString)), SLOT(ShowDownFileOpRet(QString)), Qt::QueuedConnection);
    connect(this, SIGNAL(ShowOpErrorMsgBoxSign(QString, bool, bool)), SLOT(ShowOpErrorMsgBox(QString, bool, bool)), Qt::QueuedConnection);
    connect(this, SIGNAL(TXCompleteSign(QString)), SLOT(HandleTXComplete(QString)), Qt::QueuedConnection);
}

MainWindow::~MainWindow()
{
    if (m_nTimerId != 0)
        killTimer(m_nTimerId);
    if (m_RefreshTimerId != 0)
        killTimer(m_RefreshTimerId);
    delete m_QAgentsModel;
    if (m_QAccountsModel != nullptr)
        delete m_QAccountsModel;
//    if (m_FileTableModel != nullptr)
//        delete m_FileTableModel; // or will crash
    delete ui;
}

void MainWindow::closeEvent(QCloseEvent* e)
{
    QMessageBox::StandardButton button;
    button = QMessageBox::question(this, QString::fromStdString(m_UIHints[Ui::exitWindowTitle]),
            QString::fromStdString(m_UIHints[Ui::exitWindowHint]),
            QMessageBox::Yes|QMessageBox::No);
    if(button == QMessageBox::No)
        e->ignore();
    else if(button == QMessageBox::Yes)
    {
        m_BackRPC.SendOperationCmd(m_Cmds[Ui::exitExternGUICmd], vector<QString>());
        clock_t now = clock();
        while(clock() - now < 10)
            QApplication::processEvents(QEventLoop::AllEvents, 100);

        m_CloseFlag = true;
        e->accept();
    }
}

void MainWindow::timerEvent(QTimerEvent*)
{
    DoGetSignedAgentsOp(false);
    GetAgentIPAddresses(m_AgentDetails);
    RefreshAgentDetails();

    string balVal = DoGetAccBalanceOp();
    ShowAccBalance(QString::fromStdString(balVal), QString("0"));
}

void MainWindow::UpdateTransferRate(int rate)
{
    if (rate >= 0 && rate < 100)
        ui->uploadProgressBar->setValue(rate);
    ui->uploadProgressBar->repaint();
}

void MainWindow::TryNewMetaData(qint64 totalSize)
{
    qint64 remainTemp = 0;
    qint64 totalTemp = 0;
    if (!ReadMetaData(m_NewObjID, totalTemp, remainTemp, true))
    {
        DelayMS(timeInterval);
        if (!NewMetaDataOp(m_NewObjID, m_NewSpaceLabel, totalSize))
        {
            //m_SpaceOpTimer = true; // function NewMetaDataOp() have had a negative result and restart timer to handle it
            return;
        }
        //DelayMS(timeInterval*3);
    }
    //m_SpaceOpTimer = true; // function NewMetaDataOp() have had a positive result and restart timer to handle it
    m_WaitingSpaceFlag = false;
}

void MainWindow::SetLang()
{
    if (m_LangENFlag)
    {
        m_StringsTable = Ui::stringsTableEN;
        m_UIHints = Ui::UIHintsEN;
        m_SysMsgs = Ui::SysMessages;
    }
    else
    {
        m_StringsTable = Ui::stringsTableCN;
        m_UIHints = Ui::UIHintsCN;
        m_SysMsgs = Ui::SysMessages;
    }
}

void MainWindow::ProgressUpdatingLoop()
{
    int oldRate = 0;
    while(!m_CloseFlag)
    {
        if (*m_TranferRate != oldRate)
        {
            emit ShowTransferRate(*m_TranferRate);
            oldRate = *m_TranferRate;
        }

        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
}

void MainWindow::InitUIElements()
{
    this->setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    this->resize(minimumWinW, minimumWinH);
    this->setFixedSize(minimumWinW, minimumWinH);
    QRegExp regx("[a-zA-Z0-9]+$");
    QValidator *validator = new QRegExpValidator(regx, newSpaceLabelEdit);
    newSpaceLabelEdit->setValidator(validator);
    QRegExp regxSendTx("\\d*\\.\\d+");
    QValidator *validatorTx = new QRegExpValidator(regxSendTx, ui->sendTxAmtLineEdit);
    ui->sendTxAmtLineEdit->setValidator(validatorTx);
    ui->uploadProgressBar->setStyleSheet("QProgressBar { border: 1px solid grey; border-color:#ABABAB; border-radius: 2px } QProgressBar::chunk { background-color: rgb(2, 134, 238) }");

    InitFileTable();
    InitMailTable();

    std::thread t(bind(&MainWindow::ProgressUpdatingLoop, this));
    t.detach();
}

void MainWindow::InitFileTable()
{
    QStandardItemModel* model = new QStandardItemModel();
    QStringList labels = QObject::trUtf8(fileTableLabels.data()).simplified().split(Ui::spec);
    model->setHorizontalHeaderLabels(labels);
    ui->fileTable->setModel(model);
    ui->fileTable->setColumnWidth(0, fileTableInitW0);
    ui->fileTable->setColumnWidth(1, fileTableInitW1);
    ui->fileTable->setColumnWidth(2, fileTableInitW2);
    ui->fileTable->horizontalHeader()->setSectionResizeMode(1, QHeaderView::Stretch);
}

void MainWindow::InitMailTable()
{
    QStandardItemModel* model = new QStandardItemModel();
    QStringList labels = QObject::trUtf8(mailTableLabels.data()).simplified().split(Ui::spec);
    model->setHorizontalHeaderLabels(labels);
    ui->mailTable->setModel(model);
    ui->mailTable->setColumnWidth(0, mailTableInitW0);
    ui->mailTable->setColumnWidth(1, mailTableInitW1);
    ui->mailTable->setColumnWidth(2, mailTableInitW2);
    ui->mailTable->setColumnWidth(3, mailTableInitW3);
    ui->mailTable->horizontalHeader()->setSectionResizeMode(1, QHeaderView::Stretch);
}

bool MainWindow::CheckDataDir()
{
    QString qDataDir = Config().Get(Ui::WalletSettings, Ui::DataDir).toString();
    if (qDataDir.isEmpty())
        return false;
    else
    {
        m_DataDir = qDataDir.toStdString();
        m_BackRPC.SendOperationCmd(m_Cmds[Ui::setDataDirCmd], vector<QString>{QString::fromStdString(m_DataDir)});
        return true;
    }
}

bool MainWindow::SetDataDir()
{
    // Check if data directory has been set
    if (CheckDataDir())
        return true;

    // Set default data directory
    m_DataDir = QCoreApplication::applicationDirPath().toStdString();

    // Set data directory dialog
    DataDirDlg dataDirDlg(&m_DataDir, m_LangENFlag, this);
    dataDirDlg.setModal(true);
    dataDirDlg.setFixedSize(dataDirDlgSize);
    dataDirDlg.setWindowTitle(QString::fromStdString(m_StringsTable[Ui::dataDirDlgTitle]));
    dataDirDlg.move((QApplication::desktop()->width() - dataDirDlg.width())/2,
                    (QApplication::desktop()->height() - dataDirDlg.height())/2);

    if (dataDirDlg.exec() == QDialog::Accepted)
    {
        QVariant value(QString::fromStdString(m_DataDir));
        Config().Set(Ui::WalletSettings, Ui::DataDir, value);
        m_BackRPC.SendOperationCmd(m_Cmds[Ui::setDataDirCmd], vector<QString>{QString::fromStdString(m_DataDir)});
    }
    else
    {
        dataDirDlg.close();
        return false;
    }

    return true;
}

void MainWindow::InitAgents()
{
    // Get agent list
    if (GetAgentList() == 0)
        return;

    // Get default
    GetDefaultAgent();
    m_HasAgentFlag = true;
}

int MainWindow::GetAgentList()
{
    QString qAgents = Config().Get(Ui::AgentSettings, Ui::AgentList).toString();
    if (qAgents.isEmpty())
    {
        Config().Set(Ui::AgentSettings, Ui::AgentList, QString(""));
        Config().Set(Ui::AgentSettings, Ui::DefaultAgent, QString(""));
        return 0;
    }
    else
    {
        m_QAgents = ParseAgents(qAgents);

        for(std::map<QString, QString>::const_iterator it = m_AgentInfo.begin(); it != m_AgentInfo.end(); it++)
            m_QAgents.append(it->first);

        m_QAgentsModel = new QStringListModel(m_QAgents);
        ui->agentList->setModel(m_QAgentsModel);
        return m_QAgents.length();
    }
}

void MainWindow::GetDefaultAgent()
{
    m_AgentIPInUse = Config().Get(Ui::AgentSettings, Ui::DefaultAgent).toString();
    if (m_AgentIPInUse.isEmpty())
    {
        m_AgentIPInUse = static_cast<QString>(m_QAgents.at(0));
        Config().Set(Ui::AgentSettings, Ui::DefaultAgent, m_AgentIPInUse);
    }
    ui->agentToConnEdit->setText(m_AgentIPInUse);
}

void MainWindow::AddAgent(QString agent)
{
    for (int i = 0; i < m_QAgents.length(); i++)
    {
        if (m_QAgents.at(i).compare(agent) == 0)
            return;
    }

    m_QAgents.append(agent);
    if (m_QAgentsModel == nullptr)
        m_QAgentsModel = new QStringListModel(m_QAgents);
    else
        m_QAgentsModel->setStringList(m_QAgents);
    ui->agentList->setModel(m_QAgentsModel);

    QString agents("");
    for (int i = 0; i < m_QAgents.length(); i++)
    {
        agents += static_cast<QString>(m_QAgents.at(i));
        if (i != m_QAgents.length()-1)
            agents += Ui::spec;
    }
    Config().Set(Ui::AgentSettings, Ui::AgentList, agents);

    if (m_QAgents.length() == 1)
        SetDefaultAgent(agent);
}

void MainWindow::SetDefaultAgent(QString agent)
{
    m_AgentIPInUse = agent;
    ui->agentToConnEdit->setText(m_AgentIPInUse);

    Config().Set(Ui::AgentSettings, Ui::DefaultAgent, agent);
}

void MainWindow::DeleteAgent(QString agent)
{
    QString agents("");
    for (int i = 0; i < m_QAgents.length(); i++)
    {
        if (static_cast<QString>(m_QAgents.at(i)).compare(agent) == 0)
            m_QAgents.removeAt(i);
        else
        {
            agents += static_cast<QString>(m_QAgents.at(i));
            if (i != m_QAgents.length()-1)
                agents += Ui::spec;
        }
    }
    if (m_QAgentsModel == nullptr)
        m_QAgentsModel = new QStringListModel(m_QAgents);
    else
        m_QAgentsModel->setStringList(m_QAgents);
    ui->agentList->setModel(m_QAgentsModel);

    if (agents.length() > 0)
    {
        if (agents.at(agents.length()-1) == QChar(Ui::spec.at(0)))
            agents.remove(agents.length()-1, 1);
        Config().Set(Ui::AgentSettings, Ui::AgentList, agents);

        if (m_AgentIPInUse.compare(agent) == 0)
            SetDefaultAgent(m_QAgents.at(0));
    }
}

void MainWindow::DoGetAgentInfoOp(bool showMsg)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getAgentsInfoCmd], vector<QString>{});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
        if (showMsg)
        {
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::getAgentInfoErrHint]),
                    QMessageBox::Warning, QMessageBox::Ok);
        }
        return;
    }

    if (!AgentDetailsFunc::ParseAgentInfo(QString::fromStdString(ret), m_AgentDetails, m_AgentInfo))
    {
        if (showMsg)
        {
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::getAgentInfoErrHint]),
                    QMessageBox::Warning, QMessageBox::Ok);
        }
        return;
    }
}

void MainWindow::DoGetSignedAgentsOp(bool showMsg)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getSignedAgentInfoCmd], vector<QString>{m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
        if (showMsg)
        {
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::getAgentDetailsErrHint]),
                    QMessageBox::Warning, QMessageBox::Ok);
        }
        return;
    }
    m_AgentDetailsRawData = QString::fromStdString(ret);

    if (!AgentDetailsFunc::ParseAgentDetails(QString::fromStdString(ret), m_AgentDetails))
    {
        if (showMsg)
        {
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::getAgentDetailsErrHint]),
                    QMessageBox::Warning, QMessageBox::Ok);
        }
        return;
    }
}

QString MainWindow::GetAgentIPByAcc(const QString& account)
{
    for(map<QString, QString>::const_iterator it = m_AgentInfo.begin(); it != m_AgentInfo.end(); it++)
    {
        if (it->second.compare(account) == 0)
            return it->first;
    }
    return "";

    //return QString("127.0.0.1");
}

void MainWindow::GetAgentIPAddresses(map<QString, AgentDetails>& agentDetails)
{
    for(map<QString, AgentDetails>::iterator it = agentDetails.begin(); it != agentDetails.end(); it++)
    {
        if (it->second.IPAddress.isEmpty())
            it->second.IPAddress = GetAgentIPByAcc(it->first);
    }
}

void MainWindow::GetSignedAgents()
{
    DoGetSignedAgentsOp();
    GetAgentIPAddresses(m_AgentDetails);
    ui->agentsComboBox->clear();
    for(map<QString, AgentDetails>::iterator it = m_AgentDetails.begin(); it != m_AgentDetails.end(); it++)
        ui->agentsComboBox->addItem(it->first);
}

QString MainWindow::DoGetAgentAccByIPOp(QString IPAddress)
{
    if (m_AgentInfo.find(IPAddress) != m_AgentInfo.end())
        return m_AgentInfo[IPAddress];
    return "";
}

AgentDetails MainWindow::GetAgentDetailsByAcc(const QString& account)
{
    AgentDetails details;
    details.CsAddr = account;
    if (m_AgentDetails.find(account.toLower()) != m_AgentDetails.end())
        details = m_AgentDetails[account.toLower()];
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

AgentDetails MainWindow::GetAgentDetailsByIP(QString IP)
{
    QString agentAcc;
    if (IP.compare(ui->agentToConnEdit->text()) == 0)
    {
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getCurAgentAddrCmd], vector<QString>());
        if (ret != m_SysMsgs[Ui::errorMsg] && !QString::fromStdString(ret).isEmpty())
            agentAcc = QString::fromStdString(ret);
        else
            ShowOpErrorMsgBox();
    }
    else
        agentAcc = DoGetAgentAccByIPOp(IP);
    if (agentAcc.isEmpty())
        return AgentDetails();
    AgentDetails details = GetAgentDetailsByAcc(agentAcc);
    details.IPAddress = IP;
    return details;
}

QString MainWindow::GetMyAgentNoneID()
{
    QString currentAgent = m_AgentConnectedAccount.toLower(); //ui->agentsComboBox->currentText().toLower();
    std::map<QString, AgentDetails>::iterator iter = m_AgentDetails.find(currentAgent);
    if (iter == m_AgentDetails.end())
    {
        return "";
    }
    return iter->second.AgentNodeID;
}

void MainWindow::ShowAgentDetails()
{
    if (ui->agentsComboBox->currentText().isEmpty())
    {
        ClearAgentDetails();
        return;
    }

    RefreshAgentDetails();

    EnableElements();
}

void MainWindow::UpdateAgentDetails(QString TXtoMonitor, string cmd)
{
    if (ui->agentsComboBox->currentText().isEmpty())
        return;
    QString currentAgent = ui->agentsComboBox->currentText();
    QString waitingHint = QString::fromStdString(m_UIHints[Ui::waitingHint]);
    ui->agAtServVal->setText(waitingHint);
    ui->agIPAddrVal->setText(waitingHint);
    ui->agTStartVal->setText(waitingHint);
    ui->agTStopVal->setText(waitingHint);
    ui->agFlowVal->setText(waitingHint);
    ui->agFlowUsedVal->setText(waitingHint);
    ui->agMethodVal->setText(waitingHint);

    m_BusyFlag = true;

    EnableElements();

    QApplication::processEvents();
    DoWaitTXCompleteOp(TXtoMonitor, cmd);

    EnableElements();
}

void MainWindow::ClearAgentDetails()
{
    ui->agAtServVal->setText("");
    ui->agIPAddrVal->setText("");
    ui->agTStartVal->setText("");
    ui->agTStopVal->setText("");
    ui->agFlowVal->setText("");
    ui->agFlowUsedVal->setText("");
    ui->agMethodVal->setText("");
}

void MainWindow::RefreshAgentDetails()
{
    if (ui->agentsComboBox->currentText().isEmpty())
        return;
    QString currentAgent = ui->agentsComboBox->currentText().toLower();
    if (m_AgentDetails.find(currentAgent) == m_AgentDetails.end())
    {
        ClearAgentDetails();
        return;
    }

    ui->agAtServVal->setText(m_AgentDetails[currentAgent].IsAtService);
    ui->agIPAddrVal->setText(m_AgentDetails[currentAgent].IPAddress);
    ui->agTStartVal->setText(m_AgentDetails[currentAgent].TStart);
    ui->agTStopVal->setText(m_AgentDetails[currentAgent].TEnd);
    ui->agFlowVal->setText(m_AgentDetails[currentAgent].FlowAmount);
    ui->agFlowUsedVal->setText(m_AgentDetails[currentAgent].FlowUsed);

    QString paymentMethod = m_AgentDetails[currentAgent].PaymentMeth;
    if (m_AgentDetails[currentAgent].PaymentMeth.compare("0") == 0)
        paymentMethod = QString::fromStdString(m_UIHints[Ui::chargeByTimeHint]);
    else if(m_AgentDetails[currentAgent].PaymentMeth.compare("N/A") != 0)
        paymentMethod = QString::fromStdString(m_UIHints[Ui::chargeByFlowHint]);
    ui->agMethodVal->setText(paymentMethod);
}

void MainWindow::DoWaitTXCompleteOp(QString TXtoMonitor, string cmdType)
{
    if (cmdType.compare(m_Cmds[Ui::signAgentCmd]) == 0 || cmdType.compare(m_Cmds[Ui::cancelAgentCmd]) == 0)
    {
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::waitTXCompleteCmd], vector<QString>{TXtoMonitor});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
            emit ShowOpErrorMsgBoxSign(QString::fromStdString(m_UIHints[Ui::waitingTimeoutHint]));
        else
            m_AtServiceFlag = true;
        m_BusyFlag = false;
        emit TXCompleteSign(QString::fromStdString(cmdType));
    }
    else if(cmdType.compare(m_Cmds[Ui::createObjCmd]) == 0)
    {
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::waitTXCompleteCmd], vector<QString>{TXtoMonitor});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        {
            m_CreateSpaceTX.clear();
            emit ShowOpErrorMsgBoxSign(QString::fromStdString(m_UIHints[Ui::waitingTimeoutHint]));
        }
        else
            m_CreateSpaceTX.clear();
        m_BusyFlag = false;
    }
}

void MainWindow::HandleTXComplete(QString cmdType)
{
    GetSignedAgents();
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getCurAgentAddrCmd], vector<QString>());
    if (ret != m_SysMsgs[Ui::errorMsg] && !QString::fromStdString(ret).isEmpty())
    {
        if (m_AgentDetails.find(QString::fromStdString(ret).toLower()) == m_AgentDetails.end())
        {
            AgentDetails agentDetails = GetAgentDetailsByIP(ui->agentToConnEdit->text());
            QString agentAcc = agentDetails.CsAddr;
            m_AgentDetails[agentAcc.toLower()] = agentDetails;
            ui->agentsComboBox->addItem(agentAcc);
            ui->agentsComboBox->setCurrentText(agentAcc);
        }
        ShowAgentDetails();
    }
}

string MainWindow::DoGetAccBalanceOp(QString account)
{
    vector<QString> accParam;
    string cmd;
    string errHint;
    if (account.isEmpty())
    {
        accParam.clear();
        cmd = m_Cmds[Ui::getAccTBalanceCmd];
        errHint = m_UIHints[Ui::getAccTBalanceErrHint];
    }
    else
    {
        accParam.push_back(account);
        cmd = m_Cmds[Ui::getAccBalanceCmd];
        errHint = m_UIHints[Ui::getAccBalanceErrHint];
    }
    string ret = m_BackRPC.SendOperationCmd(cmd, accParam);
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
//        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
//                QString::fromStdString(errHint),
//                QMessageBox::Warning, QMessageBox::Ok);
        return "";
    }

    return ret;
}

void MainWindow::ShowAccBalance(QString val, QString pend, bool isTotal)
{
    if (isTotal)
    {
        ui->balSumVal->setText(val);
        ui->balSumPendVal->setText(pend);
    }
    else
    {
        ui->balSelVal->setText(val);
        ui->balSelPendVal->setText(pend);
    }
}

bool MainWindow::InitAccounts(string& err)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::showAccountsCmd], vector<QString>());
    if (ret.compare(m_SysMsgs[Ui::noAnyAccountMsg]) == 0)
    {
        Config().Set(Ui::AccountSettings, Ui::DefaultAccount, QString(""));
        MessageBoxShow(QString::fromStdString(m_SysMsgs[Ui::noAnyAccountMsg]),
                QString::fromStdString(m_UIHints[Ui::noAnyAccountMsg]),
                QMessageBox::Information, QMessageBox::Ok);
        ui->tabWidget->setCurrentIndex(3);
    }
    else if(ret.compare(m_SysMsgs[Ui::errorMsg]) == 0)
    {
        ShowOpErrorMsgBox();
        return false;
    }
    else
    {
        // Set account list
        SetAccountList(QString::fromStdString(ret));

        // Get default
        GetDefaultAccount();
    }

    return true;
}

void MainWindow::SetAccountList(QString accountsStr)
{
    m_QAccounts = ParseAccounts(accountsStr);
    if (m_QAccountsModel == nullptr)
        m_QAccountsModel = new QStringListModel(m_QAccounts);
    else
        m_QAccountsModel->setStringList(m_QAccounts);
    ui->accountList->setModel(m_QAccountsModel);
    for (int i = 0; i < m_QAccounts.length(); i++)
        ui->accountComboBox->addItem(m_QAccounts.at(i));
}

void MainWindow::GetDefaultAccount()
{
    QString defaultAccount = Config().Get(Ui::AccountSettings, Ui::DefaultAccount).toString();
    if (defaultAccount.isEmpty())
    {
        defaultAccount = static_cast<QString>(m_QAccounts.at(0));
        Config().Set(Ui::AccountSettings, Ui::DefaultAccount, defaultAccount);
    }
    ui->defAccLineEdit->setText(defaultAccount);
    ui->accountComboBox->setCurrentText(defaultAccount);
    m_AccountInUse = defaultAccount;
}

void MainWindow::AddAccount(QString account)
{
    m_QAccounts.append(account);
    if (m_QAccountsModel == nullptr)
        m_QAccountsModel = new QStringListModel(m_QAccounts);
    else
        m_QAccountsModel->setStringList(m_QAccounts);
    ui->accountList->setModel(m_QAccountsModel);
    ui->accountComboBox->addItem(account);
}

void MainWindow::SetDefaultAccount(QString account)
{
    m_AccountInUse = account;
    ui->defAccLineEdit->setText(m_AccountInUse);
    ui->accountComboBox->setCurrentText(m_AccountInUse);

    Config().Set(Ui::AccountSettings, Ui::DefaultAccount, account);
}

bool MainWindow::GetFileList()
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getObjInfoCmd], vector<QString>{m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::getObjErrHint]));
    else
    {
        //cout<<ret<<endl; ///////////////////////////////////////debug
        QString qRet = QString::fromStdString(ret);
        QList<FileInfo> sharingList;
        if (!ParseObjList(qRet, sharingList))
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseObjErrHint]));

        if (sharingList.size() > 0)
            m_VecFileList.push_back(pair<QList<FileInfo>, qint64>(sharingList, 0));
    }

    qint64 spareSpace = 0;
    qint64 remainTemp = 0;
    qint64 totalSpace = 0;
    map<qint64, pair<qint64, QString> >::iterator iter;
    for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); iter++)
    {
        DelayMS(100);
        qint64 totalSpaceTemp = 0;
        if (!ReadMetaData(iter->first, totalSpaceTemp, remainTemp))
            return false;
        spareSpace += remainTemp;
        if (ui->curSpaceComboBox->currentText().compare(iter->second.second) == 0)
            totalSpace = totalSpaceTemp;
    }
    if (m_VecFileList.size() > 0)
        ShowFileTable(totalSpace);

    return true;
}

void MainWindow::RefreshFiles()
{
    QString currentSpaceLabel = ui->curSpaceComboBox->currentText();
    m_GetListFlag = true;
    m_ObjInfos.clear();
    m_VecFileList.clear();
    ui->curSpaceComboBox->clear();

    GetFileList();

    QString sharingsLabel = QString::fromStdString(m_StringsTable[Ui::sharingSpaceLabel]);
    for(size_t i = 0; i < m_VecFileList.size(); i++)
    {
        for(int j = 0; j < m_VecFileList[i].first.length(); j++)
        {
            if (m_VecFileList[i].first.at(j).fileSpaceLabel.compare(sharingsLabel) == 0)
            {
                ui->curSpaceComboBox->addItem(sharingsLabel);
                goto getFileList;
            }
        }
    }

    getFileList:
    if (currentSpaceLabel.isNull())
    {
        map<qint64, pair<qint64, QString> >::iterator iter;
        for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); )
        {
            ui->curSpaceComboBox->setCurrentText(iter->second.second);
            return;
        }
    }

    ui->curSpaceComboBox->setCurrentText(currentSpaceLabel);
}

void MainWindow::RefreshMails()
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getMailsCmd], vector<QString>{m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::getMailsFailedHint]));
        return;
    }

    //QString abc("{\"Title\":\"title_1\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"1...1\",\"TimeStamp\":\"2019-01-01 00:05:00\",\"FileSharing\":\"\",\"Signature\":\"\"}/{\"Title\":\"title_2\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"2...2\",\"TimeStamp\":\"2019-01-02 00:00:00\",\"FileSharing\":\"\",\"Signature\":\"...\"}/{\"Title\":\"title_3\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"3...3\",\"TimeStamp\":\"2019-01-03 00:05:00\",\"FileSharing\":\"{\\\"FileName\\\":\\\"Qt5Networkd.dll\\\",\\\"FileOffset\\\":\\\"16352632\\\",\\\"FileSize\\\":\\\"94562078\\\",\\\"ObjectID\\\":1,\\\"Price\\\":\\\"0.12\\\",\\\"Receiver\\\":\\\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\\\",\\\"SharerCSNodeID\\\":\\\"\\\",\\\"Signature\\\":\\\"0x51eb9f01c56e9be964a2885187e97135336667c1ae511f3961550b2ee4a486f91b990700ffbfb524094b32562fdae6dde6e302f35fd8f0aaf63697ec49e439dc00\\\",\\\"TimeStart\\\":\\\"2019-01-01 00:00:00\\\",\\\"TimeStop\\\":\\\"2019-01-01 00:00:00\\\"}\",\"Signature\":\"...\"}");
    QString mailDetals = QString::fromStdString(ret);
    if (mailDetals.compare("null") == 0)
        return;
    ParseMailList(mailDetals);
    ShowMailTable();
}

bool MainWindow::ReadMetaData(qint64 objID, qint64& spaceTotal, qint64& spaceRemain, bool showNoMsg)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::blizReadCmd],
            vector<QString>{QString::number(objID), "0", QString::fromStdString(fHeadLengthStr), m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseMetaErrHint]));
        return false;
    }

    //cout<<ret<<endl; ///////////////////////////////////////debug
    QString qRet = QString::fromStdString(ret);
    QString label;
    if (!FileManager::CheckHead4K(qRet, label, spaceTotal, spaceRemain))
    {
        // TODO: flush them all and re-format
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseMetaErrHint]));
        return false;
    }
    else
    {
        m_ObjInfos[objID].second = label;
        if (ui->curSpaceComboBox->findText(label) < 0)
            ui->curSpaceComboBox->addItem(label);
    }
    QList<FileInfo> myFileList;
    FileManager::GetFileList(qRet, myFileList);
    m_VecFileList.push_back(pair<QList<FileInfo>, qint64>(myFileList, spaceRemain));

    return true;
}

void MainWindow::ShowFileTable(qint64 totalSpace, QString curLabel)
{
    QString fLabel = curLabel.isEmpty() ? ui->curSpaceComboBox->currentText() : curLabel;
    m_FileTableModel = new QStandardItemModel(ui->fileTable);
    QStringList labels = QObject::trUtf8(fileTableLabels.data()).simplified().split(Ui::spec);
    m_FileTableModel->setHorizontalHeaderLabels(labels);

    qint64 spareSpace = 0;
    bool isSharings = true;
    map<qint64, pair<qint64, QString> >::iterator iter;
    for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); iter++)
    {
        if (iter->second.second.compare(fLabel) == 0)
        {
            spareSpace = iter->second.first;
            ShowSpaceAmount(totalSpace, spareSpace);
            isSharings = false;
        }
    }

    for(size_t i = 0; i < m_VecFileList.size(); i++)
    {
        for(int j = 0; j < m_VecFileList[i].first.length(); j++)
        {
            if (m_VecFileList[i].first.at(j).fileSpaceLabel.compare(fLabel) == 0)
            {
                spareSpace = m_VecFileList[i].second;
                QString name = m_VecFileList[i].first.at(j).fileName;
                QString size = Utils::AutoSizeString(m_VecFileList[i].first.at(j).fileSize.toLongLong());
                QString fileTime = QString("-");
                if (fLabel.toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) == 0)
                    fileTime = m_VecFileList[i].first.at(j).shareStart + "~" + m_VecFileList[i].first.at(j).shareStop;
                QList<QStandardItem*> list;
                list << new QStandardItem(name) << new QStandardItem(fileTime) << new QStandardItem(size);
                m_FileTableModel->insertRow(0, list);
                m_FileTableModel->item(0, 1)->setTextAlignment(Qt::AlignHCenter | Qt::AlignVCenter);
                m_FileTableModel->item(0, 2)->setTextAlignment(Qt::AlignHCenter | Qt::AlignVCenter);

                ShowSpaceAmount(totalSpace, spareSpace);
            }
        }
    }
    ui->fileTable->setModel(m_FileTableModel);

    if (isSharings)
        ShowSpaceAmount(0, 0);
    ui->uploadBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                              ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
    ui->sharingBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                               ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
    ui->expSpaceBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                                ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
}

void MainWindow::ShowMailTable()
{
    QStandardItemModel* model = new QStandardItemModel(ui->mailTable);
    QStringList labels = QObject::trUtf8(mailTableLabels.data()).simplified().split(Ui::spec);
    model->setHorizontalHeaderLabels(labels);

    for(size_t i = 0; i < m_MailList.size(); i++)
    {
        QString sender = m_MailList[i].mailSender;
        QString title = m_MailList[i].mailTitle;
        QString attachment = m_MailList[i].mailFileSharingCode.isEmpty() ? "no" : "yes";
        QString time = m_MailList[i].mailTimeStamp;
        QList<QStandardItem*> list;
        list << new QStandardItem(sender) << new QStandardItem(title) << new QStandardItem(attachment) << new QStandardItem(time);
        model->insertRow(0, list);
        model->item(0, 2)->setTextAlignment(Qt::AlignHCenter | Qt::AlignVCenter);
        model->item(0, 3)->setTextAlignment(Qt::AlignHCenter | Qt::AlignVCenter);
    }

    ui->mailTable->setModel(model);
}

void MainWindow::ShowSpaceAmount(qint64 totalSpace, qint64 spareBytes)
{
    if (totalSpace == 0 && spareBytes == 0)
    {
        ui->tSpaceValLabel->setText(QString("-"));
        ui->remSpaceValLabel->setText(QString("-"));
    }
    else
    {
        ui->tSpaceValLabel->setText(Utils::AutoSizeString(totalSpace));
        ui->remSpaceValLabel->setText(Utils::AutoSizeString(spareBytes));
    }
}

bool MainWindow::ParseObjList(QString& objListStr, QList<FileInfo>& sharings)
{
    const QString noObjId("object id not find");
    if (objListStr.compare(noObjId) == 0)
        return true;

    //objListStr = QString("{\"Sharing\":false,\"ObjId\":\"1\",\"Size\":\"256\"}/{\"Sharing\":false,\"ObjId\":\"2\",\"Size\":\"128\"}/{\"Sharing\":true,\"Owner\":\"0x5f663f10F12503Cb126Eb5789A9B5381f594A0eB\",\"ObjId\":\"3\",\"Offset\":\"64\",\"Length\":\"128\",\"StartTime\":\"2019-01-01 00:00:00\",\"StopTime\":\"2020-01-01 00:00:00\",\"FileName\":\"123.txt\",\"SharerCSNodeID\":\"...\"}");
    QStringList list = objListStr.split(Ui::spec);
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
            if (!m_ObjInfos[objID].second.isEmpty())
                label = m_ObjInfos[objID].second;
            m_ObjInfos[objID] =
                    pair<qint64, QString>(info.objectSize.toLongLong() * MB_Bytes, label);
        }
        else
        {
            // Get sharings
            std::map<Ui::GUIStrings, std::string> stringTable = Ui::stringsTableEN;
            FileInfo inf(QString::fromStdString(stringTable[Ui::sharingSpaceLabel]),
                    info.fileName, info.offset, info.length, true,
                    info.objectID, info.owner, info.sharerCSNodeID, info.key,
                    info.startTime, info.stopTime);
            sharings.append(inf);
        }
    }

    return true;
}

bool MainWindow::WriteMetaDataOp(qint64 id, QString& metaData)
{
    if (metaData.length() > fHeadLength)
        return false;
    for(int i = 0; metaData.length() < fHeadLength; i++)
        metaData += " ";
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::blizWriteCmd], vector<QString>{QString::number(id), "0",
                metaData, m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        return false;
    else
        return true;
}

bool MainWindow::NewMetaDataOp(qint64 id, QString label, qint64 size)
{
    QString data = label + Ui::spec + QString::number(size) + Ui::spec + QString::number(size-fHeadLength);
    return WriteMetaDataOp(id, data);
}

bool MainWindow::UpdateMetaDataOp(qint64 id, QString fName, qint64 fSize, qint64& posStart, bool showNoMsg)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::blizReadCmd],
            vector<QString>{QString::number(id), "0", QString::fromStdString(fHeadLengthStr), m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseMetaErrHint]));
        return false;
    }

    QString qRet = QString::fromStdString(ret);
    qint64 total, spare;
    QString label;
    if (!FileManager::CheckHead4K(qRet, label, total, spare))
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseMetaErrHint]));
        return false;
    }

    if (!FileManager::CheckSpace(spare, fSize))
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::notEnoughSpaceHint]));
        return false;
    }

    QString newData = FileManager::UpdateMetaData(qRet, fHeadLength, fName, fSize, posStart);
    if (newData.compare(QString::fromStdString(m_SysMsgs[Ui::fNameduplicatedMsg])) == 0)
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox(QString::fromStdString(m_SysMsgs[Ui::fNameduplicatedMsg]));
        return false;
    }

    if (!WriteMetaDataOp(id, newData))
    {
        if (!showNoMsg)
            ShowOpErrorMsgBox();
        return false;
    }
    return true;
}

void MainWindow::DoDownloadFileOp(string filePath, QString fName)
{
    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr = "0";
    QString sharerAgentNodeID = "0";
    QString key = "0";
    QString spaceName = ui->curSpaceComboBox->currentText();
    qint64 objID = Utils::GetFileCompleteInfo(&m_VecFileList, &m_ObjInfos, fName, spaceName, offset, length, sharerAddr, sharerAgentNodeID, key,
                                              spaceName.toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) == 0);
    if (objID == 0 && sharerAddr == "0")
    {
        emit ShowDownFileOpRetSign(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));
        //ShowOpErrorMsgBox();
        return;
    }
    //GetFileOffsetLen(ui->curSpaceComboBox->currentText(), fName, offset, length);
    if (!AgentDetailsFunc::CheckFlow(m_AgentDetails[m_AgentConnectedAccount.toLower()], length))
    {
        emit ShowDownFileOpRetSign(QString::fromStdString(m_UIHints[Ui::needMoreFlow]));
        //ShowOpErrorMsgBox();
        return;
    }
    if (offset == 0 || length == 0)
    {
        emit ShowDownFileOpRetSign(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));
        //ShowOpErrorMsgBox();
        return;
    }

    filePath += Ui::spec.toStdString() + fName.toStdString();
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::blizReadFileCmd], vector<QString>{QString::number(objID),
                QString::number(offset), QString::number(length), sharerAddr, sharerAgentNodeID, key,
                QString::fromStdString(filePath), m_AccountPwd});

    emit ShowDownFileOpRetSign(QString::fromStdString(ret));
}

void MainWindow::ShowDownFileOpRet(QString qRet)
{
    if (m_MsgBox != nullptr)
        m_MsgBox->close();

    if (qRet.toStdString().compare(m_SysMsgs[Ui::errorMsg]) == 0)
    {
        ShowOpErrorMsgBox();
        return;
    }
    else if (qRet.toStdString().compare(m_UIHints[Ui::opTimedOutMsg]) == 0)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::opTimedOutMsg]));
        return;
    }
    else if (qRet.toStdString().compare(m_UIHints[Ui::needMoreFlow]) == 0)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::needMoreFlow]));
        return;
    }
    else
    {
        ui->uploadProgressBar->setValue(100);
        ShowOpSuccessMsgBox();
    }
}

void MainWindow::DoUploadFileOp(string filePath)
{
    qint64 objID = Utils::GetSpaceObjID(&m_ObjInfos, ui->curSpaceComboBox->currentText());
    if (objID == 0)
    {
        emit ShowUpFileOpRetSign(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));
        //ShowOpErrorMsgBox();
        return;
    }

    QFileInfo info(QString::fromStdString(filePath));
    qint64 offset = 0;
    if (!AgentDetailsFunc::CheckFlow(m_AgentDetails[m_AgentConnectedAccount.toLower()], info.size()))
    {
        emit ShowDownFileOpRetSign(QString::fromStdString(m_UIHints[Ui::needMoreFlow]));
        //ShowOpErrorMsgBox();
        return;
    }
    if (!UpdateMetaDataOp(objID, info.fileName(), info.size(), offset, true))
    {
        emit ShowUpFileOpRetSign(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));
        return;
    }

    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::blizWriteFileCmd], vector<QString>{QString::number(objID),
                QString::number(offset), info.fileName(), m_AccountPwd,
                QString::number(info.size()), QString::fromStdString(filePath)});

    emit ShowUpFileOpRetSign(QString::fromStdString(ret));
}

bool MainWindow::DoSignAgentOp(QString csAddr, QString expireTime, QString flowAmount, QString password, QString& result)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::signAgentCmd], vector<QString>{csAddr,
                expireTime, flowAmount, password});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        return false;

    result = QString::fromStdString(ret);
    return true;
}

bool MainWindow::DoCancelAgentOp(QString csAddr, QString password, QString& result)
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::cancelAgentCmd], vector<QString>{csAddr, password});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        return false;

    result = QString::fromStdString(ret);
    return true;
}

void MainWindow::ShowUpFileOpRet(QString qRet)
{
    if (m_MsgBox != nullptr)
        m_MsgBox->close();

    if (qRet.toStdString().compare(m_SysMsgs[Ui::errorMsg]) == 0)
        ShowOpErrorMsgBox();
    else if (qRet.toStdString().compare(m_UIHints[Ui::opTimedOutMsg]) == 0)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::opTimedOutMsg]));
        return;
    }
    else
    {
        ui->uploadProgressBar->setValue(100);
        ShowOpSuccessMsgBox(true, true);
        RefreshFiles();
    }
}

void MainWindow::EnableElements()
{
    bool hasAgentDefault = true;
    if (ui->agentToConnEdit->text().isEmpty())
        hasAgentDefault = false;
    ui->fastConnBtn->setEnabled(m_IsLoginFlag && !m_HasConnFlag && hasAgentDefault);
    ui->loginBtn->setEnabled(!m_IsLoginFlag);
    ui->logoutBtn->setEnabled(m_IsLoginFlag);
    ui->accountComboBox->setEnabled(!m_IsLoginFlag);

    ui->downloadBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag);
    ui->uploadBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                              ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
    ui->sharingBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                               ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
    ui->expSpaceBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag &&
                                ui->curSpaceComboBox->currentText().toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) != 0);
    ui->createSpaceBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && m_GetListFlag && m_AtServiceFlag && !m_BusyFlag);
    ui->updateFlistBtn->setEnabled(m_HasConnFlag && m_IsLoginFlag && !m_WaitingSpaceFlag && m_AtServiceFlag && !m_BusyFlag);
    ui->curSpaceComboBox->setEnabled(m_AtServiceFlag && !m_BusyFlag);
    ui->updatingLabel->setHidden(!m_WaitingSpaceFlag);

    ui->agSetDefBtn->setEnabled(!m_HasConnFlag);
    ui->deleteAgBtn->setEnabled(!m_HasConnFlag);
    ui->getAgDetailsBtn->setEnabled(m_HasConnFlag);

    bool canCancelService = true;
    QString currentAgent = ui->agentsComboBox->currentText();
    if (m_AgentDetails.find(currentAgent.toLower()) != m_AgentDetails.end())
        canCancelService = (m_AgentDetails[currentAgent.toLower()].IsAtService.compare(QString("Yes")) == 0);
    else
        canCancelService = false;
    ui->applyForServBtn->setEnabled(!ui->agentsComboBox->currentText().isEmpty() && !canCancelService && !m_BusyFlag &&
                                    !m_AtServiceFlag);
    ui->cancelServBtn->setEnabled(!ui->agentsComboBox->currentText().isEmpty() && canCancelService && !m_BusyFlag);
    ui->agentsComboBox->setEnabled(!m_BusyFlag);

    ui->accSetDefBtn->setEnabled(!m_IsLoginFlag);

    ui->newMailBtn->setEnabled(m_HasConnFlag && m_AtServiceFlag);
    ui->refreshMailBtn->setEnabled(m_HasConnFlag && m_AtServiceFlag);
    //ui->getMailAttachBtn->setEnabled(m_HasConnFlag && m_AtServiceFlag);

    ui->sendTxFromLineEdit->setText(m_AccountInUse);
}

void MainWindow::WaitingForNewSpaceCreating()
{
    const int timeOutLimit = 20; // 10s for time out
    int timeoutCnt = 0;
    while(m_WaitingSpaceFlag)
    {
        QApplication::processEvents(QEventLoop::AllEvents, 500);
        std::this_thread::sleep_for(std::chrono::milliseconds(500));

        if (!m_CreateSpaceTX.empty())
            continue;

        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getObjInfoCmd], vector<QString>{m_AccountPwd});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) != 0)
        {
            QString qRet = QString::fromStdString(ret);
            QList<FileInfo> sharingList;
            if (!ParseObjList(qRet, sharingList))
            {
                ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::parseObjErrHint]));
                return;//////////////////////////// TODO: recover 4096 data
            }
        }

        map<qint64, pair<qint64, QString> >::iterator iter = m_ObjInfos.find(m_NewObjID);
        if (iter != m_ObjInfos.end())
        {
            //m_SpaceOpTimer = false; // to wait TryNewMetaData() to return by stopping function in timer
            QApplication::processEvents();
            TryNewMetaData(iter->second.first);
            if (!m_WaitingSpaceFlag)
                break;
        }

        timeoutCnt++;
        if (timeoutCnt >= timeOutLimit)
            break;
    }

    RefreshFiles();
    m_BusyFlag = false;
    if (m_MsgBox != nullptr)
        m_MsgBox->close();
    if (timeoutCnt >= timeOutLimit)
        ShowOpErrorMsgBox(QString::fromStdString(m_SysMsgs[Ui::opTimedOutMsg]), true, true);
    else
        ShowOpSuccessMsgBox(true, true);
}

int MainWindow::MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
                               QMessageBox::StandardButton btn, bool modal, bool exec)
{
    if (m_MsgBox != nullptr)
    {
        delete m_MsgBox;
        m_MsgBox = nullptr;
    }
    m_MsgBox = new QMessageBox(icon, title, text, btn);
    m_MsgBox->setModal(modal);
    if (btn == QMessageBox::NoButton)
        m_MsgBox->setStandardButtons(QMessageBox::NoButton);
    if (!exec)
    {
        m_MsgBox->show();
        return 0;
    }
    else
        return m_MsgBox->exec();
}

void MainWindow::ShowOpErrorMsgBox(QString text, bool modal, bool exec)
{
    if (text.isEmpty())
        MessageBoxShow(QString::fromStdString(m_SysMsgs[Ui::errorMsg]),
                QString::fromStdString(m_UIHints[Ui::opFailedHint]),
                QMessageBox::Critical, QMessageBox::Ok, modal, exec);
    else
        MessageBoxShow(QString::fromStdString(m_SysMsgs[Ui::errorMsg]), text,
                QMessageBox::Critical, QMessageBox::Ok);
}

void MainWindow::ShowOpSuccessMsgBox(bool modal, bool exec)
{
    MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::opSucceedHint]),
            QMessageBox::Information, QMessageBox::Ok, modal, exec);
}

int MainWindow::ShowPasswdEnterDlg()
{
    QDialog* pwdDlg = new QDialog();
    pwdDlg->setModal(true);
    pwdDlg->setFixedSize(pwdEnterDlgSize);
    pwdDlg->setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    QVBoxLayout* layoutV = new QVBoxLayout(pwdDlg);
    QLabel* hint = new QLabel(QString::fromStdString(m_UIHints[Ui::needAccountpwdHint]));
    hint->setWordWrap(true);
    layoutV->addWidget(hint);
    pwdEdit->setEchoMode(QLineEdit::Password);
    pwdEdit->setFixedHeight(btnSize.height());
    layoutV->addWidget(pwdEdit);
    QHBoxLayout* layoutH = new QHBoxLayout();
    QPushButton* confirmBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::confirmBtn]));
    confirmBtn->connect(confirmBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(accept()));
    confirmBtn->setFixedSize(btnSize);
    QPushButton* cancelBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::cancelBtn]));
    cancelBtn->connect(cancelBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(close()));
    cancelBtn->setFixedSize(btnSize);
    QSpacerItem* spacer = new QSpacerItem(btnSize.width()/2, btnSize.height());
    layoutH->addWidget(confirmBtn);
    layoutH->addSpacerItem(spacer);
    layoutH->addWidget(cancelBtn);
    layoutV->addLayout(layoutH);

    return pwdDlg->exec();
}

int MainWindow::ShowCreateSpaceDlg()
{
    QDialog* pwdDlg = new QDialog();
    pwdDlg->setModal(true);
    pwdDlg->setFixedSize(createSpaceDlgSize);
    pwdDlg->setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    QVBoxLayout* layoutV = new QVBoxLayout(pwdDlg);
    QLabel* hintLabel = new QLabel(QString::fromStdString(m_UIHints[Ui::createSpaceHint]));
    hintLabel->setWordWrap(true);
    layoutV->addWidget(hintLabel);
    newSpaceLabelEdit->setFixedHeight(btnSize.height());
    layoutV->addWidget(newSpaceLabelEdit);
    QLabel* hintSize = new QLabel(QString::fromStdString(m_UIHints[Ui::createSpaceSizeHint]));
    hintSize->setWordWrap(true);
    layoutV->addWidget(hintSize);
    newSpaceSizeEdit->setFixedHeight(btnSize.height());
    layoutV->addWidget(newSpaceSizeEdit);

    QHBoxLayout* layoutH = new QHBoxLayout();
    QPushButton* confirmBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::confirmBtn]));
    confirmBtn->connect(confirmBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(accept()));
    confirmBtn->setFixedSize(btnSize);
    QPushButton* cancelBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::cancelBtn]));
    cancelBtn->connect(cancelBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(close()));
    cancelBtn->setFixedSize(btnSize);
    QSpacerItem* spacer = new QSpacerItem(btnSize.width()/8, btnSize.height());
    layoutH->addWidget(confirmBtn);
    layoutH->addSpacerItem(spacer);
    layoutH->addWidget(cancelBtn);
    layoutV->addLayout(layoutH);

    return pwdDlg->exec();
}

int MainWindow::ShowExpandSpaceDlg()
{
    QDialog* pwdDlg = new QDialog();
    pwdDlg->setModal(true);
    pwdDlg->setFixedSize(pwdEnterDlgSize);
    pwdDlg->setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    QVBoxLayout* layoutV = new QVBoxLayout(pwdDlg);
    QLabel* hint = new QLabel(QString::fromStdString(m_UIHints[Ui::needAccountpwdHint]));
    hint->setWordWrap(true);
    layoutV->addWidget(hint);
    pwdEdit->setEchoMode(QLineEdit::Password);
    pwdEdit->setFixedHeight(btnSize.height());
    layoutV->addWidget(pwdEdit);
    QHBoxLayout* layoutH = new QHBoxLayout();
    QPushButton* confirmBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::confirmBtn]));
    confirmBtn->connect(confirmBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(accept()));
    confirmBtn->setFixedSize(btnSize);
    QPushButton* cancelBtn =
            new QPushButton(QString::fromStdString(m_StringsTable[Ui::cancelBtn]));
    cancelBtn->connect(cancelBtn, SIGNAL(clicked(bool)), pwdDlg, SLOT(close()));
    cancelBtn->setFixedSize(btnSize);
    QSpacerItem* spacer = new QSpacerItem(btnSize.width()/2, btnSize.height());
    layoutH->addWidget(confirmBtn);
    layoutH->addSpacerItem(spacer);
    layoutH->addWidget(cancelBtn);
    layoutV->addLayout(layoutH);

    return pwdDlg->exec();
}

void MainWindow::on_actionEnglish_triggered()
{
    if (ui->actionChinese->isChecked())
    {
        ui->actionChinese->setChecked(false);
    }
    else
        ui->actionEnglish->setChecked(true);
}

void MainWindow::on_actionChinese_triggered()
{
    if (ui->actionEnglish->isChecked())
    {
        ui->actionEnglish->setChecked(false);
    }
    else
        ui->actionChinese->setChecked(true);
}

void MainWindow::on_fastConnBtn_clicked()
{
    MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::connectingHint]),
            QMessageBox::NoIcon, QMessageBox::NoButton, true, false);

    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::remoteIPCmd], vector<QString>{ui->agentToConnEdit->text()});
    if (ret != m_SysMsgs[Ui::errorMsg])
    {
        m_HasConnFlag = true;

        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::connSucceedHint]),
                QMessageBox::Information, QMessageBox::Ok, true, true);
        string balVal = DoGetAccBalanceOp();
        ShowAccBalance(QString::fromStdString(balVal), QString("0"));
        GetSignedAgents();

        m_AgentConnectedAccount = QString::fromStdString(ret);
        if (m_AgentDetails.find(QString::fromStdString(ret).toLower()) != m_AgentDetails.end() &&
            m_AgentDetails[QString::fromStdString(ret).toLower()].IsAtService.compare("Yes") == 0)
            m_AtServiceFlag = true;
        else
        {
            if (m_AgentDetails.find(QString::fromStdString(ret).toLower()) == m_AgentDetails.end())
            {
                AgentDetails agentDetails = GetAgentDetailsByIP(ui->agentToConnEdit->text());
                QString agentAcc = agentDetails.CsAddr;
                m_AgentDetails[agentAcc.toLower()] = agentDetails;
                ui->agentsComboBox->addItem(agentAcc);
                ui->agentsComboBox->setCurrentText(agentAcc);
            }
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::agentInvalidHint]),
                    QMessageBox::Warning, QMessageBox::Ok, true, false);
        }

        EnableElements();
        ShowAgentDetails();
        m_RefreshTimerId = startTimer(timeRefreshInterv);
        return;
    }

    ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::connFailedHint]));
}

void MainWindow::on_agentSettingBtn_clicked()
{
    ui->tabWidget->setCurrentIndex(2);
}

void MainWindow::on_newAccountBtn_clicked()
{
    if (!ui->pwd1LineEdit->text().isEmpty())
    {
        if (ui->pwd1LineEdit->text().compare(ui->pwd2LineEdit->text()) == 0)
        {
            string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::newAccountCmd], vector<QString>{ui->pwd1LineEdit->text()});
            if (ret == m_SysMsgs[Ui::errorMsg])
                ShowOpErrorMsgBox();
            else
            {
                AddAccount(QString::fromStdString(ret));
                if (m_AccountInUse.isEmpty())
                    SetDefaultAccount(QString::fromStdString(ret));

                ui->pwd1LineEdit->setText(QString(""));
                ui->pwd2LineEdit->setText(QString(""));
                ShowOpSuccessMsgBox();
            }
        }
        else
            MessageBoxShow(QString::fromStdString(m_UIHints[Ui::incorrectPwdTitle]),
                    QString::fromStdString(m_UIHints[Ui::confirmPwdErrHint]),
                    QMessageBox::Critical, QMessageBox::Ok);
    }
    else
        MessageBoxShow(QString::fromStdString(m_UIHints[Ui::incorrectPwdTitle]),
                QString::fromStdString(m_UIHints[Ui::newPwdEmptyHint]),
                QMessageBox::Critical, QMessageBox::Ok);
}

void MainWindow::on_accSettingBtn_clicked()
{
    ui->tabWidget->setCurrentIndex(3);
}

void MainWindow::on_loginBtn_clicked()
{
    if (ShowPasswdEnterDlg() == QDialog::Accepted)
    {
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::loginAccountCmd],
                vector<QString>{ui->accountComboBox->currentText(), pwdEdit->text()});
        if (ret.compare(m_SysMsgs[Ui::okMsg]) == 0)
        {
            m_IsLoginFlag = true;
            m_AccountPwd = pwdEdit->text();
            SetDefaultAccount(ui->accountComboBox->currentText());
            ShowOpSuccessMsgBox();
            EnableElements();
        }
        else
        {
            ShowOpErrorMsgBox();
            // TODO: show the reason
        }
    }

    pwdEdit->clear();
}

void MainWindow::on_logoutBtn_clicked()
{
    m_IsLoginFlag = false;
    EnableElements();
}

void MainWindow::on_agSetDefBtn_clicked()
{
    QModelIndexList modelIndexList = ui->agentList->selectionModel()->selectedIndexes();
    if (modelIndexList.length() == 0)
        return;
    const QModelIndex modalIndex = static_cast<QModelIndex>(modelIndexList.at(0));
    SetDefaultAgent(m_QAgentsModel->data(modalIndex, 0).toString());
    EnableElements();
}

void MainWindow::on_deleteAgBtn_clicked()
{
    QModelIndexList modelIndexList = ui->agentList->selectionModel()->selectedIndexes();
    if (modelIndexList.length() == 0)
        return;
    const QModelIndex modalIndex = static_cast<QModelIndex>(modelIndexList.at(0));
    DeleteAgent(m_QAgentsModel->data(modalIndex, 0).toString());
    EnableElements();
}

void MainWindow::on_addAgentBtn_clicked()
{
    if (ui->newAgentEdit->text().isEmpty())
        return;
    AddAgent(ui->newAgentEdit->text());
    ui->newAgentEdit->setText(QString(""));
    EnableElements();
}

void MainWindow::on_getAgDetailsBtn_clicked()
{
    QModelIndexList modelIndexList = ui->agentList->selectionModel()->selectedIndexes();
    if (modelIndexList.length() == 0)
        return;
    const QModelIndex modalIndex = static_cast<QModelIndex>(modelIndexList.at(0));
    AgentDetails details = GetAgentDetailsByIP(m_QAgentsModel->data(modalIndex, 0).toString());
    if (ui->agentsComboBox->findText(details.CsAddr) < 0)
        ui->agentsComboBox->addItem(details.CsAddr);
    ui->agentsComboBox->setCurrentText(details.CsAddr);
    ShowAgentDetails();
}

void MainWindow::on_accSetDefBtn_clicked()
{
    QModelIndexList modelIndexList = ui->accountList->selectionModel()->selectedIndexes();
    if (modelIndexList.length() == 0)
        return;
    const QModelIndex modalIndex = static_cast<QModelIndex>(modelIndexList.at(0));
    SetDefaultAccount(m_QAccountsModel->data(modalIndex, 0).toString());
    EnableElements();
}

void MainWindow::on_copyAccBtn_clicked()
{
    QModelIndexList modelIndexList = ui->accountList->selectionModel()->selectedIndexes();
    if (modelIndexList.length() == 0)
        return;
    const QModelIndex modalIndex = static_cast<QModelIndex>(modelIndexList.at(0));
    QClipboard* clipboard = QApplication::clipboard();
    clipboard->setText(m_QAccountsModel->data(modalIndex, 0).toString());
}

void MainWindow::on_refreshAccBtn_clicked()
{
    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::showAccountsCmd], vector<QString>());
    if(ret.compare(m_SysMsgs[Ui::errorMsg]) == 0)
    {
        ShowOpErrorMsgBox();
        return;
    }
    else if (ret.compare(m_SysMsgs[Ui::noAnyAccountMsg]) != 0)
        SetAccountList(QString::fromStdString(ret));
    EnableElements();
}

void MainWindow::on_downloadBtn_clicked()
{
    int row = ui->fileTable->currentIndex().row();
    if (row < 0)
    {
        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::downloadNoSelectHint]),
                QMessageBox::Information, QMessageBox::Ok, true, false);
        return;
    }
    QAbstractItemModel* model = ui->fileTable->model();
    QModelIndex index = model->index(row, 0);
    QString fName = model->data(index).toString();

    string filePath = QCoreApplication::applicationDirPath().toStdString();
    DataDirDlg fileDownSelectDlg(&filePath, m_LangENFlag, this, true, true);
    fileDownSelectDlg.setModal(true);
    fileDownSelectDlg.setFixedSize(upFileSelectDlgSize);
    fileDownSelectDlg.setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    fileDownSelectDlg.move((QApplication::desktop()->width() - fileDownSelectDlg.width())/2,
                           (QApplication::desktop()->height() - fileDownSelectDlg.height())/2);
    fileDownSelectDlg.SetText(QString::fromStdString(m_UIHints[Ui::downloadSelectHint]));
    if (fileDownSelectDlg.exec() != QDialog::Accepted)
    {
        fileDownSelectDlg.close();
        return;
    }

    MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::downloadingHint]),
            QMessageBox::NoIcon, QMessageBox::NoButton, true, false);

    DoDownloadFileOp(filePath, fName);
}

void MainWindow::on_uploadBtn_clicked()
{
    string filePath;
    ui->uploadProgressBar->setValue(0);
    DataDirDlg fileUpSelectDlg(&filePath, m_LangENFlag, this, false);
    fileUpSelectDlg.setModal(true);
    fileUpSelectDlg.setFixedSize(upFileSelectDlgSize);
    fileUpSelectDlg.setWindowTitle(QString::fromStdString(m_StringsTable[Ui::programTitle]));
    fileUpSelectDlg.move((QApplication::desktop()->width() - fileUpSelectDlg.width())/2,
                         (QApplication::desktop()->height() - fileUpSelectDlg.height())/2);
    fileUpSelectDlg.SetText(QString::fromStdString(m_UIHints[Ui::uploadSelectHint]));
    fileUpSelectDlg.SetSelectBtn();
    if (fileUpSelectDlg.exec() != QDialog::Accepted)
    {
        fileUpSelectDlg.close();
        return;
    }

    MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::uploadingHint]),
            QMessageBox::NoIcon, QMessageBox::NoButton, true, false);

    DoUploadFileOp(filePath);
}

void MainWindow::on_createSpaceBtn_clicked()
{
    if (ShowCreateSpaceDlg() == QDialog::Accepted)
    {
        QString objLabel = newSpaceLabelEdit->text();
        if (objLabel.toStdString().compare(m_StringsTable[Ui::sharingSpaceLabel]) == 0)
        {
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::spaceNameErrorHint]));
            return;
        }
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::createObjCmd], vector<QString>{QString::number(m_ObjInfos.size()+1),
                    QString::number(newSpaceSizeEdit->text().toLongLong() * MB_Bytes), m_AccountPwd});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
            ShowOpErrorMsgBox();
        else
        {
            //cout<<ret<<endl;
            QString qRet = QString::fromStdString(ret);
            QStringList list = qRet.split(Ui::spec);
            if (list.length() > 0)
            {
                MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                        QString::fromStdString(m_UIHints[Ui::creatingSpaceHint]),
                        QMessageBox::NoIcon, QMessageBox::NoButton, true, false);

                m_CreateSpaceTX = list.at(0).toStdString();
                string cmd = m_Cmds[Ui::createObjCmd];
                QApplication::processEvents();
                DoWaitTXCompleteOp(list.at(0), cmd);
            }
            else
            {
                ShowOpErrorMsgBox();
                return;
            }

            m_BusyFlag = true;
            m_WaitingSpaceFlag = true;
            m_NewObjID = int(m_ObjInfos.size()) + 1;
            m_NewSpaceLabel = newSpaceLabelEdit->text();
            WaitingForNewSpaceCreating();
        }
    }

    newSpaceLabelEdit->clear();
    newSpaceSizeEdit->clear();
    EnableElements();
}

void MainWindow::on_expSpaceBtn_clicked()
{

}

void MainWindow::on_updateFlistBtn_clicked()
{
    RefreshFiles();
    EnableElements();

//    QString abc("{\"Sharing\":false,\"ObjId\":\"1\",\"Size\":\"256\"}/{\"Sharing\":false,\"ObjId\":\"2\",\"Size\":\"128\"}/{\"Sharing\":true,\"Owner\":\"0x5f663f10F12503Cb126Eb5789A9B5381f594A0eB\",\"ObjId\":\"3\",\"Offset\":\"64\",\"Length\":\"128\",\"StartTime\":\"2019-01-01 00:00:00\",\"StopTime\":\"2020-01-01 00:00:00\",\"FileName\":\"123.txt\",\"SharerCSNodeID\":\"...\"}");
//    QStringList list = abc.split(Ui::spec);
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
}

void MainWindow::on_curSpaceComboBox_currentTextChanged(const QString &arg1)
{
    map<qint64, pair<qint64, QString> >::iterator iter;
    for (iter = m_ObjInfos.begin(); iter != m_ObjInfos.end(); iter++)
    {
        if (iter->second.second.compare(arg1) == 0)
            ShowFileTable(iter->second.first, arg1);
    }

    if (m_StringsTable[Ui::sharingSpaceLabel].compare(arg1.toStdString()) == 0)
        ShowFileTable(0, arg1);
}

void MainWindow::on_copyAgentBtn_clicked()
{
    if (ui->agentsComboBox->currentText().isEmpty())
        return;
    QClipboard* clipboard = QApplication::clipboard();
    clipboard->setText(ui->agentsComboBox->currentText());
}

void MainWindow::on_agentsComboBox_currentTextChanged(const QString &arg1)
{
    if (arg1.compare(m_AgentConnectedAccount) == 0)
        ui->AgentDetailLabel->setText(QString("Details (current connected):"));
    else
        ui->AgentDetailLabel->setText(QString("Details:"));

    ShowAgentDetails();
}

void MainWindow::on_accountList_clicked(const QModelIndex &index)
{
    if (HasConnection && m_HasConnFlag)
    {
        string ret = DoGetAccBalanceOp(m_QAccountsModel->data(index, 0).toString());
        ShowAccBalance(QString::fromStdString(ret), QString("0"), false);

        //ui->sendTxFromLineEdit->setText(m_QAccountsModel->data(index, 0).toString());
    }
}

void MainWindow::on_applyForServBtn_clicked()
{
    if (ui->agentsComboBox->currentText().isEmpty())
        return;

    QString start, stop, flow;
    SignAgentDlg dlg(&start, &stop, &flow);
    if (dlg.exec() == QDialog::Accepted)
    {
        QString agentAccount = ui->agentsComboBox->currentText();
        QString flowAmount, timePeriod, ret;
        if (flow.compare("0") != 0)
        {
            flowAmount = QString::number(flow.toLongLong() * GB_Bytes);
            //flowAmount = QString::number(flow.toLongLong()); // for dev purpose only
            timePeriod = "0000-00-00 00:00:00~0000-00-00 00:00:00";
        }
        else
        {
            flowAmount = "0";
            timePeriod = start + "~" + stop;
        }

        if (!DoSignAgentOp(agentAccount, timePeriod, flowAmount, m_AccountPwd, ret))
            ShowOpErrorMsgBox();
        else
        {
            if (agentAccount.compare(m_AgentConnectedAccount) == 0)
                m_AtServiceFlag = true;
            UpdateAgentDetails(ret, m_Cmds[Ui::signAgentCmd]);
            ShowOpSuccessMsgBox();
        }
    }
}

void MainWindow::on_cancelServBtn_clicked()
{
    if (ui->agentsComboBox->currentText().isEmpty())
        return;

    QMessageBox::StandardButton button;
    button = QMessageBox::question(this, QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::cancelAgentHint]),
            QMessageBox::Yes|QMessageBox::No);
    if(button == QMessageBox::No)
        return;

    QString agentAccount = ui->agentsComboBox->currentText();
    QString IP = m_AgentDetails[agentAccount].IPAddress;
    QString ret;
    if (!DoCancelAgentOp(agentAccount, m_AccountPwd, ret))
        ShowOpErrorMsgBox();
    else
    {
        if (agentAccount.compare(m_AgentConnectedAccount) == 0)
            m_AtServiceFlag = false;
        UpdateAgentDetails(ret, m_Cmds[Ui::cancelAgentCmd]);
        ShowOpSuccessMsgBox();
    }
}

void MainWindow::on_sharingBtn_clicked()
{
    bool noFileSelected = false;
    int row = ui->fileTable->currentIndex().row();
    if (row < 0)
        noFileSelected = true;

    SharingFileDlg dlg(noFileSelected, m_AccountPwd, GetMyAgentNoneID(), m_UIHints, &m_BackRPC);
    if (!noFileSelected)
    {
        QAbstractItemModel* model = ui->fileTable->model();
        QModelIndex index = model->index(row, 0);
        QString fName = model->data(index).toString();

        qint64 offset = 0;
        qint64 length = 0;
        QString sharerAddr = "0";
        QString sharerAgentNodeID = "0";
        QString key = "0";
        qint64 objID = Utils::GetFileCompleteInfo(&m_VecFileList, &m_ObjInfos, fName, ui->curSpaceComboBox->currentText(), offset,
                                                  length, sharerAddr, sharerAgentNodeID, key);
        if (objID == 0)
            ShowOpErrorMsgBox(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));

        dlg.SetFileInfo(fName, objID, offset, length);
    }

    if (dlg.exec() == QDialog::Accepted) {}
}

void MainWindow::on_sendTxBtn_clicked()
{
    if (!(HasConnection && m_HasConnFlag))
    {
        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::needConnectionHint]),
                QMessageBox::Warning, QMessageBox::Ok, true, false);
        return;
    }

    if (ui->sendTxFromLineEdit->text().isEmpty())
    {
        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::selectAccToSendHint]),
                QMessageBox::Warning, QMessageBox::Ok, true, false);
        return;
    }

    if (ui->sendTxToLineEdit->text().isEmpty())
    {
        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::enterReceiverToSendHint]),
                QMessageBox::Warning, QMessageBox::Ok, true, false);
        return;
    }

    QString amount = ui->sendTxAmtLineEdit->text();
    if (amount.isEmpty())
    {
        MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                QString::fromStdString(m_UIHints[Ui::enterAmountToSendHint]),
                QMessageBox::Warning, QMessageBox::Ok, true, false);
        return;
    }
    else
    {
        double number = amount.toDouble();
        if (number > ui->balSelVal->text().toDouble())
        {
            MessageBoxShow(QString::fromStdString(m_StringsTable[Ui::programTitle]),
                    QString::fromStdString(m_UIHints[Ui::amountExceededHint]),
                    QMessageBox::Warning, QMessageBox::Ok, true, false);
            return;
        }
    }

    QMessageBox::StandardButton button;
    button = QMessageBox::question(this, QString::fromStdString(m_StringsTable[Ui::programTitle]),
            QString::fromStdString(m_UIHints[Ui::sendTransactionHint]),
            QMessageBox::Yes|QMessageBox::No);
    if(button == QMessageBox::No)
        return;

    string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::sendTransactionCmd], vector<QString>{ui->sendTxFromLineEdit->text(),
                ui->sendTxToLineEdit->text(), amount, m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
        ShowOpErrorMsgBox();
    else
        ShowOpSuccessMsgBox(true, true);

    ui->sendTxFromLineEdit->setText("");
    ui->sendTxToLineEdit->setText("");
    ui->sendTxAmtLineEdit->setText("");
    ui->balSelVal->setText("");
    ui->balSelPendVal->setText("");
}

void MainWindow::on_tabWidget_currentChanged(int index)
{
    if (!m_HasConnFlag || !m_AtServiceFlag)
        return;

    if (index == 1)
        RefreshMails();
}

void MainWindow::on_newMailBtn_clicked()
{
    QString* mailContent = new(QString);
    NewMailDlg dlg(mailContent, m_AccountInUse, &m_ObjInfos, &m_VecFileList, m_AccountPwd, GetMyAgentNoneID(), &m_BackRPC);
    if (dlg.exec() == QDialog::Accepted)
    {
        string ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::getSignatureCmd],
                std::vector<QString>{*mailContent, m_AccountPwd});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0)
        {
            ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::getSignFailedHint]));
            return;
        }

        MailDetails mailSigned;
        mailSigned.DecodeQJson(*mailContent);
        mailSigned.SetSignature(QString::fromStdString(ret));

        QString mailSignedStr = mailSigned.EncodeQJson();
        ret = m_BackRPC.SendOperationCmd(m_Cmds[Ui::sendMailCmd], vector<QString>{mailSignedStr, m_AccountPwd});
        if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0 || ret.compare(m_SysMsgs[Ui::opTimedOutMsg]) == 0)
            ShowOpErrorMsgBox();
        else
            ShowOpSuccessMsgBox(true, true);

        //ParseMailList(mailSignedStr); ShowMailTable(); // Test
    }

    delete mailContent;
}

void MainWindow::on_refreshMailBtn_clicked()
{
    RefreshMails();
}

void MainWindow::on_getMailAttachBtn_clicked()
{
    if (m_MailCurrentSelected.mailFileSharingCode.isEmpty())
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::getMailAttachFailedHint]));
        return;
    }

    FileSharingInfo fileJson;
    fileJson.DecodeQJson(m_MailCurrentSelected.mailFileSharingCode);
    QString sharerAgentNodeID = fileJson.GetShareAgentNodeID();
    SharingFileDlg dlg(true, m_AccountPwd, sharerAgentNodeID, m_UIHints, &m_BackRPC);
    dlg.SetSharingCode(m_MailCurrentSelected.mailFileSharingCode);
    if (dlg.exec() == QDialog::Accepted) {}
}

void MainWindow::on_mailTable_clicked(const QModelIndex &index)
{
    QAbstractItemModel* model = ui->mailTable->model();
    QModelIndex indexAttachmentFlag = model->index(index.row(), 2);
    QString attachmentFlag = model->data(indexAttachmentFlag).toString();
    QModelIndex indexTitle = model->index(index.row(), 1);
    QString title = model->data(indexTitle).toString();
    QModelIndex indexSender = model->index(index.row(), 0);
    QString sender = model->data(indexSender).toString();
    QModelIndex indexTime = model->index(index.row(), 3);
    QString time = model->data(indexTime).toString();
    QString content;
    bool foundFlag = false;
    for (size_t i = 0; i < m_MailList.size(); ++i)
    {
        if (m_MailList[i].mailTimeStamp.compare(time) == 0)
        {
            content = m_MailList[i].mailContent;
            m_MailCurrentSelected = m_MailList[i];
            foundFlag = true;
        }
    }
    if (!foundFlag)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_UIHints[Ui::openMailFailedHint]));
        return;
    }

    ui->getMailAttachBtn->setEnabled((attachmentFlag.compare("yes") == 0) && m_HasConnFlag && m_AtServiceFlag);
    ui->mailTitleLabel->setText(title);
    ui->mailFromLabel->setText(sender);
    ui->mailTextEdit->setPlainText(content);
}
