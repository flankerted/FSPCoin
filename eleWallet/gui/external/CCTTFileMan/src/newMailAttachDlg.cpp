#include <iostream>
#include <QStandardItemModel>
#include "newMailAttachDlg.h"
#include "sharingFileDlg.h"
#include "utils.h"
#include "ui_newMailAttachDlg.h"

using namespace std;

NewMailAttachDlg::NewMailAttachDlg(const std::map<qint64, std::pair<qint64, QString> >* objInfos,
                                   const std::vector<std::pair<QList<FileInfo>, qint64> >* fileList, QString* fileName,
                                   qint64* fileObjID, qint64* fileOffset, qint64* fileSize, bool en, QWidget* parent) :
    QDialog(parent),
    ui(new Ui::NewMailAttachDialog),
    m_MsgBox(nullptr),
    m_ObjInfos(objInfos),
    m_VecFileList(fileList),
    m_FileName(fileName),
    m_FileObjID(fileObjID),
    m_FileOffset(fileOffset),
    m_FileSize(fileSize)
{
    ui->setupUi(this);

    map<qint64, pair<qint64, QString> >::const_iterator iter;
    QString curlabel = "";
    for (iter = objInfos->begin(); iter != objInfos->end(); iter++)
    {
        QString label = iter->second.second;
        curlabel = label;
        if (iter == objInfos->begin())
        {
            ui->spaceComboBox->addItem(label);
            ui->spaceComboBox->setCurrentText(label);
            //cout<<label.toStdString()<<endl;
        }
        else if (ui->spaceComboBox->findText(label) < 0)
            ui->spaceComboBox->addItem(label);
    }
    ShowFileTable(curlabel);

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

NewMailAttachDlg::~NewMailAttachDlg()
{
    if (m_ObjInfos != nullptr)
        m_ObjInfos = nullptr;
    if (m_VecFileList != nullptr)
        m_VecFileList = nullptr;
    delete ui;
}

void NewMailAttachDlg::SetLang(bool en)
{
    if (en)
    {
        m_StringsTable = Ui::stringsTableEN;
        m_UIHints = Ui::UIHintsEN;
        m_SysMsgs = Ui::SysMessages;
    }
    else
    {
        m_StringsTable = Ui::stringsTableCN;
        m_UIHints = Ui::UIHintsCN;
        m_SysMsgs = Ui::SysMessages;
    }
}

void NewMailAttachDlg::ShowFileTable(QString curLabel)
{
    QString fLabel = curLabel.isEmpty() ? ui->spaceComboBox->currentText() : curLabel;
    QStandardItemModel* model = new QStandardItemModel();
    QStringList labels = QObject::trUtf8(mailFileTableLabels.data()).simplified().split(Ui::spec);
    model->setHorizontalHeaderLabels(labels);
    ui->mailFileTable->setModel(model);
    ui->mailFileTable->setColumnWidth(0, mailFileTableInitW0);
    ui->mailFileTable->setColumnWidth(1, mailFileTableInitW1);
    ui->mailFileTable->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);

    bool updateFlag = false;
    for(size_t i = 0; i < m_VecFileList->size(); i++)
    {
        for(int j = 0; j < (*m_VecFileList)[i].first.length(); j++)
        {
            if ((*m_VecFileList)[i].first.at(j).fileSpaceLabel.compare(fLabel) == 0)
            {
                QString name = (*m_VecFileList)[i].first.at(j).fileName;
                QString size = Utils::AutoSizeString((*m_VecFileList)[i].first.at(j).fileSize.toLongLong());
                QList<QStandardItem*> list;
                list << new QStandardItem(name) << new QStandardItem(size);
                model->insertRow(0, list);
                model->item(0, 1)->setTextAlignment(Qt::AlignHCenter | Qt::AlignVCenter);
                updateFlag = true;
            }
        }
    }

    if (updateFlag)
        ui->mailFileTable->setModel(model);
}

int NewMailAttachDlg::MessageBoxShow(const QString& title, const QString& text, QMessageBox::Icon icon,
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

void NewMailAttachDlg::ShowOpErrorMsgBox(QString text, bool modal, bool exec)
{
    if (text.isEmpty())
        MessageBoxShow(QString::fromStdString(m_SysMsgs[Ui::errorMsg]),
                QString::fromStdString(m_UIHints[Ui::opFailedHint]),
                QMessageBox::Critical, QMessageBox::Ok, modal, exec);
    else
        MessageBoxShow(QString::fromStdString(m_SysMsgs[Ui::errorMsg]), text,
                QMessageBox::Critical, QMessageBox::Ok);
}

void NewMailAttachDlg::on_selectBtn_clicked()
{
    int row = ui->mailFileTable->currentIndex().row();
    if (row < 0)
        return;

    QAbstractItemModel* model = ui->mailFileTable->model();
    QModelIndex index = model->index(row, 0);
    *m_FileName = model->data(index).toString();

    qint64 offset = 0;
    qint64 length = 0;
    QString sharerAddr = "0";
    QString sharerAgentNodeID = "0";
    QString key = "0";
    *m_FileObjID = Utils::GetFileCompleteInfo(m_VecFileList, m_ObjInfos, *m_FileName, ui->spaceComboBox->currentText(), offset,
                                              length, sharerAddr, sharerAgentNodeID, key);

    if (*m_FileObjID == 0)
    {
        ShowOpErrorMsgBox(QString::fromStdString(m_SysMsgs[Ui::errorMsg]));
        return;
    }

    *m_FileOffset = offset;
    *m_FileSize = length;

    this->accept();
}

void NewMailAttachDlg::on_cancelBtn_clicked()
{
    this->close();
}

void NewMailAttachDlg::on_spaceComboBox_currentTextChanged(const QString &arg1)
{
    map<qint64, pair<qint64, QString> >::const_iterator iter;
    for (iter = m_ObjInfos->begin(); iter != m_ObjInfos->end(); iter++)
    {
        if (iter->second.second.compare(arg1) == 0)
            ShowFileTable(arg1);
    }

//    if (m_StringsTable[Ui::sharingSpaceLabel].compare(arg1.toStdString()) == 0)
//        ShowFileTable(arg1);
}
