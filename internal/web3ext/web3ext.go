// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

// package web3ext contains geth specific web3.js extensions.
package web3ext

var Modules = map[string]string{
	"admin":      Admin_JS,
	"chequebook": Chequebook_JS,
	"clique":     Clique_JS,
	"debug":      Debug_JS,
	"eth":        Eth_JS,
	"miner":      Miner_JS,
	"net":        Net_JS,
	"personal":   Personal_JS,
	"rpc":        RPC_JS,
	"shh":        Shh_JS,
	"swarmfs":    SWARMFS_JS,
	"txpool":     TxPool_JS,
	"eleminer":   EleMiner_JS,
	"police":     Police_JS,
	"elephant":   Elephant_JS,
	"storage":    Storage_JS,
	"blizcs":     BlizCS_JS,
	"blizzard":   Blizzard_JS,
	"bases":      Bases_JS,
	"device":     Device_JS,
	"ftrans":     FTrans_JS,
	"eletxpool":  EleTxPool_JS,
}

const Chequebook_JS = `
web3._extend({
	property: 'chequebook',
	methods: [
		new web3._extend.Method({
			name: 'deposit',
			call: 'chequebook_deposit',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Property({
			name: 'balance',
			getter: 'chequebook_balance',
			outputFormatter: web3._extend.utils.toDecimal
		}),
		new web3._extend.Method({
			name: 'cash',
			call: 'chequebook_cash',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'issue',
			call: 'chequebook_issue',
			params: 2,
			inputFormatter: [null, null]
		}),
	]
});
`

const Clique_JS = `
web3._extend({
	property: 'clique',
	methods: [
		new web3._extend.Method({
			name: 'getSnapshot',
			call: 'clique_getSnapshot',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSnapshotAtHash',
			call: 'clique_getSnapshotAtHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'getSigners',
			call: 'clique_getSigners',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getSignersAtHash',
			call: 'clique_getSignersAtHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'propose',
			call: 'clique_propose',
			params: 2
		}),
		new web3._extend.Method({
			name: 'discard',
			call: 'clique_discard',
			params: 1
		}),
	],
	properties: [
		new web3._extend.Property({
			name: 'proposals',
			getter: 'clique_proposals'
		}),
	]
});
`

const Admin_JS = `
web3._extend({
	property: 'admin',
	methods: [
		new web3._extend.Method({
			name: 'addPeer',
			call: 'admin_addPeer',
			params: 1
		}),
		new web3._extend.Method({
			name: 'removePeer',
			call: 'admin_removePeer',
			params: 1
		}),
		new web3._extend.Method({
			name: 'exportChain',
			call: 'admin_exportChain',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'importChain',
			call: 'admin_importChain',
			params: 1
		}),
		new web3._extend.Method({
			name: 'sleepBlocks',
			call: 'admin_sleepBlocks',
			params: 2
		}),
		new web3._extend.Method({
			name: 'startRPC',
			call: 'admin_startRPC',
			params: 4,
			inputFormatter: [null, null, null, null]
		}),
		new web3._extend.Method({
			name: 'stopRPC',
			call: 'admin_stopRPC'
		}),
		new web3._extend.Method({
			name: 'startWS',
			call: 'admin_startWS',
			params: 4,
			inputFormatter: [null, null, null, null]
		}),
		new web3._extend.Method({
			name: 'stopWS',
			call: 'admin_stopWS'
		}),
	],
	properties: [
		new web3._extend.Property({
			name: 'nodeInfo',
			getter: 'admin_nodeInfo'
		}),
		new web3._extend.Property({
			name: 'peers',
			getter: 'admin_peers'
		}),
		new web3._extend.Property({
			name: 'datadir',
			getter: 'admin_datadir'
		}),
	]
});
`

