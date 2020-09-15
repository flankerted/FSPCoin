#include <iostream>
#include <thread>
#include <QApplication>
#include <QMetaObject>
#include "eleWalletBackRPC.h"

using namespace std;

EleWalletBackRPCCli::EleWalletBackRPCCli(QString ip, quint16 port) :
    m_MsgDialog(nullptr),
    m_GuiString(GUIString::GetSingleton()),
    m_Ip(ip),
    m_Port(port),
    m_CmdIndex(0)
    //m_NewIncomingOpRspFlag(false),
    //m_Rate(showRate)
{
    for(size_t i = 0; i < m_GuiString->RPCCmdLength(); ++i)
        m_RPCCommands[GUIString::RpcCmds(i)] = m_GuiString->rpcCmds(GUIString::RpcCmds(i)).toStdString();

    m_Socket = new QTcpSocket();
    connect(m_Socket, SIGNAL(connected()), this, SLOT(connectSuc()));
    connect(m_Socket, SIGNAL(disconnected()), this, SLOT(connectDis()));
    connect(m_Socket, SIGNAL(error(QAbstractSocket::SocketError)), this, SLOT(connectErr(QAbstractSocket::SocketError)));
    connect(m_Socket, SIGNAL(readyRead()), this, SLOT(readData()));
    connect(this, SIGNAL(showMsgBoxSig(QString, int)), this, SLOT(showMsgBox(QString, int)), Qt::QueuedConnection);
    connect(this, SIGNAL(writeSocketSig(QString)), this, SLOT(writeSocket(QString)), Qt::QueuedConnection);
}

EleWalletBackRPCCli::~EleWalletBackRPCCli()
{
    delete m_Socket;
}

void EleWalletBackRPCCli::SetRate(int* rate)
{
    m_Rate = rate;
}

void EleWalletBackRPCCli::ConnectBackend(bool* connectedFlag)
{
    m_Connected = connectedFlag;
    m_Socket->connectToHost(m_Ip, m_Port);
}

bool EleWalletBackRPCCli::IsConnected()
{
    return m_Connected;
}

QString EleWalletBackRPCCli::recoverJsonStr(int i, int len, QString s)
{
    if (i == 0)
        s += "}";
    else if (i == len-1)
        s = "{" + s;
    else
        s = "{" + s + "}";

    return s;
}

void EleWalletBackRPCCli::readData()
{
    m_MsgLocker.lock();
    QString msg = m_Socket->readAll();
    QStringList list = msg.split("}{");
    if (list.length() > 1)
    {
        cout<<"new incoming msgs cnt: 2"<<endl;
        for(int i = 0; i < list.length(); ++i)
            handleJsonMsg(recoverJsonStr(i, list.length(), list[i]));
    }
    else
        handleJsonMsg(msg);
    m_MsgLocker.unlock();
}

void EleWalletBackRPCCli::handleJsonMsg(const QString& msg)
{
    Utils::BackendMsg newMsg;
    newMsg.DecodeQJson(msg);

    if (newMsg.IsOpRspMsg())
        m_OpRspMsgQueue[newMsg.GetOpCmd()] = newMsg;

    if (newMsg.GetOpCmd().compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccTBalanceCmd])) != 0 &&
            newMsg.GetOpCmd().compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccBalanceCmd])) != 0 &&
            newMsg.GetOpCmd().compare(QString::fromStdString(m_RPCCommands[GUIString::GetSignedAgentInfoCmd])) != 0)
        cout<<"new incoming msg: "<<msg.toStdString()<<endl;

    if (newMsg.IsRateMsg())
        *m_Rate = newMsg.OpRet.toInt();
}

void EleWalletBackRPCCli::connectSuc()
{
    *m_Connected = true;
//    std::cout<<"connected"<<std::endl;
}

void EleWalletBackRPCCli::connectErr(QAbstractSocket::SocketError)
{
    *m_Connected = false;
    //std::cout<<"connect error"<<std::endl;
}

void EleWalletBackRPCCli::connectDis()
{
    *m_Connected = false;
    //std::cout<<"disconnected"<<std::endl;
}

void EleWalletBackRPCCli::showMsgBox(QString msg, int dlgType)
{
    if (m_MsgDialog == nullptr)
        return;
    //QMetaObject::invokeMethod(msgDialog, "openMsg", Q_ARG(QString, "world hello"));
    m_MsgDialog->setProperty("visible", true);
    QObject *textObject = m_MsgDialog->findChild<QObject*>("content");
    textObject->setProperty("text", msg); //TODO: classify error or success

    if (dlgType == GUIString::HintDlg)
    {
        QMetaObject::invokeMethod(m_MsgDialog, "closeWaitingBox");
        QMetaObject::invokeMethod(m_MsgDialog, "openHintBox");
    }
    else if(dlgType == GUIString::SelectionDlg)
        QMetaObject::invokeMethod(m_MsgDialog, "openSelectionBox");
    else if(dlgType == GUIString::WaitingDlg)
        QMetaObject::invokeMethod(m_MsgDialog, "openWaitingBox");
}

void EleWalletBackRPCCli::SetMsgDialog(QObject* qmlObj)
{
    m_MsgDialog = qmlObj;
}

void EleWalletBackRPCCli::writeSocket(QString msg)
{
    m_Socket->write(msg.toUtf8());
}

string EleWalletBackRPCCli::SendOperationCmd(GUIString::RpcCmds opCmd, const std::vector<QString>& opParams)
{
    QList<QString> paramList;
    for(size_t i = 0; i < opParams.size(); ++i)
        paramList.append(opParams[i]);
    return sendOperationCmd(opCmd, paramList).toStdString();
}

