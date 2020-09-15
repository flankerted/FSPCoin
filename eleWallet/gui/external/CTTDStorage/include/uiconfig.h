#ifndef UICONFIG_H
#define UICONFIG_H

#include <QSettings>
#include <QVariant>

namespace CTTUi {

const QString WalletSettings = QString("walletSettings");
const QString DataDir = QString("dataDir");
const QString Lang = QString("lang");

const QString AgentSettings = QString("agentSettings");
const QString AgentList = QString("agentList");
const QString DefaultAgent = QString("defaultAgent");

const QString AccountSettings = QString("accountSettings");
const QString DefaultAccount = QString("defaultAccount");

const QString AutoConnectSettings = QString("autoConnectSettings");
const QString AgentServerIP = QString("agentServerIP");

const QString DiskMetaDataBackup = QString("diskMetaDataBackup");
const QString IncompletedMetaData = QString("incompletedMetaData");
const QString LastMetaDataCache = QString("lastMetaDataCache");
const QString AllReformatMetaData = QString("allReformatMetaData");

}

class UiConfig : public QObject
{
    Q_OBJECT

public:
    UiConfig(QString qstrfilename = "");
    virtual ~UiConfig(void);
    void Set(QString, QString, QVariant);
    QVariant Get(QString, QString);

private:
    QString m_qstrFileName;
    QSettings *m_psetting;
};

#endif // UICONFIG_H