const Debug_JS = `
web3._extend({
	property: 'debug',
	methods: [
		new web3._extend.Method({
			name: 'printBlock',
			call: 'debug_printBlock',
			params: 1
		}),
		new web3._extend.Method({
			name: 'getBlockRlp',
			call: 'debug_getBlockRlp',
			params: 1
		}),
		new web3._extend.Method({
			name: 'setHead',
			call: 'debug_setHead',
			params: 1
		}),
		new web3._extend.Method({
			name: 'seedHash',
			call: 'debug_seedHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'dumpBlock',
			call: 'debug_dumpBlock',
			params: 1
		}),
		new web3._extend.Method({
			name: 'chaindbProperty',
			call: 'debug_chaindbProperty',
			params: 1,
			outputFormatter: console.log
		}),
		new web3._extend.Method({
			name: 'chaindbCompact',
			call: 'debug_chaindbCompact',
		}),
		new web3._extend.Method({
			name: 'metrics',
			call: 'debug_metrics',
			params: 1
		}),
		new web3._extend.Method({
			name: 'verbosity',
			call: 'debug_verbosity',
			params: 1
		}),
		new web3._extend.Method({
			name: 'vmodule',
			call: 'debug_vmodule',
			params: 1
		}),
		new web3._extend.Method({
			name: 'backtraceAt',
			call: 'debug_backtraceAt',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'stacks',
			call: 'debug_stacks',
			params: 0,
			outputFormatter: console.log
		}),
		new web3._extend.Method({
			name: 'freeOSMemory',
			call: 'debug_freeOSMemory',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'setGCPercent',
			call: 'debug_setGCPercent',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'memStats',
			call: 'debug_memStats',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'gcStats',
			call: 'debug_gcStats',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'cpuProfile',
			call: 'debug_cpuProfile',
			params: 2
		}),
		new web3._extend.Method({
			name: 'startCPUProfile',
			call: 'debug_startCPUProfile',
			params: 1
		}),
		new web3._extend.Method({
			name: 'stopCPUProfile',
			call: 'debug_stopCPUProfile',
			params: 0
		}),
		new web3._extend.Method({
			name: 'goTrace',
			call: 'debug_goTrace',
			params: 2
		}),
		new web3._extend.Method({
			name: 'startGoTrace',
			call: 'debug_startGoTrace',
			params: 1
		}),
		new web3._extend.Method({
			name: 'stopGoTrace',
			call: 'debug_stopGoTrace',
			params: 0
		}),
		new web3._extend.Method({
			name: 'blockProfile',
			call: 'debug_blockProfile',
			params: 2
		}),
		new web3._extend.Method({
			name: 'setBlockProfileRate',
			call: 'debug_setBlockProfileRate',
			params: 1
		}),
		new web3._extend.Method({
			name: 'writeBlockProfile',
			call: 'debug_writeBlockProfile',
			params: 1
		}),
		new web3._extend.Method({
			name: 'writeMemProfile',
			call: 'debug_writeMemProfile',
			params: 1
		}),
		new web3._extend.Method({
			name: 'traceBlock',
			call: 'debug_traceBlock',
			params: 2,
			inputFormatter: [null, null]
		}),
		new web3._extend.Method({
			name: 'traceBlockFromFile',
			call: 'debug_traceBlockFromFile',
			params: 2,
			inputFormatter: [null, null]
		}),
		new web3._extend.Method({
			name: 'traceBlockByNumber',
			call: 'debug_traceBlockByNumber',
			params: 2,
			inputFormatter: [null, null]
		}),
		new web3._extend.Method({
			name: 'traceBlockByHash',
			call: 'debug_traceBlockByHash',
			params: 2,
			inputFormatter: [null, null]
		}),
		new web3._extend.Method({
			name: 'traceTransaction',
			call: 'debug_traceTransaction',
			params: 2,
			inputFormatter: [null, null]
		}),
		new web3._extend.Method({
			name: 'preimage',
			call: 'debug_preimage',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'getBadBlocks',
			call: 'debug_getBadBlocks',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'storageRangeAt',
			call: 'debug_storageRangeAt',
			params: 5,
		}),
		new web3._extend.Method({
			name: 'getModifiedAccountsByNumber',
			call: 'debug_getModifiedAccountsByNumber',
			params: 2,
			inputFormatter: [null, null],
		}),
		new web3._extend.Method({
			name: 'getModifiedAccountsByHash',
			call: 'debug_getModifiedAccountsByHash',
			params: 2,
			inputFormatter:[null, null],
		}),
	],
	properties: []
});
`

