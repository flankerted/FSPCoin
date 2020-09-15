#include <iostream>
#include <QJsonParseError>
#include <QJsonObject>
#include <QJsonArray>
#include <QDateTime>
#include "agentDetails.h"
#include "strings.h"

using namespace std;

AgentDetailsFunc::AgentDetailsFunc()
{

}

AgentDetailsFunc::~AgentDetailsFunc()
{

}

bool AgentDetailsFunc::ParseAgentDetails(const QString& rawData, map<QString, AgentDetails>& agentDetails,
                                    std::map<QString, QString>* agentInfo)
{
    //std::cout<<rawData.toStdString()<<std::endl;
    QByteArray byte_array(rawData.toStdString().data());
    QJsonParseError json_error;
    QJsonDocument parse_doucment = QJsonDocument::fromJson(byte_array, &json_error);
    if(json_error.error != QJsonParseError::NoError)
        return false;
    if(!parse_doucment.isObject())
        return false;
    //agentDetails.clear();
    QJsonObject rootObj = parse_doucment.object();
    if (!rootObj.contains("csArray"))
        return false;
    QJsonArray subArray = rootObj.value("csArray").toArray();

    for(int i = 0; i < subArray.size(); i++)
    {
        if (!subArray.at(i).isObject())
            return false;

        AgentDetails details;
        details.IsAtService = false;
        QJsonObject jsonObj = subArray.at(i).toObject();
        if (jsonObj.size() == 0)
            continue;

        if (!jsonObj.contains("csAddr"))
            return false;
        details.CsAddr = jsonObj.value("csAddr").toString().toLower();
        //std::cout << i <<" value is:" << jsonObj.value("csAddr").toString().toStdString()<<std::endl;

        if (!jsonObj.contains("payMethod"))
            return false;
        details.PaymentMeth = jsonObj.value("payMethod").toString();
        //std::cout << i <<" value is:" << jsonObj.value("payMethod").toString().toStdString()<<std::endl;

        if (details.PaymentMeth.compare("0") == 0)
        {
            details.FlowAmount = QString("N/A");
            details.FlowUsed = QString("N/A");

            if (!jsonObj.contains("start"))
                return false;
            details.TStart = jsonObj.value("start").toString();
            //std::cout << i <<" value is:" << jsonObj.value("start").toString().toStdString()<<std::endl;
            if (!jsonObj.contains("end"))
                return false;
            details.TEnd = jsonObj.value("end").toString();
            //std::cout << i <<" value is:" << jsonObj.value("end").toString().toStdString()<<std::endl;
        }
        else
        {
            if (!jsonObj.contains("flow"))
                return false;
            details.FlowAmount = jsonObj.value("flow").toString();
            //std::cout << i <<" value is:" << jsonObj.value("flow").toString().toStdString()<<std::endl;
            if (!jsonObj.contains("usedFlow"))
                return false;
            details.FlowUsed = jsonObj.value("usedFlow").toString();
            //std::cout << i <<" value is:" << jsonObj.value("usedFlow").toString().toStdString()<<std::endl;

            details.TStart = QString("N/A");
            details.TEnd = QString("N/A");
        }
        if (jsonObj.contains("price"))
            details.Price = jsonObj.value("price").toString();

        details.AgentNodeID = jsonObj.value("nodeID").toString();
        if (jsonObj.contains("ipAddress"))
        {
            if (agentInfo != nullptr)
                (*agentInfo)[jsonObj.value("ipAddress").toString()] = details.CsAddr;
            details.IPAddress = jsonObj.value("ipAddress").toString();
        }
        details.IsAtService = IsAtService(details.PaymentMeth, details.TStart, details.TEnd, details.FlowAmount, details.FlowUsed);

        if (!details.CsAddr.isEmpty())
            agentDetails[details.CsAddr] = details;
    }

    return true;
}

