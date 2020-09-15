#include <iostream>
#include <QApplication>
#include <QDesktopWidget>
#include "signAgentDlg.h"
#include "ui_signAgentDlg.h"
#include "utils.h"

SignAgentDlg::SignAgentDlg(QString *start, QString *stop, QString *flow, QWidget *parent) :
    QDialog(parent),
    ui(new Ui::SignAgentDialog),
    m_MsgBox(nullptr)
{
    ui->setupUi(this);
    this->setWindowFlags(Qt::SubWindow);

    this->move((QApplication::desktop()->width() - this->width())/2,
               (QApplication::desktop()->height() - this->height())/2);

    m_PaymentSel = new QButtonGroup(this);
    m_PaymentSel->addButton(ui->timeRadioBtn);
    m_PaymentSel->addButton(ui->flowRadioBtn);

    QRegExp regx("[0-9]+$");
    QValidator *validator = new QRegExpValidator(regx, ui->flowLineEdit);
    ui->flowLineEdit->setValidator(validator);

    m_StartTime = start;
    m_StopTime = stop;
    m_Flow = flow;
}

SignAgentDlg::~SignAgentDlg()
{

}

int SignAgentDlg::MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
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

void SignAgentDlg::on_cancelBtn_clicked()
{
    this->close();
}

void SignAgentDlg::on_timeRadioBtn_toggled(bool checked)
{
    if (checked)
    {
        ui->startDateTimeEdit->setEnabled(checked);
        ui->stopDateTimeEdit->setEnabled(checked);
        ui->flowLineEdit->setEnabled(!checked);
        ui->flowLineEdit->clear();
    }
}

void SignAgentDlg::on_flowRadioBtn_toggled(bool checked)
{
    if (checked)
    {
        ui->startDateTimeEdit->setEnabled(!checked);
        ui->stopDateTimeEdit->setEnabled(!checked);
        ui->flowLineEdit->setEnabled(checked);
    }
}

void SignAgentDlg::on_confirmBtn_clicked()
{
    if (ui->timeRadioBtn->isChecked())
    {
        if (ui->startDateTimeEdit->dateTime() >= ui->stopDateTimeEdit->dateTime())
        {
            MessageBoxShow(this->windowTitle(), QString("The service start time and stop time is not correct !"));
            return;
        }

        QString yearStr1 = QString::number(ui->startDateTimeEdit->date().year());
        QString monthStr1 = QString::number(ui->startDateTimeEdit->date().month());
        QString dayStr1 = QString::number(ui->startDateTimeEdit->date().day());
        Utils::CalculateTime(yearStr1, monthStr1, dayStr1, ui->startDateTimeEdit->time().toString(), true,
                      m_StartTime, m_StopTime);

        QString yearStr2 = QString::number(ui->stopDateTimeEdit->date().year());
        QString monthStr2 = QString::number(ui->stopDateTimeEdit->date().month());
        QString dayStr2 = QString::number(ui->stopDateTimeEdit->date().day());
        Utils::CalculateTime(yearStr2, monthStr2, dayStr2, ui->stopDateTimeEdit->time().toString(), false,
                      m_StartTime, m_StopTime);

        *m_Flow = QString("0");
        this->accept();
    }
    else
    {
        if (ui->flowLineEdit->text().isEmpty())
        {
            MessageBoxShow(this->windowTitle(), QString("The parameter of flow amount to use can not be empty !"));
            return;
        }
        *m_Flow = ui->flowLineEdit->text();

        *m_StartTime = QString("0");
        *m_StopTime = QString("0");
        this->accept();
    }
}
