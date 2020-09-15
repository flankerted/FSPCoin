#include <iostream>
#include <QApplication>
#include <QDesktopWidget>
#include <QClipboard>
#include <QObject>
#include "sharingFileDlg.h"
#include "ui_sharingFileDlg.h"
#include "utils.h"

SharingFileDlg::SharingFileDlg(bool noFileSelected, QString passwd, QString agent, const std::map<Ui::GUIStrings, std::string>& uiHints,
                               EleWalletBackRPCCli* backRPC, bool isAttachment, QString* mailToOrCode, QWidget* parent) :
    QDialog(parent),
    ui(new Ui::SharingFileDialog),
    m_UiHints(uiHints),
    m_MsgBox(nullptr),
    m_SharerAgentNodeID(agent),
    m_AccountPwd(passwd),
    m_BackRPC(backRPC),
    m_SysMsgs(Ui::SysMessages),
    m_Cmds(Ui::RPCCommands),
    m_IsAttached(isAttachment),
    m_MailSharingCode(mailToOrCode)
{
    ui->setupUi(this);
    this->setWindowFlags(Qt::SubWindow);

    this->move((QApplication::desktop()->width() - this->width())/2,
               (QApplication::desktop()->height() - this->height())/2);

    QRegExp regx("[0-9]+$");
    QValidator *validator = new QRegExpValidator(regx, ui->priceLineEdit);
    ui->priceLineEdit->setValidator(validator);

    ui->tabWidget->setTabEnabled(0, !noFileSelected);
    ui->tabWidget->setTabEnabled(1, !isAttachment);

    if (isAttachment)
    {
        ui->shareBtn->setText("Done");
        ui->doneBtn->setText("Cancel");
        ui->shareAddrLineEdit->setText(*mailToOrCode);
        ui->shareAddrLineEdit->setEnabled(false);
    }
}

SharingFileDlg::~SharingFileDlg()
{

}

void SharingFileDlg::SetFileInfo(QString fileName, qint64 fileObjID, qint64 fileOffset, qint64 fileSize)
{
    m_FileName = fileName;
    m_FileObjID = fileObjID;
    m_FileOffset = fileOffset;
    m_FileSize = fileSize;

    ui->fileSlectedEdit->setText(m_FileName);
}

int SharingFileDlg::MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
                                   QMessageBox::StandardButton btn, bool modal, bool exec)
{
    if (m_MsgBox != nullptr)
    {
        delete m_MsgBox;
        m_MsgBox = nullptr;
    }
    m_MsgBox = new QMessageBox(icon, title, text, btn);
    m_MsgBox->setModal(modal);
    if (btn == QMessageBox::NoButton)
        m_MsgBox->setStandardButtons(QMessageBox::NoButton);
    if (!exec)
    {
        m_MsgBox->show();
        return 0;
    }
    else
        return m_MsgBox->exec();
}

QString SharingFileDlg::GenerateSharingCode()
{
    std::string ret = m_BackRPC->SendOperationCmd(m_Cmds[Ui::getSharingCodeCmd],
            std::vector<QString>{QString::number(m_FileObjID), m_FileName,
                QString::number(m_FileOffset), QString::number(m_FileSize),
                m_Receiver, m_StartTime, m_StopTime, QString::number(m_Price),
                m_SharerAgentNodeID, m_AccountPwd});

    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0)
        return QString("0");

    return QString::fromStdString(ret);
}

void SharingFileDlg::SetSharingCode(const QString& code)
{
    ui->fileSharingCodeTextEdit->setText(code);
    emit on_getSharingInfoBtn_clicked();
}

void SharingFileDlg::on_doneBtn_clicked()
{
    this->close();
}

void SharingFileDlg::on_cancelBtn_clicked()
{
    this->close();
}

void SharingFileDlg::on_shareBtn_clicked()
{
    if (ui->priceLineEdit->text().isEmpty())
    {
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::sharingPriceEmptyHint]));
        return;
    }
    if (ui->shareAddrLineEdit->text().isEmpty())
    {
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::sharingReicvEmptyHint]));
        return;
    }
    m_Receiver = ui->shareAddrLineEdit->text();
    m_Price = ui->priceLineEdit->text().toLongLong();

    QString yearStr1 = QString::number(ui->startDateTimeEdit->date().year());
    QString monthStr1 = QString::number(ui->startDateTimeEdit->date().month());
    QString dayStr1 = QString::number(ui->startDateTimeEdit->date().day());
    Utils::CalculateTime(yearStr1, monthStr1, dayStr1, ui->startDateTimeEdit->time().toString(), true,
                  &m_StartTime, &m_StopTime);

    QString yearStr2 = QString::number(ui->stopDateTimeEdit->date().year());
    QString monthStr2 = QString::number(ui->stopDateTimeEdit->date().month());
    QString dayStr2 = QString::number(ui->stopDateTimeEdit->date().day());
    Utils::CalculateTime(yearStr2, monthStr2, dayStr2, ui->stopDateTimeEdit->time().toString(), false,
                  &m_StartTime, &m_StopTime);

    QString sharingCode = GenerateSharingCode();
    if (sharingCode.compare("0") == 0)
    {
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::genSharingCodeFailedHint]));
        return;
    }

    ui->shareLinkTextEdit->setPlainText(sharingCode);
    ui->copyBtn->setEnabled(true);

    if (m_IsAttached)
    {
        *m_MailSharingCode = sharingCode;
        this->accept();
    }
}

void SharingFileDlg::on_copyBtn_clicked()
{
    QClipboard* clipboard = QApplication::clipboard();
    clipboard->setText(ui->shareLinkTextEdit->toPlainText());
}

void SharingFileDlg::on_getSharingInfoBtn_clicked()
{
    QString content = ui->fileSharingCodeTextEdit->toPlainText();
    FileSharingInfo fileJson;
    fileJson.DecodeQJson(content);
    fileJson.GetValues(m_FileObjID, m_FileName, m_FileOffset, m_FileSize, m_Receiver, m_StartTime, m_StopTime, m_Price,
                       m_SharerAgentNodeID);
    m_SharingCode = content;

    std::string sharer = m_BackRPC->SendOperationCmd(m_Cmds[Ui::checkSignatureCmd],
            std::vector<QString>{content});
    if (sharer.compare("") == 0 || sharer.compare(m_SysMsgs[Ui::errorMsg]) == 0)
    {
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::checkSignFailedHint]));
        return;
    }

    ui->fileSharingNameLineEdit->setText(m_FileName);
    ui->getStartDateLineEdit->setText(m_StartTime);
    ui->getStopDateLineEdit->setText(m_StopTime);
    ui->priceShowLineEdit->setText(QObject::tr("%1").arg(m_Price));
    ui->ownerAddrLineEdit->setText(QString::fromStdString(sharer));

    ui->paySharingBtn->setEnabled(true);
}

void SharingFileDlg::on_paySharingBtn_clicked()
{
    std::string ret = m_BackRPC->SendOperationCmd(m_Cmds[Ui::payForSharedFileCmd],
            std::vector<QString>{m_SharingCode, m_AccountPwd});
    if (ret.compare(m_SysMsgs[Ui::errorMsg]) == 0)
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::payForSharingFailedHint]));
    else
        MessageBoxShow(this->windowTitle(), QString::fromStdString(m_UiHints[Ui::payForSharingSuccHint]),
                       QMessageBox::Information);
}
