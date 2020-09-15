#ifndef ELEWALLETBACKRPC_H
#define ELEWALLETBACKRPC_H

#include <mutex>
#include <QObject>
#include <QTcpSocket>
#include "utils.h"

class EleWalletBackRPCCli : public QObject {
    Q_OBJECT
public:
    explicit EleWalletBackRPCCli(QString ip, quint16 port, int* showRate, QObject* parent = nullptr);

    ~EleWalletBackRPCCli();

    void ConnectBackend(bool *connectedFlag);

    bool IsConnected();

    // Assembles the client's payload, sends it and presents the response back
    // from the server.
    std::string SendOperationCmd(std::string opCmd, const std::vector<QString> &opParams);

private:
    std::map<Ui::GUIStrings, std::string> m_SysMsg;
    std::map<Ui::GUIStrings, std::string> m_RPCCommands;
    QTcpSocket* m_Socket;
    QString m_Ip;
    quint16 m_Port;
    std::map<QString, Utils::BackendMsg> m_OpRspMsgQueue;

    //bool m_NewIncomingOpRspFlag;
    bool* m_Connected;
    int* m_Rate;

    std::mutex msgLocker;

private slots:
    void readData();

    void connectSuc();

    void connectErr(QAbstractSocket::SocketError);

    void connectDis();

private:
    QString recoverJsonStr(int i, int len, QString s);

    void handleJsonMsg(const QString& msg);
};


#endif // ELEWALLETBACKRPC_H
