#include "guiStrings.h"

GUIString* GUIString::m_TheSingleton;
QString GUIString::Spec = QString("/");

GUIString::GUIString()
{

}

GUIString::~GUIString()
{

}

GUIString* GUIString::GetSingleton()
{
    if (m_TheSingleton == nullptr)
    {
        m_TheSingleton = new GUIString();
        m_TheSingleton->m_Language = "CN";

        m_TheSingleton->m_RPCCommands =
        {
            {SetDataDirCmd, "gui_setDataDir"},
            {ShowAccountsCmd, "ele_accounts"},
            {GetAgentsInfoCmd, "gui_getAgentsInfo"},
            {ExitExternGUICmd, "gui_close"},
            {NewAccountCmd, "personal_newAccount"},
            {LoginAccountCmd, "gui_setAccountInUse"},
            {RemoteIPCmd, "gui_setRemoteIP"},
            {GetObjInfoCmd, "blizcs_getObjectsInfo"},
            {BlizReadCmd, "blizcs_read"},
            {BlizReadFileCmd, "blizcs_readFile"},
            {BlizWriteCmd, "blizcs_write"},
            {BlizWriteFileCmd, "blizcs_writeFile"},
            {CreateObjCmd, "blizcs_createObject"},
            {AddObjStorageCmd, "blizcs_addObjectStorage"},
            {GetCurAgentAddrCmd, "gui_getCurAgentAddress"},
            {GetSignedAgentInfoCmd, "elephant_getSignedCs"},
            {GetAccTBalanceCmd, "elephant_getTotalBalance"},
            {GetAccBalanceCmd, "elephant_getBalance"},
            {GetTransactionCmd, "elephant_checkTx"},
            {SignAgentCmd, "elephant_signCs"},
            {CancelAgentCmd, "elephant_cancelCs"},
            {WaitTXCompleteCmd, "gui_waitTx"},
            {GetSignatureCmd, "gui_getSignature"},
            {CheckSignatureCmd, "gui_checkSignature"},
            {GetSharingCodeCmd, "gui_getSharingCode"},
            {PayForSharedFileCmd, "gui_payForSharedFile"},
            {SendTransactionCmd, "elephant_sendTransaction"},
            {SendMailCmd, "elephant_sendMail"},
            {GetMailsCmd, "elephant_getMails"},
        };

        m_TheSingleton->m_SysMessages =
        {
            {OkMsg, "OK"},
            {ErrorMsg, "Error"},
            {OpRspMsg, "OpRsp"},
            {RateMsg, "Rate"},
            {OpTimedOutMsg, "Operation timed out"},
            {NoAnyAccountMsg, "No any accounts"},
            {FNameduplicatedMsg, "Duplicated file name"},
            {ParseAgentsFailedMsg, "Parse agent servers failed"},
            {SharingSpaceLabel, "Sharings"},
            {NeedMoreFlow, "Not enough flow"},
        };

        m_TheSingleton->m_UIHintsCN =
        {
            {CriticalErrorHint, "发生了严重错误"},
            {IncorrecctPasswdHint, "密码错误"},
            {UnavailableHint, "不可用 "},
            {WaitingForCompleteHint, "等待中‧‧‧"},
            {WaitingTXCompleteHint, "等待交易处理中‧‧‧"},
            {TXTimedoutHint, "交易处理超时, 请稍后再查看"},
            {TXIDHint, "交易哈希: "},
            {CreateAccoutFailed, "创建新账户超时或失败, 尝试重新启动程序修复此问题"},
            {InvalidAgentServDurationHint, "错误, 要申请代理服务的起止时间无效"},
            {InvalidAgentServFlowHint, "错误, 要申请代理服务的总流量值无效"},
            {AgentApplyNotCurrentHint, "要申请代理服务器不是当前所连接的, 继续要申请吗?"},
            {AgentApplyStartedHint, "开始申请代理服务‧‧‧"},
            {AgentCancelStartedHint, "开始取消代理服务‧‧‧"},
            {AgentApplyFailedHint, "申请代理服务失败, 尝试重新启动程序修复此问题"},
            {AgentCancelFailedHint, "取消代理服务失败, 尝试重新启动程序修复此问题"},
            {AgentApplySucceededHint, "申请代理服务成功"},
            {AgentCancelSucceededHint, "取消代理服务成功"},
            {AgentInvalidHint, "请先购买中转服务"},
            {ConnFailedHint, "连接到该代理商服务器失败，请稍后重试"},
            {GetAccTBalanceErrHint, "获取总余额失败"},
            {GetAccBalanceErrHint, "获取余额失败"},
            {OperationFailedHint, "操作失败"},
            {TxReturnErrorHint, "操作返回的交易解析失败"},
            {GetAgentDetailsErrHint, "获取代理商服务器详细信息失败"},
            {ParseAgentDetailsErrHint, "解析代理商服务器详细信息失败"},
            {DuplicatedSpaceNameHint, "错误, 已有同名的云盘空间"},
            {CreateSpaceSuccessHint, "创建空间已完成"},
            {CreateSpaceFailedHint, "创建空间失败"},
            {GetObjInfoErrHint, "获取存储空间信息失败"},
            {ParseObjErrHint, "存储空间解析失败"},
            {ReadMetaDataErrHint, "空间存储元数据获取失败"},
            {IncorrectSpaceMetaHint, "(已损坏)"},
            {ParseMetaErrHint, "空间存储元数据解析失败"},
            {NoMetaBakToRecoverHint, "没有可用的元数据备份来恢复磁盘"},
            {NeedToBeReformattedHint, "该空间需要被重新格式化才能继续使用, 您需要现在就格式化该空间吗?"},
            {FileInfoNotFoundHint, "错误, 未找到所需要的文件相关信息, 尝试重新启动程序修复此问题"},
            {NeedMoreFlowHint, "错误, 传输文件所需要的流量超出代理商服务的剩余流量, 请申请更多的代理商流量或变更代理商服务"},
            {DownloadFileFailedHint, "错误, 下载文件失败"},
            {NotEnoughSpaceToUpHint, "错误, 该空间的大小不足以上传该文件"},
            {DuplicatedFileNameHint, "错误, 同一个空间内文件名不能重复"},
            {WriteMetaDataErrHint, "空间存储元数据上传写入失败"},
            {InvalidSharingDurationHint, "错误, 分享的起止时间无效"},
            {GenSharingCodeFailedHint, "生成分享码失败, 请检查填入的各项参数是否正确"},
            {GetSignatureFailedHint, "获取签名失败, 尝试重新启动程序修复此问题"},
            {PayForSharingSuccHint, "已经成功获取分享的文件, 请稍等片刻, 待交易处理完毕后可以在文件的空间列表中的\"来自分享\"分类里看到该文件以便下载, 但注意不要再重复执行获取分享的操作"},
            {PayForSelfSharingHint, "错误, 无法获取自己给自己的文件分享"},
            {TxAmountTooLowHint, "错误, 您的余额不足, 无法完成"},
            {TxAmountExceededHint, "转账数额大于余额, 无法发起转账"},
            {TxSuccessfullySentHint, "发起转账成功, 请等待区块打包完成"},
            {TxSentFailedHint, "发起转账失败, 请稍后重试"},
            {GetMailsErrorHint, "获取邮件列表及内容失败, 请稍后重试"},
            {MailsGettingHint, "正在收取邮件‧‧‧"},
            {MailSendingHint, "正在发送邮件‧‧‧"},
            {MailSuccessfullySentHint, "邮件已发送"},
            {MakeSureToLeaveHint, "确定要退出吗"},
        };

        m_TheSingleton->m_UIHintsEN =
        {
            {CriticalErrorHint, "Critical error occured"},
            {IncorrecctPasswdHint, "Incorrect password"},
            {UnavailableHint, "Unavailable "},
            {WaitingForCompleteHint, "Waiting‧‧‧"},
            {WaitingTXCompleteHint, "Waiting for the transaction to complete‧‧‧"},
            {TXTimedoutHint, "Transaction has spent more than regular time, please check it later"},
            {CreateAccoutFailed, "Error, failed to create a new account, please try to restart application to fix the problem"},
            {TXIDHint, "Transaction hash: "},
            {InvalidAgentServDurationHint, "Error, the valid duration of the agent service is invalid"},
            {InvalidAgentServFlowHint, "Error, the available flow amount of the agent service is invalid"},
            {AgentApplyNotCurrentHint, "The agent service you want to apply for is not offer by current connected agent server, do you want to continue?"},
            {AgentApplyStartedHint, "Applying a transfer agent‧‧‧"},
            {AgentCancelStartedHint, "Cancelling the transfer agent‧‧‧"},
            {AgentApplyFailedHint, "Error, failed to apply for transfer agent service, please try to restart application to fix the problem"},
            {AgentCancelFailedHint, "Error, failed to cancel transfer agent service, please try to restart application to fix the problem"},
            {AgentApplySucceededHint, "Succeeded to apply for transfer agent service"},
            {AgentCancelSucceededHint, "Succeeded to cancel the transfer agent service"},
            {AgentInvalidHint, "Please purchase a transfer service from an agent"},
            {ConnFailedHint, "Failed to connect the agent, please try again later"},
            {GetAccTBalanceErrHint, "Failed to acquire total balance"},
            {GetAccBalanceErrHint, "Failed to acquire balance"},
            {OperationFailedHint, "Operation failed"},
            {TxReturnErrorHint, "Operation’s transaction failed"},
            {GetAgentDetailsErrHint, "Failed to acquire agent information"},
            {ParseAgentDetailsErrHint, "Failed to parse agent informaiton"},
            {DuplicatedSpaceNameHint, "Error, there are two spaces with the same name"},
            {CreateSpaceSuccessHint, "Space creation completed"},
            {CreateSpaceFailedHint, "Space creation failed"},
            {GetObjInfoErrHint, "Failed to acquire storage space information"},
            {ParseObjErrHint, "Failed to parse storage space"},
            {ReadMetaDataErrHint, "Failed to acquire storage metadata of space"},
            {IncorrectSpaceMetaHint, "(corrupted)"},
            {ParseMetaErrHint, "Failed to parse storage metadata of space"},
            {NoMetaBakToRecoverHint, "There are no meta data backup available to recover the space"},
            {NeedToBeReformattedHint, "The space need to be reformatted for use, do you want to reformat it now?"},
            {FileInfoNotFoundHint, "Error, failed to find related information of the required file, please try to restart application to fix the problem"},
            {NeedMoreFlowHint, "Error, the flow needed to transfer the file has exceeded the residue flow, please apply more follow from agent or change agent"},
            {DownloadFileFailedHint, "Error, failed to download files"},
            {NotEnoughSpaceToUpHint, "Error, the space is not big enough to upload the file"},
            {DuplicatedFileNameHint, "Error, there are two files with the same name in the space"},
            {WriteMetaDataErrHint, "Failed to upload storage metadata of the space"},
            {InvalidSharingDurationHint, "Error, the valid duration of the sharing file is invalid"},
            {GenSharingCodeFailedHint, "Failed to create sharing code, please check the parameter"},
            {GetSignatureFailedHint, "Failed to acquire signature, try to restart the application to fix the problem"},
            {PayForSharingSuccHint, "Succeeded to share the file, please wait a moment. When the transaction has completed, then you can download the file from \"From Share\" to download the file. Meanwhile, please do not repeat the sharing operation"},
            {PayForSelfSharingHint, "Error, unable to acquire the file you shared to yourself"},
            {TxAmountTooLowHint, "Error, not enough balance to complete"},
            {TxAmountExceededHint, "The transferred amount of token is bigger than the balance, and the transaction cannot be completed"},
            {TxSuccessfullySentHint, "Succeeded to broadcast the transacttion, please wait the transaction block to be packed"},
            {TxSentFailedHint, "Failed to broadcast the transaction, please try it later"},
            {GetMailsErrorHint, "Failed to acquire mail list and contents, please try it later"},
            {MailsGettingHint, "Receiving mails‧‧‧"},
            {MailSendingHint, "Sending the mail‧‧‧"},
            {MailSuccessfullySentHint, "Succeeded to send the mail"},
            {MakeSureToLeaveHint, "Are you sure to quit?"},
        };

        m_TheSingleton->m_UIStatusCN =
        {
            {RefreshingFilesStatus, "正在刷新文件列表‧‧‧"},
            {CreatingSpaceText, "创建空间操作"},
            {CreatingSpaceStatus, "正在格式化新空间‧‧‧"},
            {DownloadingStatus, "正在下载文件‧‧‧"},
            {UploadingStatus, "正在上传文件‧‧‧"},
            {TxSendedStatus, "交易请求已发送, 等待处理"},
            {TxConfirmedStatus, "交易处理已完成"},
        };

        m_TheSingleton->m_UIStatusEN =
        {
            {RefreshingFilesStatus, "Refreshing file list‧‧‧"},
            {CreatingSpaceText, "Create space operation"},
            {CreatingSpaceStatus, "Reformatting new space‧‧‧"},
            {DownloadingStatus, "Downloading file‧‧‧"},
            {UploadingStatus, "Uploading file‧‧‧"},
            {TxSendedStatus, "Waiting for the broadcasted transaction to be packed"},
            {TxConfirmedStatus, "The transaction has been packed successfully"},
        };
    }

    return m_TheSingleton;
}

size_t GUIString::RPCCmdLength()
{
    return m_TheSingleton->m_RPCCommands.size();
}

void GUIString::SetLanguage(QString lang)
{
    m_TheSingleton->m_Language = lang;
}

QString GUIString::rpcCmds(RpcCmds cmd)
{
    return QString::fromStdString(m_TheSingleton->m_RPCCommands[cmd]);
}

QString GUIString::sysMsgs(SysMessages msg)
{
    return QString::fromStdString(m_TheSingleton->m_SysMessages[msg]);
}

QString GUIString::uiHints(UIHints hint)
{
    if (m_TheSingleton->m_Language.compare("CN") == 0)
        return QString::fromStdString(m_TheSingleton->m_UIHintsCN[hint]);
    else
        return QString::fromStdString(m_TheSingleton->m_UIHintsEN[hint]);
}

QString GUIString::uiStatus(UIStatus status)
{
    if (m_TheSingleton->m_Language.compare("CN") == 0)
        return QString::fromStdString(m_TheSingleton->m_UIStatusCN[status]);
    else
        return QString::fromStdString(m_TheSingleton->m_UIStatusEN[status]);
}
