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

package params

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []string{
	/*
		// Ethereum Foundation Go Bootnodes
		"enode://a979fb575495b8d6db44f750317d0f4622bf4c2aa3365d6af7c284339968eef29b69ad0dce72a4d8db5ebb4968de0e3bec910127f134779fbcb0cb6d3331163c@52.16.188.185:30303", // IE
		"enode://3f1d12044546b76342d59d4a05532c14b85aa669704bfe1f864fe079415aa2c02d743e03218e57a33fb94523adb54032871a6c51b2cc5514cb7c7e35b3ed0a99@13.93.211.84:30303",  // US-WEST
		"enode://78de8a0916848093c73790ead81d1928bec737d565119932b98c6b100d944b7a95e94f847f689fc723399d2e31129d182f7ef3863f2b4c820abbf3ab2722344d@191.235.84.50:30303", // BR
		"enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@13.75.154.138:30303", // AU
		"enode://1118980bf48b0a3640bdba04e0fe78b1add18e1cd99bf22d53daac1fd9972ad650df52176e7c7d89d1114cfef2bc23a2959aa54998a46afcf7d91809f0855082@52.74.57.123:30303",  // SG

		// Ethereum Foundation C++ Bootnodes
		"enode://979b7fa28feeb35a4741660a16076f1943202cb72b6af70d327f053e248bab9ba81760f39d0701ef1d8f89cc1fbd2cacba0710a12cd5314d5e0c9021aa3637f9@5.1.83.226:30303", 	// DE
	*/
	// jianghan 测试node
	// "enode://9d7bb806d0addcdcfbf94bab0dbe24901351e93ca2fd3a661358968f6fe8bb29b0c4c0137c86b69babd0858c2e8a28976a5bcbc82bfa2ba8726f417523aca70e@127.0.0.1:30303", // chain-jh
	// "enode://278261e67454b7350316211f15b2daf61d743a8a7847941402ecf566c18ecaa8e3848c1da0686a0fe4bbc64db960f1b09490cae085f567186c8a0c2fa72f7c65@127.0.0.1:30304", // chain-jh-2
	// "enode://894723644e5550d21f55847edd22cb3e9891ecaa4f4f58721eea1443792ea1fe072632e555e856ebfaa7503509d4baaf6782993f32525a9e4d98e935973a45bd@192.168.198.132:30303", // chain-qiwy
	// "enode://937739dad7b75883be8c08fc6d38ef0baea992b608079fc61336222284f6e4368089288f75be2cf9bb4e90c617fca4f441c600dd612a30121c9c3ea278767c0e@192.168.198.132:30304", // chain-qiwy-2
	// "enode://10c532b5084083096361f7a69862b5ff4e9183c767093b9e891faf3e753e5ce0eaae462ec1c05ae41a79a645ed673953443427b0c544a427abbd668bc4a0ca32@192.168.198.132:30305", // chain-qiwy-3
	// "enode://0e24c6e46798ac424eb8068fe0dcfab871439b5160b699e24530a10ebb6d1453a3997133b0905276ea1b794848090f8d6fd87c18a94b72a459800e3832a6acf1@127.0.0.1:30303",       // chain-qiwy-1-windows
	// "enode://a6809bb99fe72f87f2327dc8bb39bf4219164dec39ce243a0863f3d83f97022b8a13e164bf25d1463e5cbf6b185df2dd01fbfe5b172f6d951335cb66f2239329@127.0.0.1:30304",       // chain-qiwy-2-windows
	// "enode://a5a0796fbae5684eb8eb8e3b04cd41ed9c1515846c5d0879debf9fed0a2b08151ac44ffb38b153dc6945ee8f19ef1ec8b14f11addd8790221068b987028b67c7@127.0.0.1:30305",       // chain-qiwy-3-windows
	// "enode://a97d084bb5c8c4e7cd2c6df1bd7a0388043a6b93df6e4ebdc41abcc859a27825f49a21f631c0eebc675257cfa14720686f5345c12e4dc07f929707450969826c@127.0.0.1:30306",       // chain-qiwy-4-windows
	// "enode://ab0ed3fa04fee7494c9c7b4bd7c09e44dadce4a0b839d71c9afd63a2c2056aa66f3a3ef3a12797c261fb7e450523a9b1311654f7fb3cac8914e19be47ddbb41c@27.102.118.20:30303",   // internet-Korea
	// "enode://ac1307faa9db990d583bdb3088205f103d729f65892c547587c83996498eb84ffd96af9239e97d47cd8d337965432ab8615e4ade276a5f8e911b57928946696e@67.216.221.223:30303",  // internet-Los Angeles
	// "enode://d54fa49aab7edbbb1300fdd9ac647445dff018b7735115c613a34d500e93897af3db7d4536f85369eb0d1960c97fc74fd9cad37011a1b7bbc6ea1a58ab4a22d4@118.89.165.39:30303", // internet-China
	"enode://19a44d6a982a01a910b6884e6dd32ac5ac29f8f349736f230b090556971460d10619b20c46310d597cb7410790d2e4a80edb39e5e7fc616749a51b4b20970305@212.129.144.237:30303", // internet-China2
	"enode://141216631bf9ed615b5c361be70257afdcb834c3510a0df209d96a09b0c3fdc01df1fb8fdb7df5a1eb6891f592a6486761f0b0fed5f7aa41efca9f1d9900dd69@129.211.123.23:30303",  // internet-China3
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Ropsten test network.
var TestnetBootnodes = []string{
	"enode://30b7ab30a01c124a6cceca36863ece12c4f5fa68e3ba9b0b51407ccc002eeed3b3102d20a88f1c1d3c3154e2449317b8ef95090e77b312d5cc39354f86d5d606@52.176.7.10:30303",    // US-Azure geth
	"enode://865a63255b3bb68023b6bffd5095118fcc13e79dcf014fe4e47e065c350c7cc72af2e53eff895f11ba1bbb6a2b33271c1116ee870f266618eadfc2e78aa7349c@52.176.100.77:30303",  // US-Azure parity
	"enode://6332792c4a00e3e4ee0926ed89e0d27ef985424d97b6a45bf0f23e51f0dcb5e66b875777506458aea7af6f9e4ffb69f43f3778ee73c81ed9d34c51c4b16b0b0f@52.232.243.152:30303", // Parity
	"enode://94c15d1b9e2fe7ce56e458b9a3b672ef11894ddedd0c6f247e0f1d3487f52b66208fb4aeb8179fce6e3a749ea93ed147c37976d67af557508d199d9594c35f09@192.81.208.223:30303", // @gpip
}

// RinkebyBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Rinkeby test network.
var RinkebyBootnodes = []string{
	"enode://a24ac7c5484ef4ed0c5eb2d36620ba4e4aa13b8c84684e1b4aab0cebea2ae45cb4d375b77eab56516d34bfbd3c1a833fc51296ff084b770b94fb9028c4d25ccf@52.169.42.101:30303", // IE
	"enode://343149e4feefa15d882d9fe4ac7d88f885bd05ebb735e547f12e12080a9fa07c8014ca6fd7f373123488102fe5e34111f8509cf0b7de3f5b44339c9f25e87cb8@52.3.158.184:30303",  // INFURA
	"enode://b6b28890b006743680c52e64e0d16db57f28124885595fa03a562be1d2bf0f3a1da297d56b13da25fb992888fd556d4c1a27b1f39d531bde7de1921c90061cc6@159.89.28.211:30303", // AKASHA
}

// DiscoveryV5Bootnodes are the enode URLs of the P2P bootstrap nodes for the
// experimental RLPx v5 topic-discovery network.
var DiscoveryV5Bootnodes = []string{
	"enode://06051a5573c81934c9554ef2898eb13b33a34b94cf36b202b69fde139ca17a85051979867720d4bdae4323d4943ddf9aeeb6643633aa656e0be843659795007a@35.177.226.168:30303",
	"enode://0cc5f5ffb5d9098c8b8c62325f3797f56509bff942704687b6530992ac706e2cb946b90a34f1f19548cd3c7baccbcaea354531e5983c7d1bc0dee16ce4b6440b@40.118.3.223:30304",
	"enode://1c7a64d76c0334b0418c004af2f67c50e36a3be60b5e4790bdac0439d21603469a85fad36f2473c9a80eb043ae60936df905fa28f1ff614c3e5dc34f15dcd2dc@40.118.3.223:30306",
	"enode://85c85d7143ae8bb96924f2b54f1b3e70d8c4d367af305325d30a61385a432f247d2c75c45c6b4a60335060d072d7f5b35dd1d4c45f76941f62a4f83b6e75daaf@40.118.3.223:30307",
}

var TestBootNodeIntranet1 = []string{
	"enode://894723644e5550d21f55847edd22cb3e9891ecaa4f4f58721eea1443792ea1fe072632e555e856ebfaa7503509d4baaf6782993f32525a9e4d98e935973a45bd@192.168.198.132:30303", // chain-qiwy
	"enode://937739dad7b75883be8c08fc6d38ef0baea992b608079fc61336222284f6e4368089288f75be2cf9bb4e90c617fca4f441c600dd612a30121c9c3ea278767c0e@192.168.198.132:30304", // chain-qiwy-2
	"enode://10c532b5084083096361f7a69862b5ff4e9183c767093b9e891faf3e753e5ce0eaae462ec1c05ae41a79a645ed673953443427b0c544a427abbd668bc4a0ca32@192.168.198.132:30305", // chain-qiwy-3
	"enode://d7088cbb59f82139e5a03589f57c3428220b078433c6d5e163bd1cdd372d8830136aac4946b35f1431b80d97e9bbcc41802d6ca5f215cf4c5ac9654d54d2798a@127.0.0.1:30303",       // chain-qiwy-1-windows
	"enode://02536e28a857b08b4a286278b75e0555f1d1be79f5a5049f309970984fc004d1e456e892c08614e6d902867056cc4a10d3d78b71ee407cad59232066ca53a44b@127.0.0.1:30304",       // chain-qiwy-2-windows
	"enode://42c45c7277aea9f09b4d9a3bc42907216a653b31b17fdca857656f590d1f85cc84c6346c3668ce18300015e7fc082e766e42074c61c1e8022a80e8bdc1059006@127.0.0.1:30305",       // chain-qiwy-3-windows
	"enode://06ec82dee28bf98700d9309e4a6658255a132e46d8680a0304814b594d90747d0270af74d6dbad2467c44ad8bdc82e4e15bf7dc2706ec2e7c02fbad2d8acc4e5@127.0.0.1:30306",       // chain-qiwy-4-windows
	"enode://ea44180fea5fcbcb38384fd08d23dbd17777e3cd7fdfc0ae16625921a963bd2b86f25b60a6076873c9a7c9769ed0b8f1678627b7a6effbdf87465065bd592029@127.0.0.1:30307",       // chain-qiwy-5-windows
}

var TestBootNode2 = []string{
	"enode://19a44d6a982a01a910b6884e6dd32ac5ac29f8f349736f230b090556971460d10619b20c46310d597cb7410790d2e4a80edb39e5e7fc616749a51b4b20970305@212.129.144.237:30303", // internet-China2
}

var TestBootNode3 = []string{
	"enode://141216631bf9ed615b5c361be70257afdcb834c3510a0df209d96a09b0c3fdc01df1fb8fdb7df5a1eb6891f592a6486761f0b0fed5f7aa41efca9f1d9900dd69@129.211.123.23:30303", // internet-China3
}

var TestBootNode4 = []string{
	"enode://2ec4a1404c62aeb91a5c7439b3a0508dac895cf0b6dac5bb97bf2acd50b6c894be11bdc62029752c0437f1df1f41f377b43a777995a98727f0628baa9694a9b5@106.54.54.183:30303", // internet-China4
}

var TestBootNode5 = []string{
	"enode://62b6565abc3a5094bda17ed8895c2e0daae0513f8ea0153df7368a3fa0d4bf3e064e5e7086ce6913835540c1b3d22810783bbb484cc519371e8f1cb0ae5306c3@112.229.236.194:30303", // internet-China5
}

var TestBootNode6 = []string{
	"enode://2d43f4bce36d30b1a3e09c8b134d521f3ace7127d019696c5e4767f75eb94d4d8de44dda45f37c903ead21030bf61dce9f4bcfcfce0e8ada33e3d63188ce9654@111.229.122.224:30303", // internet-China6
}

var TestBootNode7 = []string{
	"enode://d40e9052a8d1e77301484910345f0ffecb752b1f992f392673c037ce2ff55a86572732fc5ae2a57be94599e94f836095f978b5415b70e3d3faa31aa7797b3776@122.51.69.195:30303", // internet-China7
}
