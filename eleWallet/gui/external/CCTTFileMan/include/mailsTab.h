#ifndef MAILSTAB_H
#define MAILSTAB_H

#include <vector>
#include <QString>
#include <QJsonObject>
#include <QJsonDocument>

// {"Title":"title_1","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"","Signature":"..."}/{"Title":"title_2","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"...","Signature":"..."}/{"Title":"title_3","Sender":"0x5f663f10f12503cb126eb5789a9b5381f594a0eb","Receiver":"0x790113A1E87A6e845Be30358827FEE65E0BE8A58",
// "Content":"...","TimeStamp":"2019-01-01 00:00:00","FileSharing":"...","Signature":"..."}
typedef struct _MailDetails
{
    QString mailTitle;
    QString mailSender;
    QString mailReceiver;
    QString mailContent;
    QString mailTimeStamp;
    QString mailFileSharingCode;

    QString mailSignature;

    void SetValues(QString title, QString sender, QString receiver, QString content, QString timeStamp,
                   QString fileSharingCode, QString sig="0")
    {
        mailTitle = title;
        mailSender= sender;
        mailReceiver = receiver;
        mailContent = content;
        mailTimeStamp = timeStamp;
        mailFileSharingCode = fileSharingCode;
        mailSignature = sig;
    }

    void SetSignature(QString sig)
    {
        mailSignature = sig;
    }

    void GetValues(QString& title, QString& sender, QString& receiver, QString& content, QString& timeStamp,
                   QString& fileSharingCode, QString& sig)
    {
        title = mailTitle;
        sender = mailSender;
        receiver = mailReceiver;
        content = mailContent;
        timeStamp = mailTimeStamp;
        fileSharingCode = mailFileSharingCode;

        sig = mailSignature;
    }

    QString EncodeQJson()
    {
        QJsonObject json;
        json.insert("Title", mailTitle);
        json.insert("Sender", mailSender);
        json.insert("Receiver", mailReceiver);
        json.insert("Content", mailContent);
        json.insert("TimeStamp", mailTimeStamp);
        json.insert("FileSharing", mailFileSharingCode);
        json.insert("Signature", mailSignature);

        QJsonDocument jsonDoc(json);
        return jsonDoc.toJson(QJsonDocument::Compact);
    }

    void DecodeQJson(QString jsonStr)
    {
        QJsonParseError err;
        QJsonDocument jsonDoc = QJsonDocument::fromJson(jsonStr.toUtf8(), &err);
        if(err.error == QJsonParseError::NoError && !jsonDoc.isNull())
        {
            QJsonObject json = jsonDoc.object();
            mailTitle = json["Title"].toString();
            mailSender= json["Sender"].toString();
            mailReceiver = json["Receiver"].toString();
            mailContent = json["Content"].toString();
            mailTimeStamp = json["TimeStamp"].toString();
            mailFileSharingCode = json["FileSharing"].toString();
            mailSignature = json["Signature"].toString();
        }
        else
        {
            mailSignature = "0";
        }
    }

}MailDetails;

#endif // MAILSTAB_H
