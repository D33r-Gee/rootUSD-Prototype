# Setting up a test environment with two LND nodes and two Taro nodes
This tutorial will help you set up a test environment with two LND nodes, two Taro nodes and a Bitcoin regtest on your computer. This was tested on Ubuntu 20.04, but should work on other Linux distributions as well. It should also work on MacOS and Windows, but you will have to figure out how to install the dependencies yourself.

## Step 1: Install Bitcoin Core in Regtest mode
The first step is to download Bitcoin Core, and run it in regtest mode. This allows for easy (simulated) mining of new blocks, and does not make you dependant on obtaining official testnet coins.

[Download Bitcoin Core](https://bitcoincore.org/en/download/) and install it. Before running it, make a file called bitcoin.conf with the following contents:

```
regtest=1
txindex=1
daemon=0
maxmempool=100
txindex=1
listen=0
# Enable publish raw block in <address>
zmqpubrawblock=tcp://127.0.0.1:28332
# Enable publish raw transaction in <address>
zmqpubrawtx=tcp://127.0.0.1:28333
debug=mempool
debug=rpc
shrinkdebugfile=1
rest=1
# Username and HMAC-SHA-256 hashed password for JSON-RPC connections. The
# field <userpw> comes in the format: <USERNAME>:<SALT>$<HASH>. A
# canonical python script is included in share/rpcauth. The client
# then connects normally using the
# rpcuser=<USERNAME>/rpcpassword=<PASSWORD> pair of arguments. This
# option can be specified multiple times
rpcauth=<USERNAME>/rpcpassword=<PASSWORD>
server=1
disablewallet=0
# Options for regtest
[regtest]
# rpcport=18444
addresstype=legacy
changetype=legacy

```

and place it in the correct location depending on your operating system:

| Platform | Location |
|----|----|
| Linux   | `$HOME/.bitcoin/` |

In your .bashrc file (Linux), create aliases for bitcoind and bitcoin-cli in regtest mode:
* alias regtestd='bitcoind -regtest -fallbackfee=0.00001'
* alias regtest-cli='bitcoin-cli -regtest'

Then, open up a terminal window and startup Bitcoin Core in regtest mode:
```
regtestd
```

And in another window create or load a wallet
```
regtest-cli createwallet "name"
# or load it
regtest-cli loadwallet "name"
```

## Step 2: Generate the first blocks

Bitcoin Core will start in regtest mode

Since this is an empty blockchain right now, you need to generate some blocks to get it going.
In your regtest-cli terminal input the following to generate 200 blocks:
```
regtest-cli -generate 200
```

now check your balance:
```
regtest-cli getbalance
```
There should be some bitcoin in there now.

## Step 3: Install the LND nodes

Now the blockchain is up, we can install the LND nodes. Because we will be using Taro it is important to build LND from source with the relevant tags (please follow instructions [here](https://github.com/lightningnetwork/lnd/blob/master/docs/INSTALL.md)).
When it's time to make install use the following tags:
```
make install tags="signrpc chainrpc walletrpc routerrpc invoicesrpc chainkit dev"
```
Create two subfolders in your '$HOME' directory, called `.lnd1` for the `Oracle` and `.lnd2` for `Alisha`. Create a file called `lnd.conf` with the following contents:

```
[Bitcoin]

bitcoin.active=1
bitcoin.regtest=1
bitcoin.node=bitcoind

[Bitcoind]

bitcoind.rpchost=localhost
bitcoind.rpcuser=<USERNAME>
bitcoind.rpcpass=<PASSWORD>
bitcoind.zmqpubrawblock=tcp://127.0.0.1:28332
bitcoind.zmqpubrawtx=tcp://127.0.0.1:28333

[protocol]
# Enable large channels support
#protocol.wumbo-channels=1
# Enable Custom Messages
protocol.custom-message=554
```

Copy the file to both folders (`.lnd1` and `.lnd2`). Open the copy in the `.lnd2` folder and edit it with the following contents:
```
[Application Options]

listen=0.0.0.0:9734
rpclisten=localhost:11009
restlisten=0.0.0.0:8180

[Bitcoin]

bitcoin.active=1
bitcoin.regtest=1
bitcoin.node=bitcoind

[Bitcoind]

bitcoind.rpchost=localhost
bitcoind.rpcuser=<USERNAME>
bitcoind.rpcpass=<PASSWORD>
bitcoind.zmqpubrawblock=tcp://127.0.0.1:28332
bitcoind.zmqpubrawtx=tcp://127.0.0.1:28333

[protocol]
# Enable large channels support
#protocol.wumbo-channels=1
# Enable Custom Messages
protocol.custom-message=554
```

Then create aliases in your .bashrc file for both the `Oracle` (LND1)  and `Alisha` (LND2) nodes:
```
export LND1_DIR="/home/<USER>/.lnd1"
alias lnd1="lnd --lnddir=$LND1_DIR";
alias lncli1="lncli -n regtest --lnddir=$LND1_DIR"

export LND2_DIR="/home/<USER>/.lnd2"
alias lnd2="lnd --lnddir=$LND2_DIR";
alias lncli2="lncli -n regtest --lnddir=$LND2_DIR --rpcserver=localhost:11009"
```
## Step 4: Starting the LND nodes

Open two terminal or command-line windows. In the first window, type this command to start up the first LND node:

```lnd1``` 

When starting up LND for the first time, open another terminal window and use the cli command to create a wallet:
```
lncli1 create
```
then follow the instructions

Then, similarly for the first node, use the second terminal window to start the second node:

```lnd2``` 

## Step 5: Fund the LND wallets

Now that the nodes are up, we should fund the wallets. In the cli window in the following command to creata a Pay To Taproot address (p2tr):

(On Unix / MacOS:)
```lncli1 newaddress p2tr```

Copy the response (it should start with `bcrt1[...]`) and go to your regtest-cli terminal window and input the following:

```regtest-cli sendtoaddress  <NEW-LND-P2TR-ADDRESS> 10```

Here we are sending 10 BTC from our regtest Bitcoin node to our LND1 (`Oracle`) node

Next, use the debug console to generate another 10 blocks
```
regtest-cli -generate 10
```

When you issue the `walletbalance` command again in `lncli1` you'll see that the updated `total_balance` and `confirmed_balance`:

```
lncli1 wallet balance

```

Repeat the funding steps for the lnd2 node.


## Step 6: Connect the nodes together

In order for the nodes to connect to each other, you need to find out the address of the first node, and then instruct the second node to connect. Connections are bi-directional, so they don't both need to connect to each other.

First, find out the address on the second node (lnd2) by issuing the `lncli2 getinfo` command. You will see something like this:

```
{
    "version": "0.15.99-beta commit=kvdb/v1.4.1-17-g33a0cbe63",
    "commit_hash": "33a0cbe63440d5ae50cf6fc81855acc5b90b2622",
    "identity_pubkey": "<LND2_PUBKEY>",
    "alias": "TESTING",
    "color": "#3399ff",
    "num_pending_channels": 0,
    "num_active_channels": 1,
    "num_inactive_channels": 1,
    "num_peers": 1,
    "block_height": 1423,
    "block_hash": "7f09563eb68da125980a9c7034bc8c598908a9c8a332e96d823964e9f2ff452c",
    "best_header_timestamp": "1679942219",
    "synced_to_chain": false,
    "synced_to_graph": true,
    "testnet": false,
    "chains": [
        {
            "chain": "bitcoin",
            "network": "regtest"
        }
    ],
[...]
}
```

Now we will connect node one to node two by running:

```
lncli1 connect <LND2_PUBKEY>@localhost:<port_number>
```

Now run ```lncli1 listpeers```. You should see an output similar to this:

```
"peers": [
        {
            "pub_key": "<LND2_PUBKEY>",
            "address": "127.0.0.1:<port_number>",
            "bytes_sent": "28959",
            "bytes_recv": "28999",
            "sat_sent": "53041",
            "sat_recv": "868083",
            "inbound": true,
            "ping_time": "607",
            "sync_type": "ACTIVE_SYNC",
            [...]
        }]

```

Now your two nodes are connected to one another.

## Step 7: Create a channel between the nodes

Now that the nodes are connected, we can create a channel between them. First, we need to find out the `funding_txid` of the channel. To do this, we can use the `lncli1 listchannels` command:

```
lncli1 openchannel <LND2_PUBKEY> 1000000
```

Go to your bitcoin-cli terminal window and generate 6 blocks:

```
regtest-cli -generate 6
```
Then confirm that the channel is open by running:

```
lncli1 listchannels
lncli2 listchannels
```
The last commands should return the status of your channels, like this:

```
"channels": [
        {
            "active": true,
            "remote_pubkey": "<LND1_PUBKEY>",
            "channel_point": "d51ffea8c778fa21215d08ea46ab6cd4ac566d473ff7e943472a8d2af3636b73:0",
            "chan_id": "1146790627835904",
            "capacity": "1000000",
            [...]
        } ]

```

The nodes are now connected together. 

## Step 8: Creating and Sending an invoice

Now that the nodes are connected, we can create an invoice on one node and pay it on the other. First, we need to create an invoice on the first node:

```
lncli1 addinvoice --amt 1000
```

This will return an invoice with a payment hash, like this:
    
    {
        "r_hash": "<PAYMENT_HASH>",
        "pay_req": "<INVOICE>"
    }

Let's inspect the invoice on the `lnd2` node:

```
lncli2 decodepayreq <INVOICE>
```

Copy the payment hashrequest and use it to pay the invoice on the second node:

```
lncli2 payinvoice <INVOICE>
```

if everything worked the response will look like this:

```
[...]
+------------+--------------+--------------+--------------+-----+----------+------------------+----------------------+
| HTLC_STATE | ATTEMPT_TIME | RESOLVE_TIME | RECEIVER_AMT | FEE | TIMELOCK | CHAN_OUT         | ROUTE                |
+------------+--------------+--------------+--------------+-----+----------+------------------+------+------------+--------------+--------------+--------------+-----+----------+------------------+----------------------+
| HTLC_STATE | ATTEMPT_TIME | RESOLVE_TIME | RECEIVER_AMT | FEE | TIMELOCK | CHAN_OUT         | ROUTE                |
+------------+--------------+--------------+--------------+-----+----------+------------------+----------------------+
| SUCCEEDED  |        2.474 |        3.204 | 1200         | 0   |     1466 | 1146790627835904 | 022d0485886b9cd7cfdb |
+------------+--------------+--------------+--------------+-----+----------+------------------+----------------------+
Amount + fee:   1000 + 0 sat
Payment hash:   b8dc48d6d575555775b818372795be8744cc14bb4271bd9fae068411ec11f502
Payment status: SUCCEEDED, preimage: 3a6b25aff704b06283461cefd43d3656d8b28b531cbecd8b1e590a27baf2b53e

```


## Step 9: Install the Taro Nodes

It's time to set up the Taro nodes. To build Taro from source follow the instructions [here](https://github.com/lightninglabs/taro#installation)

Then make two new directories in your home directory:

```
mkdir -p ~/.taro1
mkdir -p ~/.taro2
```
then create aliases for the taro binaries:

```
# Taro 1 Node
alias tarod1='tarod --tarodir=/home/<USER>/.taro1 --network=regtest --rpclisten=127.0.0.1:10030 --restlisten=127.0.0.1:8090 --lnd.host=localhost:10009 --lnd.macaroonpath=/home/<USER>/.lnd1/data/chain/bitcoin/regtest/admin.macaroon --lnd.tlspath=/home/<USER>/.lnd1/tls.cert'

alias taro1-cli='tarocli --network=regtest --rpcserver=127.0.0.1:10030 --tarodir=/home/<USER>/.taro1'

# Taro 2 Node

alias tarod2='tarod --tarodir=/home/<USER>/.taro2 --network=regtest --rpclisten=127.0.0.1:11029 --restlisten=127.0.0.1:8091 --lnd.host=localhost:11009 --lnd.macaroonpath=/home/<USER>/.lnd2/data/chain/bitcoin/regtest/admin.macaroon --lnd.tlspath=/home/<USER>/.lnd2/tls.cert'

alias taro2-cli='tarocli --network=regtest --rpcserver=127.0.0.1:11029 --tarodir=/home/<USER>/.taro2'

```
Open two new terminal windows and run the following commands in each window:

```
tarod1
```
    
```
tarod2
```

## Step 10: Mint some Taro coins (root USD/rUSD)

Now that the Taro nodes are running, we can mint some Taro coins. First, we need to create a new address on the first node:

```
taro1-cli assets mint --type normal --name rUSD --supply 100 --meta "root USD" --skip_batch
```


```
taro1-cli assets list
```
This will return somehting like this:
```
{
    "assets": [
        {
            "version": 0,
            "asset_genesis": {
                "genesis_point": "e7fb1b0245be6e5935067285ed980623413c3f248f52cce4ddcc8733f9187476:1",
                "name": "rUSD",
                "meta": "726f6f7420555344",
                "asset_id": "208ac1e44357cda3c5ce7543e54170d459b7209fec6cccca69fb4ccc62bcd028",
                "output_index": 0,
                "genesis_bootstrap_info": "767418f93387ccdde4cc528f243f3c41230698ed85720635596ebe45021bfbe700000001047255534408726f6f74205553440000000000",
                "version": 0
            },
            "asset_type": "NORMAL",
            "amount": "53",
            "lock_time": 0,
            "relative_lock_time": 0,
            "script_version": 0,
            "script_key": "026cdb549ace9230b24712693d99569c3cc35c98bfe8047ecfddd68dd933782d07",
            "asset_group": null,
            "chain_anchor": {
                [...]
            }
        }
    ]
}
```
## Step 11: Request a payment from the second Taro node

```
taro2-cli addrs new --genesis_bootstrap_info 767418[...]0000000 --amt 23
```
This will return something like this:
```
{
    "encoded": "tarort1qqqsqq3hwe6p37fnslxdmexv228jg0eugy3sdx8ds4eqvd2ed6ly2qsml0nsqqqqqyz8y42ngsy8ymm0wss9256yqqqqqqqqqsssydash3qqw3rc6fz2ql3lnyy0vxj54fhcpk2ked2h9kn6we733k93qcssy98gsflzq6pfnjm8njeu00rgdltu7ut2cqhr6n7qngqhvxsmr234pqqsz4h3huc",
    "asset_id": "208ac1e44357cda3c5ce7543e54170d459b7209fec6cccca69fb4ccc62bcd028",
    "asset_type": "NORMAL",
    "amount": "23",
    "group_key": null,
    "script_key": "0237b0bc40074478d244a07e3f9908f61a54aa6f80d956cb5572da7a767d18d8b1",
    "internal_key": "0214e8827e2068299cb679cb3c7bc686fd7cf716ac02e3d4fc09a01761a1b1aa35",
    "taproot_output_key": "b6b0b7b7c788251713d18efa6908a933e3826256dfefc30083c7c037ac3ce393"
}
```
Copy the `encoded` field and paste it after the command `assets send --addr` in the first terminal window where the first Taro node is running, like so:
```
taro1-cli assets send --addr tarort1q[...]sz4h3huc
```

This will return something like this:
```
{
    "transfer_txid": "9993655f8[...]21eb9ee2",
    "anchor_output_index": 0,
    "transfer_tx_bytes": "020000000001[...]3bb00000000",
    "taro_transfer": {
        "old_taro_root": "0000147e8f[...]20a4deea72",
        "new_taro_root": "e5677ac5cd[...]f41da8799",
        "prev_inputs": [...],
        "new_outputs": [...],
    },
    "total_fee_sats": "12725"
}
```

## Step 12: Check the balances of the Taro nodes

In the tertimal window where the Bitcoin node is running, run the following command:
```
regtest-cli -generate 1
```
This will mine a new block and confirm the transaction.

Now run the following command in the first terminal window where the first Taro node is running:
```
taro1-cli assets balance list
```
Check the `balance` field of the `rUSD` asset. It should be `77` now.

Then do the same in the second terminal window where the second Taro node is running:
```
taro2-cli assets balance list
```
Check the `balance` field of the `rUSD` asset. It should be `23` now.

Ok we are now all set up and ready to go!


## Next steps

Now that you have a set up with two LND nodes, two Taro nodes and Bitcoin Core daemon running on regtest, you can continue with the proof of concept tutorial of exchanging sats for rUSD using a Discreet Log Contract:

* [Executing a Discreet Log Contract using LND-AF] (execute-dlc-litaf.md)
