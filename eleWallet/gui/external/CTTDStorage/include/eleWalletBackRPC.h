#ifndef ELEWALLETBACKRPC_H
#define ELEWALLETBACKRPC_H

#include <mutex>
#include <QObject>
#include <QTcpSocket>
#include <QThread>
#include "utils.h"

class EleWalletBackRPCCli : public QThread
{
    Q_OBJECT
public:
    Q_INVOKABLE QString sendOperationCmd(int opCmd, QList<QString> opParams, bool isthread=false, int timeOutLimit=0, int cmdInx=0);

    Q_INVOKABLE QString sendOperationCmdThread(int opCmd, QList<QString> opParams);

signals:
    void showMsgBoxSig(QString msg, int btnType);

    void writeSocketSig(QString msg);

public:
    explicit EleWalletBackRPCCli(QString ip, quint16 port);

    ~EleWalletBackRPCCli();

    void SetRate(int* rate);

    void SetMsgDialog(QObject* qmlObj);

    void ConnectBackend(bool* connectedFlag);

    bool IsConnected();

    // Assembles the client's payload, sends it and presents the response back
    // from the server.
    std::string SendOperationCmd(GUIString::RpcCmds opCmd, const std::vector<QString>& opParams);

private:
    QObject* m_MsgDialog;

    //std::map<CTTUi::GUIStrings, std::string> m_SysMsg;
    std::map<GUIString::RpcCmds, std::string> m_RPCCommands;
    GUIString* m_GuiString;
    QTcpSocket* m_Socket;
    QString m_Ip;
    quint16 m_Port;
    std::map<QString, Utils::BackendMsg> m_OpRspMsgQueue;

    //bool m_NewIncomingOpRspFlag;
    bool* m_Connected;
    int* m_Rate;
    int m_CmdIndex;

    std::mutex m_MsgLocker;

private slots:
    void readData();

    void connectSuc();

    void connectErr(QAbstractSocket::SocketError);

    void connectDis();

    void showMsgBox(QString msg, int dlgType);

    void writeSocket(QString msg);

private:
    QString recoverJsonStr(int i, int len, QString s);

    void handleJsonMsg(const QString& msg);

private:
#ifdef WIN32
    const int opTimeOut = 10000; //10s for time out

    const int txTimeOut = 60000; //60s for waiting tx time out
#else
    const int opTimeOut = 10000000; //10s for time out

    const int txTimeOut = 60000000; //60s for waiting tx time out
#endif
};


#endif // ELEWALLETBACKRPC_H
