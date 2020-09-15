#include <iostream>
#include <thread>
#include <QApplication>
#include "eleWalletBackRPC.h"

using namespace std;

EleWalletBackRPCCli::EleWalletBackRPCCli(QString ip, quint16 port, int* showRate, QObject* parent) :
    QObject(parent),
    m_SysMsg(Ui::SysMessages),
    m_RPCCommands(Ui::RPCCommands),
    m_Ip(ip),
    m_Port(port),
    //m_NewIncomingOpRspFlag(false),
    m_Rate(showRate)
{
    m_Socket = new QTcpSocket();
    connect(m_Socket, SIGNAL(connected()), this, SLOT(connectSuc()));
    connect(m_Socket, SIGNAL(disconnected()), this, SLOT(connectDis()));
    connect(m_Socket, SIGNAL(error(QAbstractSocket::SocketError)), this, SLOT(connectErr(QAbstractSocket::SocketError)));
    connect(m_Socket, SIGNAL(readyRead()), this, SLOT(readData()));
}

EleWalletBackRPCCli::~EleWalletBackRPCCli()
{
    delete m_Socket;
}

void EleWalletBackRPCCli::ConnectBackend(bool* connectedFlag)
{
    m_Socket->connectToHost(m_Ip, m_Port);
    m_Connected = connectedFlag;
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
    msgLocker.lock();
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
    msgLocker.unlock();
}

void EleWalletBackRPCCli::handleJsonMsg(const QString& msg)
{
    Utils::BackendMsg newMsg;
    newMsg.DecodeQJson(msg);

    if (newMsg.IsOpRspMsg())
        m_OpRspMsgQueue[newMsg.GetOpCmd()] = newMsg;

    if (newMsg.GetOpCmd().compare(QString::fromStdString(m_RPCCommands[Ui::getAccTBalanceCmd])) != 0 &&
            newMsg.GetOpCmd().compare(QString::fromStdString(m_RPCCommands[Ui::getSignedAgentInfoCmd])) != 0)
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

string EleWalletBackRPCCli::SendOperationCmd(string opCmd, const std::vector<QString> &opParams)
{
    int timeOutLimit = 60000; //60s for time out
    QString cmd = QString::fromStdString(opCmd);
    Utils::OperationCmdReq req;
    req.SetValues(cmd, opParams);
    QString reqStr = req.EncodeQJson();
    m_Socket->write(reqStr.toUtf8());

    if (cmd.compare(QString::fromStdString(m_RPCCommands[Ui::getAccTBalanceCmd])) != 0 &&
            cmd.compare(QString::fromStdString(m_RPCCommands[Ui::getSignedAgentInfoCmd])) != 0)
        cout<<"sent command msg: "<<reqStr.toStdString()<<endl;

    if (opCmd.compare(m_RPCCommands[Ui::exitExternGUICmd]) == 0)
        return "";
    else if(opCmd.compare(m_RPCCommands[Ui::blizWriteFileCmd]) == 0 || opCmd.compare(m_RPCCommands[Ui::blizReadFileCmd]) == 0)
        timeOutLimit = timeOutLimit * 60 * 24 * 356; // for read file or write file, time out limit is almost unlimited

    clock_t now = clock();
    while(clock() - now < timeOutLimit)
    {
        msgLocker.lock();
        QString ret;
        std::map<QString, Utils::BackendMsg>::iterator it = m_OpRspMsgQueue.find(QString::fromStdString(opCmd));
        if (it != m_OpRspMsgQueue.end())
        {
            ret = m_OpRspMsgQueue[QString::fromStdString(opCmd)].GetOpRet();
            m_OpRspMsgQueue.erase(it);
            msgLocker.unlock();
            return ret.toStdString();
        }
        msgLocker.unlock();

        QApplication::processEvents(QEventLoop::AllEvents, 50);
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }

    return m_SysMsg[Ui::opTimedOutMsg];
}
