#ifndef NEWMAILATTACHDLG_H
#define NEWMAILATTACHDLG_H

#include <QDialog>
#include <QMessageBox>
#include "fileMan.h"
#include "strings.h"

namespace Ui {
class NewMailAttachDialog;
}

class NewMailAttachDlg : public QDialog
{
    Q_OBJECT

public:
    explicit NewMailAttachDlg(const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                              const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList, QString* fileName,
                              qint64* fileObjID, qint64* fileOffset, qint64* fileSize, bool en=true, QWidget *parent = nullptr);

    ~NewMailAttachDlg();

    void SetText(QString text);

    void SetSelectBtn();

private slots:

    void on_selectBtn_clicked();

    void on_cancelBtn_clicked();

    void on_spaceComboBox_currentTextChanged(const QString &arg1);

private:
    Ui::NewMailAttachDialog *ui;
    QMessageBox* m_MsgBox;

    const int mailFileTableInitW0 = 380;
    const int mailFileTableInitW1 = 90;
    const std::string mailFileTableLabels = "file name" + Ui::spec.toStdString() + "file size";

    const std::map<qint64, std::pair<qint64, QString> >* m_ObjInfos;
    const std::vector<std::pair<QList<FileInfo>, qint64> >* m_VecFileList;
    QString* m_FileName;
    qint64* m_FileObjID;
    qint64* m_FileOffset;
    qint64* m_FileSize;

    std::map<Ui::GUIStrings, std::string> m_StringsTable;
    std::map<Ui::GUIStrings, std::string> m_UIHints;
    std::map<Ui::GUIStrings, std::string> m_SysMsgs;

private:
    void SetLang(bool en);

    void ShowFileTable(QString curLabel="");

    int MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
                       QMessageBox::StandardButton btn, bool modal=true, bool exec=false);

    void ShowOpErrorMsgBox(QString text = "", bool modal=true, bool exec=false);
};

#endif // NEWMAILATTACHDLG_H
