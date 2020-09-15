#ifndef CONFIG_H
#define CONFIG_H

#include <QSettings>
#include <QVariant>

namespace Ui {

const QString WalletSettings = QString("walletSettings");
const QString DataDir = QString("dataDir");

const QString AgentSettings = QString("agentSettings");
const QString AgentList = QString("agentList");
const QString DefaultAgent = QString("defaultAgent");

const QString AccountSettings = QString("accountSettings");
const QString DefaultAccount = QString("defaultAccount");

}

class Config
{
public:
    Config(QString qstrfilename = "");
    virtual ~Config(void);
    void Set(QString, QString, QVariant);
    QVariant Get(QString, QString);

private:
    QString m_qstrFileName;
    QSettings *m_psetting;
};

#endif // CONFIG_H
