#include <QFileDialog>
#include "dataDirDlg.h"
#include "ui_dataDirDlg.h"

DataDirDlg::DataDirDlg(std::string* dataDir, bool en, QWidget *parent, bool dirFlag, bool download) :
    QDialog(parent),
    ui(new Ui::Dialog),
    m_IsDirFlag(dirFlag),
    m_IsDownloadFlag(download),
    m_DataDir(dataDir)
{
    ui->setupUi(this);
    if (dirFlag)
        ui->dataDirLineEdit->setText(QString::fromStdString(*dataDir));

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

DataDirDlg::~DataDirDlg()
{
    m_DataDir = nullptr;
    delete ui;
}

void DataDirDlg::closeEvent(QCloseEvent*)
{
    if (m_IsDirFlag && !m_IsDownloadFlag)
        this->accept();
    //e->ignore();
}

void DataDirDlg::SetText(QString text)
{
    ui->hintLabel->setText(text);
}

void DataDirDlg::SetSelectBtn()
{
    ui->selectDirBtn->setText(QString::fromStdString(m_StringsTable[Ui::selectUpFileBtn]));
}

void DataDirDlg::on_selectDirBtn_clicked()
{
    QFileDialog *fileDialog = new QFileDialog();
    fileDialog->setWindowTitle(QString::fromStdString(m_StringsTable[Ui::dataDirDlgTitle]));
    fileDialog->setDirectory(".");
    if (m_IsDirFlag)
        fileDialog->setFileMode(QFileDialog::DirectoryOnly);
    else
        fileDialog->setFileMode(QFileDialog::ExistingFile);
    fileDialog->setViewMode(QFileDialog::Detail);

    QStringList dir;
    if(fileDialog->exec())
    {
        dir = fileDialog->selectedFiles();
    }
    for(auto dataDir:dir)
    {
        ui->dataDirLineEdit->setText(dataDir);
        break;
    }
}

void DataDirDlg::on_confirmBtn_clicked()
{
    if (ui->dataDirLineEdit->text().isEmpty())
    {
        ui->dataDirLineEdit->setPlaceholderText(QString::fromStdString(m_UIHints[Ui::uploadSelectHint]));
        return;
    }
    *m_DataDir = ui->dataDirLineEdit->text().toStdString();
    this->accept();
}