QString EleWalletBackRPCCli::sendOperationCmdThread(int cmd, QList<QString> opParams)
{
    int timeOutLimit = (cmd != GUIString::WaitTXCompleteCmd) ? opTimeOut : txTimeOut;
    int cmdIndex = m_CmdIndex;
    Utils::OperationCmdReq req;
    QString cmdStr = QString::fromStdString(m_GuiString->rpcCmds(GUIString::RpcCmds(cmd)).toStdString());
    req.SetValues(cmdStr, opParams, m_CmdIndex++);
    QString reqStr = req.EncodeQJson();
    emit writeSocketSig(reqStr);
    //m_Socket->write(reqStr.toUtf8());

    if (cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccTBalanceCmd])) != 0 &&
            cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccBalanceCmd])) != 0 &&
            cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetSignedAgentInfoCmd])) != 0)
        cout<<"sent command msg: "<<reqStr.toStdString()<<endl;

    if (cmdStr.toStdString().compare(m_RPCCommands[GUIString::ExitExternGUICmd]) == 0)
    {
#ifdef WIN32
        timeOutLimit = 10;
#else
        timeOutLimit = 10000;
#endif
        clock_t now = clock();
        while(clock() - now < timeOutLimit)
            QApplication::processEvents(QEventLoop::AllEvents, 100);
        return "exit";
    }
    else if(cmdStr.toStdString().compare(m_RPCCommands[GUIString::BlizWriteFileCmd]) == 0 ||
            cmdStr.toStdString().compare(m_RPCCommands[GUIString::BlizReadFileCmd]) == 0)
        timeOutLimit = timeOutLimit * 60 * 24 * 356; // for read file or write file, time out limit is almost unlimited

    emit showMsgBoxSig("running", GUIString::WaitingDlg);

    std::thread t(bind(&EleWalletBackRPCCli::sendOperationCmd, this, cmd, opParams, true, timeOutLimit, cmdIndex));
    t.detach();
    return "runing in thread";
}

QString EleWalletBackRPCCli::sendOperationCmd(int cmd, QList<QString> opParams,
                                              bool isthread, int timeOutLimit, int cmdInx)
{
    int timeOut = timeOutLimit; //timeOutLimit > 0 ? timeOutLimit : (cmd == GUIString::WaitTXCompleteCmd) ? opTimeOut : txTimeOut;
    QString cmdStr = QString::fromStdString(m_GuiString->rpcCmds(GUIString::RpcCmds(cmd)).toStdString());
    int cmdIndex = cmdInx;
    if (!isthread)
    {
        int timeOutLimit = opTimeOut;
        cmdIndex = m_CmdIndex;
        Utils::OperationCmdReq req;
        QString cmdStr = QString::fromStdString(m_GuiString->rpcCmds(GUIString::RpcCmds(cmd)).toStdString());
        req.SetValues(cmdStr, opParams, m_CmdIndex++);
        QString reqStr = req.EncodeQJson();
        emit writeSocketSig(reqStr);
        //m_Socket->write(reqStr.toUtf8());

        if (cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccTBalanceCmd])) != 0 &&
                cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetAccBalanceCmd])) != 0 &&
                cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::GetSignedAgentInfoCmd])) != 0)
            cout<<"sent command msg: "<<reqStr.toStdString()<<endl;

        if (cmdStr.compare(QString::fromStdString(m_RPCCommands[GUIString::NewAccountCmd])) == 0)
            timeOutLimit = timeOutLimit * 2;

        if (cmdStr.toStdString().compare(m_RPCCommands[GUIString::ExitExternGUICmd]) == 0)
        {
#ifdef WIN32
            timeOut = 10;
#else
            timeOut = 10000;
#endif
            clock_t now = clock();
            while(clock() - now < timeOut)
                QApplication::processEvents(QEventLoop::AllEvents, 100);
            return "";
        }
        else if(cmdStr.toStdString().compare(m_RPCCommands[GUIString::BlizWriteFileCmd]) == 0 ||
                cmdStr.toStdString().compare(m_RPCCommands[GUIString::BlizReadFileCmd]) == 0)
            timeOutLimit = timeOutLimit * 60 * 24 * 356; // for read file or write file, time out limit is almost unlimited
        else if (cmdStr.toStdString().compare(m_RPCCommands[GUIString::RemoteIPCmd]) == 0) {
            timeOutLimit = timeOutLimit * 12; // 2 minutes, when it is a cs server that is reading or writing a big file, it need more time
        }

        timeOut = timeOutLimit;
    }

    clock_t now = clock();
    while(clock() - now < timeOut)
    {
        m_MsgLocker.lock();
        QString ret;
        std::map<QString, Utils::BackendMsg>::iterator it = m_OpRspMsgQueue.find(cmdStr);
        if (it != m_OpRspMsgQueue.end() && cmdIndex == m_OpRspMsgQueue[cmdStr].GetOpIndex())
        {
            ret = m_OpRspMsgQueue[cmdStr].GetOpRet();
            m_OpRspMsgQueue.erase(it);
            m_MsgLocker.unlock();
            if (isthread)
                emit showMsgBoxSig(ret, GUIString::HintDlg);
            return ret;
        }
        m_MsgLocker.unlock();

        if (!isthread)
            QApplication::processEvents(QEventLoop::AllEvents, 50);
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }

    if (isthread)
        ;//TODO: emit showMsgBoxSig(ret, GUIString::HintDlg);
    return QString::fromStdString(m_GuiString->sysMsgs(GUIString::OpTimedOutMsg).toStdString());
}