const Eth_JS = `
web3._extend({
	property: 'eth',
	methods: [
		new web3._extend.Method({
			name: 'sign',
			call: 'eth_sign',
			params: 2,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter, null]
		}),
		new web3._extend.Method({
			name: 'resend',
			call: 'eth_resend',
			params: 3,
			inputFormatter: [web3._extend.formatters.inputTransactionFormatter, web3._extend.utils.fromDecimal, web3._extend.utils.fromDecimal]
		}),
		new web3._extend.Method({
			name: 'signTransaction',
			call: 'eth_signTransaction',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputTransactionFormatter]
		}),
		new web3._extend.Method({
			name: 'submitTransaction',
			call: 'eth_submitTransaction',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputTransactionFormatter]
		}),
		new web3._extend.Method({
			name: 'getRawTransaction',
			call: 'eth_getRawTransactionByHash',
			params: 1
		}),
		new web3._extend.Method({
			name: 'getRawTransactionFromBlock',
			call: function(args) {
				return (web3._extend.utils.isString(args[0]) && args[0].indexOf('0x') === 0) ? 'eth_getRawTransactionByBlockHashAndIndex' : 'eth_getRawTransactionByBlockNumberAndIndex';
			},
			params: 2,
			inputFormatter: [web3._extend.formatters.inputBlockNumberFormatter, web3._extend.utils.toHex]
		}),
	],
	properties: [
		new web3._extend.Property({
			name: 'pendingTransactions',
			getter: 'eth_pendingTransactions',
			outputFormatter: function(txs) {
				var formatted = [];
				for (var i = 0; i < txs.length; i++) {
					formatted.push(web3._extend.formatters.outputTransactionFormatter(txs[i]));
					formatted[i].blockHash = null;
				}
				return formatted;
			}
		}),
	]
});
`

const Miner_JS = `
web3._extend({
	property: 'miner',
	methods: [
		new web3._extend.Method({
			name: 'start',
			call: 'miner_start',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'stop',
			call: 'miner_stop'
		}),
		new web3._extend.Method({
			name: 'setEtherbase',
			call: 'miner_setEtherbase',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter]
		}),
		new web3._extend.Method({
			name: 'setExtra',
			call: 'miner_setExtra',
			params: 1
		}),
		new web3._extend.Method({
			name: 'setGasPrice',
			call: 'miner_setGasPrice',
			params: 1,
			inputFormatter: [web3._extend.utils.fromDecimal]
		}),
		new web3._extend.Method({
			name: 'getHashrate',
			call: 'miner_getHashrate'
		}),
	],
	properties: []
});
`

// JiangHan： elephant miner ext3 命令扩展
const EleMiner_JS = `
web3._extend({
	property: 'eleminer',
	methods: [
		new web3._extend.Method({
			name: 'start',
			call: 'eleminer_start',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'stop',
			call: 'eleminer_stop'
		}),
		new web3._extend.Method({
			name: 'setEtherbase',
			call: 'eleminer_setEtherbase',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter]
		}),
	],
	properties: []
});
`

const Police_JS = `
web3._extend({
	property: 'police',
	methods: [
		new web3._extend.Method({
			name: 'start',
			call: 'police_start',
		}),
		new web3._extend.Method({
			name: 'stop',
			call: 'police_stop'
		}),
		new web3._extend.Method({
			name: 'setEtherbase',
			call: 'police_setEtherbase',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter]
		}),
	],
	properties: []
});
`

// JiangHan： elephant ext3 命令扩展
const Elephant_JS = `
web3._extend({
	property: 'elephant',
	methods: [
		new web3._extend.Method({
			name: 'getBalance',
			call: 'elephant_getBalance',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter],
			outputFormatter: web3._extend.formatters.outputBigNumberFormatter
		}),
		new web3._extend.Method({
			name: 'getSignedCs',
			call: 'elephant_getSignedCs',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter],
		}),
		new web3._extend.Method({
			name: 'getTransaction',
			call: 'elephant_getTransactionByHash',
			params: 1,
			outputFormatter: web3._extend.formatters.outputTransactionFormatter
		}),
		new web3._extend.Method({
			name: 'getBlockNumber',
			call: 'elephant_getBlockNumber',
			params: 0,
			outputFormatter: web3._extend.utils.toDecimal
		}),
		new web3._extend.Method({
			name: 'getBlock',
			call: function(args) {
				return (web3._extend.utils.isString(args[0]) && args[0].indexOf('0x') === 0) ? 'elephant_getBlockByHash' : 'elephant_getBlockByNumber';
			},
			params: 2,
			inputFormatter: [web3._extend.formatters.inputBlockNumberFormatter, function (val) { return !!val; }],
        	outputFormatter: web3._extend.formatters.outputBlockFormatter
		}),
		new web3._extend.Method({
			name: 'help',
			call: 'elephant_help',
		}),
		new web3._extend.Method({
			name: 'setLogFlag',
			call: 'elephant_setLogFlag',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'exportKS',
			call: 'elephant_exportKS',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'pbftReq',
			call: 'elephant_pbftReq',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'getShardingIndex',
			call: 'elephant_getShardingIndex',
		}),
		new web3._extend.Method({
			name: 'getMails',
			call: 'elephant_getMails',
			params: 1,
			inputFormatter: [web3._extend.formatters.inputAddressFormatter],
		}),
		new web3._extend.Method({
			name: 'regMail',
			call: 'elephant_regMail',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'checkReg',
			call: 'elephant_checkReg',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'encrypt',
			call: 'elephant_encrypt',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'desEncrypt',
			call: 'elephant_desEncrypt',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'sendTransaction',
			call: 'elephant_sendTransaction',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'sendTransactions',
			call: 'elephant_sendTransactions',
			params: 4,
		}),
	],
	properties: []
});
`

