#ifndef DATADIRDLG_H
#define DATADIRDLG_H

#include <QDialog>
#include "strings.h"

namespace Ui {
class Dialog;
}

class DataDirDlg : public QDialog
{
    Q_OBJECT

public:
    explicit DataDirDlg(std::string* dataDir, bool en=true, QWidget *parent = nullptr, bool dirFlag=true, bool download=false);

    ~DataDirDlg();

    void SetText(QString text);

    void SetSelectBtn();

private slots:
    void on_selectDirBtn_clicked();

    void on_confirmBtn_clicked();

private:
    Ui::Dialog *ui;

    bool m_IsDirFlag;
    bool m_IsDownloadFlag;
    std::map<Ui::GUIStrings, std::string> m_StringsTable;
    std::map<Ui::GUIStrings, std::string> m_UIHints;
    std::string* m_DataDir;

private:
    void closeEvent(QCloseEvent*);
};

#endif // DATADIRDLG_H
