#ifndef STRINGS_H
#define STRINGS_H

#include <map>
#include <vector>
#include <QObject>
#include <QString>

#define Bytes 1
#define KB_Bytes 1024
#define MB_Bytes (1024 * KB_Bytes)
#define GB_Bytes (1024 * MB_Bytes)

class GUIString : public QObject
{
    Q_OBJECT

public:
    static QString Spec;

    enum MsgDlgType {
        HintDlg = 0,
        SelectionDlg,
        WaitingDlg
    };
    Q_ENUMS(MsgDlgType)

    enum RpcCmds {
        SetDataDirCmd = 0,
        ShowAccountsCmd,
        GetAgentsInfoCmd,
        ExitExternGUICmd,
        NewAccountCmd,
        RemoteIPCmd,
        LoginAccountCmd,
        GetObjInfoCmd,
        BlizReadCmd,
        BlizReadFileCmd,
        BlizWriteCmd,
        BlizWriteFileCmd,
        CreateObjCmd,
        AddObjStorageCmd,
        GetCurAgentAddrCmd,
        GetSignedAgentInfoCmd,
        GetAccTBalanceCmd,
        GetAccBalanceCmd,
        GetTransactionCmd,
        SignAgentCmd,
        CancelAgentCmd,
        WaitTXCompleteCmd,
        GetSignatureCmd,
        CheckSignatureCmd,
        GetSharingCodeCmd,
        PayForSharedFileCmd,
        SendTransactionCmd,
        GetMailsCmd,
        SendMailCmd,
    };
    Q_ENUMS(RpcCmds)

    enum SysMessages {
        OkMsg,
        ErrorMsg,
        OpRspMsg,
        RateMsg,
        OpTimedOutMsg,
        NoAnyAccountMsg,
        FNameduplicatedMsg,
        ParseAgentsFailedMsg,
        SharingSpaceLabel,
        NeedMoreFlow,
    };
    Q_ENUMS(SysMessages)

    enum UIHints {
        CriticalErrorHint,
        IncorrecctPasswdHint,
        UnavailableHint,
        WaitingForCompleteHint,
        WaitingTXCompleteHint,
        TXTimedoutHint,
        TXIDHint,
        CreateAccoutFailed,
        InvalidAgentServDurationHint,
        InvalidAgentServFlowHint,
        AgentApplyNotCurrentHint,
        AgentApplyStartedHint,
        AgentCancelStartedHint,
        AgentApplyFailedHint,
        AgentCancelFailedHint,
        AgentApplySucceededHint,
        AgentCancelSucceededHint,
        AgentInvalidHint,
        ConnFailedHint,
        GetAccTBalanceErrHint,
        GetAccBalanceErrHint,
        OperationFailedHint,
        TxReturnErrorHint,
        GetAgentDetailsErrHint,
        ParseAgentDetailsErrHint,
        DuplicatedSpaceNameHint,
        CreateSpaceSuccessHint,
        CreateSpaceFailedHint,
        GetObjInfoErrHint,
        ParseObjErrHint,
        ReadMetaDataErrHint,
        IncorrectSpaceMetaHint,
        ParseMetaErrHint,
        NoMetaBakToRecoverHint,
        NeedToBeReformattedHint,
        FileInfoNotFoundHint,
        NeedMoreFlowHint,
        DownloadFileFailedHint,
        NotEnoughSpaceToUpHint,
        DuplicatedFileNameHint,
        WriteMetaDataErrHint,
        InvalidSharingDurationHint,
        GenSharingCodeFailedHint,
        GetSignatureFailedHint,
        PayForSharingSuccHint,
        PayForSelfSharingHint,
        TxAmountTooLowHint,
        TxAmountExceededHint,
        TxSuccessfullySentHint,
        TxSentFailedHint,
        GetMailsErrorHint,
        MailsGettingHint,
        MailSendingHint,
        MailSuccessfullySentHint,
        MakeSureToLeaveHint,
    };
    Q_ENUMS(UIHints)

    enum UIStatus {
        RefreshingFilesStatus,
        CreatingSpaceText,
        CreatingSpaceStatus,
        DownloadingStatus,
        UploadingStatus,
        TxSendedStatus,
        TxConfirmedStatus,
    };
    Q_ENUMS(UIStatus)

public:
    Q_INVOKABLE QString rpcCmds(RpcCmds cmd);

    Q_INVOKABLE QString sysMsgs(SysMessages msg);

    Q_INVOKABLE QString uiHints(UIHints hint);

    Q_INVOKABLE QString uiStatus(UIStatus status);

public:
    GUIString();
    ~GUIString();
    static GUIString* GetSingleton();
    size_t RPCCmdLength();
    void SetLanguage(QString lang);

private:
    static GUIString* m_TheSingleton;
    QString m_Language;
    std::map<RpcCmds, std::string> m_RPCCommands;
    std::map<SysMessages, std::string> m_SysMessages;
    std::map<UIHints, std::string> m_UIHintsCN;
    std::map<UIHints, std::string> m_UIHintsEN;
    std::map<UIStatus, std::string> m_UIStatusCN;
    std::map<UIStatus, std::string> m_UIStatusEN;
};


namespace CTTUi {

const QString spec = QString("/");
const QString spec2 = QString("//");
//const std::string unAuthAgentAddr = "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF";

}

#endif // STRINGS_H