const Net_JS = `
web3._extend({
	property: 'net',
	methods: [],
	properties: [
		new web3._extend.Property({
			name: 'version',
			getter: 'net_version'
		}),
	]
});
`

const Personal_JS = `
web3._extend({
	property: 'personal',
	methods: [
		new web3._extend.Method({
			name: 'importRawKey',
			call: 'personal_importRawKey',
			params: 2
		}),
		new web3._extend.Method({
			name: 'sign',
			call: 'personal_sign',
			params: 3,
			inputFormatter: [null, web3._extend.formatters.inputAddressFormatter, null]
		}),
		new web3._extend.Method({
			name: 'ecRecover',
			call: 'personal_ecRecover',
			params: 2
		}),
		new web3._extend.Method({
			name: 'openWallet',
			call: 'personal_openWallet',
			params: 2
		}),
		new web3._extend.Method({
			name: 'deriveAccount',
			call: 'personal_deriveAccount',
			params: 3
		}),
		new web3._extend.Method({
			name: 'signTransaction',
			call: 'personal_signTransaction',
			params: 2,
			inputFormatter: [web3._extend.formatters.inputTransactionFormatter, null]
		}),
		new web3._extend.Method({
			name: 'timerSendTransaction',
			call: 'personal_timerSendTransaction',
			params: 1,
			inputFormatter: [null]
		}),
		new web3._extend.Method({
			name: 'runForSealerElection',
			call: 'personal_runForSealerElection',
			params: 2
		}),
		new web3._extend.Method({
			name: 'runForPoliceElection',
			call: 'personal_runForPoliceElection',
			params: 2
		}),
	],
	properties: [
		new web3._extend.Property({
			name: 'listWallets',
			getter: 'personal_listWallets'
		}),
	]
})
`

const RPC_JS = `
web3._extend({
	property: 'rpc',
	methods: [],
	properties: [
		new web3._extend.Property({
			name: 'modules',
			getter: 'rpc_modules'
		}),
	]
});
`

const Shh_JS = `
web3._extend({
	property: 'shh',
	methods: [
	],
	properties:
	[
		new web3._extend.Property({
			name: 'version',
			getter: 'shh_version',
			outputFormatter: web3._extend.utils.toDecimal
		}),
		new web3._extend.Property({
			name: 'info',
			getter: 'shh_info'
		}),
	]
});
`

const SWARMFS_JS = `
web3._extend({
	property: 'swarmfs',
	methods:
	[
		new web3._extend.Method({
			name: 'mount',
			call: 'swarmfs_mount',
			params: 2
		}),
		new web3._extend.Method({
			name: 'unmount',
			call: 'swarmfs_unmount',
			params: 1
		}),
		new web3._extend.Method({
			name: 'listmounts',
			call: 'swarmfs_listmounts',
			params: 0
		}),
	]
});
`

const TxPool_JS = `
web3._extend({
	property: 'txpool',
	methods: [],
	properties:
	[
		new web3._extend.Property({
			name: 'content',
			getter: 'txpool_content'
		}),
		new web3._extend.Property({
			name: 'inspect',
			getter: 'txpool_inspect'
		}),
		new web3._extend.Property({
			name: 'status',
			getter: 'txpool_status',
			outputFormatter: function(status) {
				status.pending = web3._extend.utils.toDecimal(status.pending);
				status.queued = web3._extend.utils.toDecimal(status.queued);
				return status;
			}
		}),
	]
});
`