QString AgentDetailsFunc::IsAtService(QString paymentMeth, QString tStart, QString tEnd, QString flowAmount, QString flowUsed)
{
    if (paymentMeth.compare(QString("0")) != 0)
    {
        if (flowAmount.toLongLong() - flowUsed.toLongLong() > 0)
            return QString("Yes");
        else
            return QString("No");
    }
    else
    {
        QDateTime currentDateTime = QDateTime::currentDateTime();
        QDateTime startDateTime = QDateTime::fromString(tStart, "yyyy-MM-dd hh:mm:ss");
        QDateTime endDateTime = QDateTime::fromString(tEnd, "yyyy-MM-dd hh:mm:ss");
        if (currentDateTime >= startDateTime && currentDateTime <= endDateTime)
            return QString("Yes");
        else
            return QString("No");
    }
}

bool AgentDetailsFunc::CheckFlow(AgentDetails currAgent, qint64 fileSize)
{
    if (currAgent.IsAtService.compare("No") == 0)
        return false;
    if (currAgent.PaymentMeth.compare("0") == 0)
        return true;
    if (currAgent.FlowAmount.toLongLong() - currAgent.FlowUsed.toLongLong() - fileSize < 0)
        return false;
    return true;
}

// {"AgentList":[{"AgentIP":"127.0.0.1","AgentAddr":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","PaymentMethod":"0","Price":"10/100"},
// {"AgentIP":"118.89.165.39","AgentAddr":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58","PaymentMethod":"1","Price":"10"},
// {"AgentIP":"212.129.144.237","AgentAddr":"0xf5def3415b977db0d03b044c9cb4f17fdf4c51fa","PaymentMethod":"2","Price":"100"}]}
bool AgentDetailsFunc::ParseAgentInfo(const QString& rawData, map<QString, AgentDetails>& agentDetails,
                                      map<QString, QString>& agentInfo)
{
    //std::cout<<rawData.toStdString()<<std::endl;
    QByteArray byte_array(rawData.toStdString().data());
    QJsonParseError json_error;
    QJsonDocument parse_doucment = QJsonDocument::fromJson(byte_array, &json_error);
    if(json_error.error != QJsonParseError::NoError)
        return false;
    if(!parse_doucment.isObject())
        return false;
    agentInfo.clear();
    QJsonObject rootObj = parse_doucment.object();
    if (!rootObj.contains("AgentList"))
        return false;
    QJsonArray subArray = rootObj.value("AgentList").toArray();

    for(int i = 0; i < subArray.size(); i++)
    {
        if (!subArray.at(i).isObject())
            return false;

        AgentInfo info;
        AgentDetails details;
        details.IsAtService = QString("No");
        QJsonObject jsonObj = subArray.at(i).toObject();
        if (jsonObj.size() == 0)
            continue;

        if (!jsonObj.contains("AgentAddr"))
            return false;
        details.CsAddr = jsonObj.value("AgentAddr").toString().toLower();
        //std::cout << i <<" value is:" << jsonObj.value("csAddr").toString().toStdString()<<std::endl;

        if (!jsonObj.contains("AgentIP"))
            return false;
        details.IPAddress = jsonObj.value("AgentIP").toString();

        if (!jsonObj.contains("PaymentMethod"))
            return false;
        details.PaymentMeth = jsonObj.value("PaymentMethod").toString();
        //std::cout << i <<" value is:" << jsonObj.value("payMethod").toString().toStdString()<<std::endl;

        details.FlowAmount = QString("N/A");
        details.FlowUsed = QString("N/A");
        details.TStart = QString("N/A");
        details.TEnd = QString("N/A");

        if (jsonObj.contains("Price"))
            details.Price = jsonObj.value("Price").toString();

        if (!details.CsAddr.isEmpty())
        {
            agentDetails[details.CsAddr] = details;
            agentInfo[details.IPAddress] = details.CsAddr;
        }
    }

    return true;
}
