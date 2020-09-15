#include "mainwindow.h"
#include <QStyleFactory>
#include <QApplication>
#include <stdlib.h>

int main(int argc, char *argv[])
{
    QApplication a(argc, argv);
    QWidget *parent = nullptr;
    quint16 port = 10309;
    if (argc > 1)
    {
        port = quint16(atoi(argv[1]));
    }
    MainWindow w(parent, port);
    QApplication::setStyle(QStyleFactory::create("Fusion"));

    if (w.HasConnection)
    {
        w.show();
        w.move((QApplication::desktop()->width() - w.width())/2,
               (QApplication::desktop()->height() - w.height())/2);
    }
    else
    {
        w.MessageBoxShow(QString("CCTTFileMan"),
                         QString("Can not connect to the backend of eleWallet, don't run this program indepandently ."),
                         QMessageBox::Critical, QMessageBox::Ok, true, true);
        return 0;
    }

    return a.exec();
}