const Storage_JS = `
web3._extend({
	property: 'storage',
	methods:
	[
		new web3._extend.Method({
			name: 'claim',
			call: 'storage_claim',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'verify',
			call: 'storage_verify',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'rent',
			call: 'storage_rent',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'printChunkData',
			call: 'storage_printChunkData',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'printChunkVerifyData',
			call: 'storage_printChunkVerifyData',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'getChunkProvider',
			call: 'storage_getChunkProvider',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'sendBlizProtocol',
			call: 'storage_sendBlizProtocol',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'writeRawData',
			call: 'storage_writeRawData',
			params: 5,
		}),
		new web3._extend.Method({
			name: 'readRawData',
			call: 'storage_readRawData',
			params: 4,
		}),
		new web3._extend.Method({
			name: 'writeHeaderData',
			call: 'storage_writeHeaderData',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'readHeaderData',
			call: 'storage_readHeaderData',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'getChunkIdList',
			call: 'storage_getChunkIdList',
		}),
		new web3._extend.Method({
			name: 'logHeaderData',
			call: 'storage_logHeaderData',
			params: 3,
		}),
	]
});
`

// qiwy： 命令扩展
const BlizCS_JS = `
web3._extend({
	property: 'blizcs',
	methods: [
		new web3._extend.Method({
			name: 'test',
			call: 'blizcs_test',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'createObject',
			call: 'blizcs_createObject',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'addObjectStorage',
			call: 'blizcs_addObjectStorage',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'openObject',
			call: 'blizcs_openObject',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'doSeek',
			call: 'blizcs_doSeek',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'write',
			call: 'blizcs_write',
			params: 4,
		}),
		new web3._extend.Method({
			name: 'read',
			call: 'blizcs_read',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'dail',
			call: 'blizcs_dail',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'writefile',
			call: 'blizcs_writeFile',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'readfile',
			call: 'blizcs_readFile',
			params: 4,
		}),
		new web3._extend.Method({
			name: 'getRentSize',
			call: 'blizcs_getRentSize',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'claim',
			call: 'blizcs_claim',
		}),
		new web3._extend.Method({
			name: 'getObjectsInfo',
			call: 'blizcs_getObjectsInfo',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'wSWrite',
			call: 'blizcs_wSWrite',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'wSRead',
			call: 'blizcs_wSRead',
			params: 3,
		}),
		new web3._extend.Method({
			name: 'testString',
			call: 'blizcs_testString',
			params: 4,
		}),
		new web3._extend.Method({
			name: 'writeSign',
			call: 'blizcs_writeSign',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'getCsServeCost',
			call: 'blizcs_getCsServeCost',
			params: 2,
		}),
		new web3._extend.Method({
			name: 'getUsedFlow',
			call: 'blizcs_getUsedFlow',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'getAuthAllowFlowData',
			call: 'blizcs_getAuthAllowFlowData',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'showAsso',
			call: 'blizcs_showAsso',
			params: 1,
		}),
		new web3._extend.Method({
			name: 'shareObj',
			call: 'blizcs_shareObj',
		}),
	],
	properties: []
});
`

const Blizzard_JS = `
web3._extend({
	property: 'blizzard',
	methods: [
		new web3._extend.Method({
			name: 'test',
			call: 'blizzard_test',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'getShardingMiner',
			call: 'blizzard_getShardingMiner',
			params: 0,
		}),
		new web3._extend.Method({
			name: 'deleteNode',
			call: 'blizzard_deleteNode',
			params: 1,
		}),
	],
	properties: []
});
`

const Bases_JS = `
web3._extend({
	property: 'bases',
	methods: [
		new web3._extend.Method({
			name: 'test',
			call: 'bases_test',
			params: 0,
		}),
	],
	properties: []
});
`

const Device_JS = `
web3._extend({
	property: 'device',
	methods: [
		new web3._extend.Method({
			name: 'test',
			call: 'device_test',
		}),
	],
	properties: []
});
`

const FTrans_JS = `
web3._extend({
	property: 'ftrans',
	methods: [
		new web3._extend.Method({
			name: 'test',
			call: 'ftrans_test',
		}),
	],
	properties: []
});
`

const EleTxPool_JS = `
web3._extend({
	property: 'eletxpool',
	methods: [
		new web3._extend.Method({
			name: 'detailedInputTxs',
			call: 'eletxpool_detailedInputTxs',
			params: 2,
		}),
	],
	properties:
	[
		new web3._extend.Property({
			name: 'content',
			getter: 'eletxpool_content'
		}),
		new web3._extend.Property({
			name: 'inspect',
			getter: 'eletxpool_inspect'
		}),
		new web3._extend.Property({
			name: 'status',
			getter: 'eletxpool_status',
			outputFormatter: function(status) {
				status.pending = web3._extend.utils.toDecimal(status.pending);
				status.queued = web3._extend.utils.toDecimal(status.queued);
				return status;
			}
		}),
		new web3._extend.Property({
			name: 'inputContent',
			getter: 'eletxpool_inputContent'
		}),
	]
});
`
