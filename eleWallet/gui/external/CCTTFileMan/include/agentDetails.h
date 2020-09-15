#ifndef AGENTDETAILS_H
#define AGENTDETAILS_H

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

    static bool CheckFlow(AgentDetails currAgent, qint64 fileSize);

    static bool ParseAgentInfo(const QString& rawData, std::map<QString, AgentDetails>& agentDetails,
                               std::map<QString, QString>& agentInfo);
};

#endif // AGENTDETAILS_H
