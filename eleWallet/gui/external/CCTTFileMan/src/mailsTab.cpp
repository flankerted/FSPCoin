#include <QStandardItemModel>
#include "mailsTab.h"
#include "mainwindow.h"

using namespace std;

bool MainWindow::ParseMailList(QString& mailListStr)
{
    const QString noObjId("mails not found");
    if (mailListStr.compare(noObjId) == 0)
        return true;

    QStringList list = mailListStr.split(Ui::spec);
    m_MailList.clear();
    for(int i = 0; i < list.length(); ++i)
    {
        MailDetails mail;
        mail.DecodeQJson(list.at(i));
        m_MailList.push_back(mail);
    }

    return true;
}
