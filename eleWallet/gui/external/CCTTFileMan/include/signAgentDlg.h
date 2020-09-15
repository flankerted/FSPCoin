#ifndef SIGNAGENTDLG_H
#define SIGNAGENTDLG_H

#include <QDialog>
#include <QButtonGroup>
#include <QMessageBox>
#include "strings.h"

namespace Ui {
class SignAgentDialog;
}

class SignAgentDlg : public QDialog
{
    Q_OBJECT

public:
    explicit SignAgentDlg(QString* start, QString* stop, QString* flow, QWidget *parent = nullptr);

    ~SignAgentDlg();


private slots:

    void on_cancelBtn_clicked();

    void on_timeRadioBtn_toggled(bool checked);

    void on_flowRadioBtn_toggled(bool checked);

    void on_confirmBtn_clicked();

private:
    Ui::SignAgentDialog *ui;

    QButtonGroup* m_PaymentSel;
    QMessageBox* m_MsgBox;

    QString* m_StartTime;
    QString* m_StopTime;
    QString* m_Flow;

private:

    int MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon=QMessageBox::Warning,
                       QMessageBox::StandardButton btn=QMessageBox::Ok, bool modal=true, bool exec=true);
};

#endif // SIGNAGENTDLG_H
