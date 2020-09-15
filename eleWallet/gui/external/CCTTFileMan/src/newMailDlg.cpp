#include <iostream>
#include <QDateTime>
#include "newMailDlg.h"
#include "newMailAttachDlg.h"
#include "sharingFileDlg.h"
#include "mailsTab.h"
#include "ui_newMailDlg.h"

NewMailDlg::NewMailDlg(QString* mailContent, QString sender, const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                       const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList, QString accPwd, QString agNodeID,
                       EleWalletBackRPCCli* backRPC, bool en, QWidget *parent) :
    QDialog(parent),
    ui(new Ui::NewMailDialog),
    m_MailContent(mailContent),
    m_AccountPwd(accPwd),
    m_AgentNodeID(agNodeID),
    m_Sender(sender),
    m_ObjInfos(objInfos),
    m_VecFileList(fileList),
    m_BackRPC(backRPC)
{
    ui->setupUi(this);

    m_SharingCode = "";

    if (en)
    {
        m_StringsTable = Ui::stringsTableEN;
        m_UIHints = Ui::UIHintsEN;
    }
    else
    {
        m_StringsTable = Ui::stringsTableCN;
        m_UIHints = Ui::UIHintsCN;
    }
}

NewMailDlg::~NewMailDlg()
{
    m_MailContent = nullptr;
    delete ui;
}

void NewMailDlg::on_attachmentBtn_clicked()
{
    QString* fileName = new(QString);
    qint64* fileObjID = new(qint64);
    qint64* fileOffset = new(qint64);
    qint64* fileSize = new(qint64);
    QString* mailToOrSharingCode = new(QString);
    *mailToOrSharingCode = ui->mailToLineEdit->text();
    NewMailAttachDlg dlg(m_ObjInfos, m_VecFileList, fileName, fileObjID, fileOffset, fileSize);
    if (dlg.exec() == QDialog::Accepted) {
        SharingFileDlg dlg(false, m_AccountPwd, m_AgentNodeID, m_UIHints, m_BackRPC, true, mailToOrSharingCode);

        dlg.SetFileInfo(*fileName, *fileObjID, *fileOffset, *fileSize);

        if (dlg.exec() == QDialog::Accepted)
        {
            m_SharingCode = *mailToOrSharingCode;
            //m_SharingCode = m_SharingCode.replace(QRegExp("\""), "\\\"");
            ui->attachmentLineEdit->setText(*fileName);
            ui->attachmentBtn->setEnabled(false);
        }
    }

    delete fileName;
    delete fileObjID;
    delete fileOffset;
    delete fileSize;
    delete mailToOrSharingCode;
}

void NewMailDlg::on_sendMailBtn_clicked()
{
    MailDetails mail;
    QDateTime time = QDateTime::currentDateTime();
    QString timeStr = time.toUTC().toString("yyyy-MM-dd hh:mm:ss");
    mail.SetValues(ui->mailTitleLineEdit->text(), m_Sender, ui->mailToLineEdit->text(), ui->mailContentTextEdit->toPlainText(),
                   timeStr, m_SharingCode);
    *m_MailContent = mail.EncodeQJson();

    //std::cout<<m_MailContent->toStdString()<<std::endl;////////////////////
    this->accept();
}

void NewMailDlg::on_cancelBtn_clicked()
{
    this->close();
}

void NewMailDlg::on_mailTitleLineEdit_textChanged(const QString &arg1)
{
    ui->sendMailBtn->setEnabled(!arg1.isEmpty() && !ui->mailToLineEdit->text().isEmpty());
}

void NewMailDlg::on_mailToLineEdit_textChanged(const QString &arg1)
{
    ui->attachmentBtn->setEnabled(!arg1.isEmpty());
}

void NewMailDlg::on_clearAttachmentBtn_clicked()
{
    m_SharingCode = "";
    ui->attachmentLineEdit->setText(m_SharingCode);
    ui->attachmentBtn->setEnabled(true);
}
