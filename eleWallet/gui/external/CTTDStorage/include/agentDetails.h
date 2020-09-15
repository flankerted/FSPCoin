#ifndef AGENTDETAILS_H
#define AGENTDETAILS_H

#include <QAbstractListModel>
#include <QString>
#include <map>

typedef struct _AgentDetails
{
    QString CsAddr;
    QString IPAddress;
    QString TStart;
    QString TEnd;
    QString FlowAmount;
    QString FlowUsed;
    QString PaymentMeth;
    QString Price;
    QString AgentNodeID;

    QString IsAtService;
}AgentDetails;

typedef struct _AgentInfo
{
    QString CsAddr;
    QString IPAddress;
    QString PaymentMeth;
    QString Price;
}AgentInfo;

class AgentDetailsFunc
{
public:
    AgentDetailsFunc();

    ~AgentDetailsFunc();

    static bool ParseAgentDetails(const QString& rawData, std::map<QString, AgentDetails>& agentDetails,
                                  std::map<QString, QString>* agentInfo=nullptr);

    static QString IsAtService(QString paymentMeth, QString tStart, QString tEnd, QString flowAmount, QString flowUsed);

    static bool CheckIfAtService(AgentDetails currAgent);

    static bool CheckFlow(AgentDetails currAgent, qint64 fileSize);

    static bool ParseAgentInfo(const QString& rawData, std::map<QString, AgentDetails>& agentDetails,
                               std::map<QString, AgentInfo>& agentInfo);
};

// For "agentList" in agentConn.qml only
class QAgentModel : public QAbstractListModel
{
    Q_OBJECT

public:
    enum QAgentModelRole
    {
        IpRole = Qt::UserRole + 1,
        PriceRole,
        PaymentRole
    };

    QAgentModel(QObject* parent = nullptr);

    ~QAgentModel() override;

    int rowCount(const QModelIndex& parent = QModelIndex()) const override;

    QVariant data(const QModelIndex& index, int role) const override;

    virtual QHash<int, QByteArray> roleNames() const override;

    Qt::ItemFlags flags(const QModelIndex& index) const override;

    //bool setData(const QModelIndex& index, const QVariant& value, int role) override;

    bool insertRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    //bool removeRows(int position, int rows, const QModelIndex& parent = QModelIndex()) override;

    bool clearData(const QModelIndex& parent = QModelIndex());

public:
    void SetModel(const std::map<QString, AgentInfo>& agentInfo);

    QString GetAgentIP(int index);

    QString GetAgentAccount(int index);

    QString GetAgentAvailablePayment(int index);

private:
    QList<AgentInfo> m_AgentList;

};

#endif // AGENTDETAILS_H
