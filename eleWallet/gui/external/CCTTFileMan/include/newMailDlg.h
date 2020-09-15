#ifndef NEWMAILDLG_H
#define NEWMAILDLG_H

#include <QDialog>
#include "fileMan.h"
#include "eleWalletBackRPC.h"
#include "strings.h"

/*"{\"Title\":\"title_1\",\"Sender\":\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",
\"Content\":\"1...1\",\"TimeStamp\":\"2019-01-01 00:05:00\",\"FileSharing\":\"\",\"Signature\":\"\"}/{\"Title\":\"title_2\",\"Sender\":
\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"2...2\",
\"TimeStamp\":\"2019-01-02 00:00:00\",\"FileSharing\":\"\",\"Signature\":\"...\"}/{\"Title\":\"title_3\",\"Sender\":
\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\",\"Receiver\":\"0x790113A1E87A6e845Be30358827FEE65E0BE8A58\",\"Content\":\"3...3\",
\"TimeStamp\":\"2019-01-03 00:05:00\",\"FileSharing\":\"{\\\"FileName\\\":\\\"Qt5Networkd.dll\\\",\\\"FileOffset\\\":\\\"16352632\\\",
\\\"FileSize\\\":\\\"94562078\\\",\\\"ObjectID\\\":1,\\\"Price\\\":\\\"0.12\\\",\\\"Receiver\\\":\\\"0x5f663f10f12503cb126eb5789a9b5381f594a0eb\\\",
\\\"SharerCSNodeID\\\":\\\"\\\",\\\"Signature\\\":\\\"0x51eb9f01c56e9be964a2885187e97135336667c1ae511f3961550b2ee4a486f91b990700ffbfb524094b32562fdae6dde6e302f35fd8f0aaf63697ec49e439dc00\\\",
\\\"TimeStart\\\":\\\"2019-01-01 00:00:00\\\",\\\"TimeStop\\\":\\\"2019-01-01 00:00:00\\\"}\",\"Signature\":\"...\"}"
*/

namespace Ui {
class NewMailDialog;
}

class NewMailDlg : public QDialog
{
    Q_OBJECT

public:
    explicit NewMailDlg(QString* mailContent, QString sender, const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                        const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList, QString accPwd, QString agNodeID,
                        EleWalletBackRPCCli* backRPC, bool en=true, QWidget *parent = nullptr);

    ~NewMailDlg();

    void SetText(QString text);

    void SetSelectBtn();

private slots:

    void on_attachmentBtn_clicked();

    void on_sendMailBtn_clicked();

    void on_cancelBtn_clicked();

    void on_mailTitleLineEdit_textChanged(const QString &arg1);

    void on_mailToLineEdit_textChanged(const QString &arg1);

    void on_clearAttachmentBtn_clicked();

private:
    Ui::NewMailDialog *ui;

    QString* m_MailContent;
    QString m_AccountPwd;
    QString m_AgentNodeID;
    QString m_SharingCode;
    QString m_Sender;
    const std::map<qint64, std::pair<qint64, QString> >* m_ObjInfos;
    const std::vector<std::pair<QList<FileInfo>, qint64> >* m_VecFileList;
    EleWalletBackRPCCli* m_BackRPC;

    std::map<Ui::GUIStrings, std::string> m_StringsTable;
    std::map<Ui::GUIStrings, std::string> m_UIHints;

private:

};

#endif // NEWMAILDLG_H
