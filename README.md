# Proof of concept for using Discreet Log Contracts (DLCs) with Bitcoin Core, LND, and Taro, to exchange Sats for a synthetic USD "root USD / rUSD"

The code in this repository is a proof of concept for using Discreet Log Contracts (DLCs) with Bitcoin Core, LND, and Taro, to exchange sats for a synthetic USD "root USD / rUSD" Taro asset.
This is an experimental project and should not be used for any real-world transactions.
Also note that this is an indepent project, not affiliated with Lighting Labs, and is not endorsed by Lightning Labs (creators of both [LND](https://github.com/lightningnetwork/lnd) and [Taro](https://github.com/lightninglabs/taro)).
Credit goes to MIT's Digital Currency Initiative for the original idea and implementation of the [DLC](https://dci.mit.edu/smart-contracts) protocol. The code in this reposatory is based on the MIT DLC implementation, which is available here:
* [DLC Tutorial (golang)](https://github.com/mit-dci/lit-rpc-client-go-samples/blob/master/dlctutorial/dlctutorial.go)

## Background and Motivation
Please reference the [background.md](background.md) file for more information on the background, intention and motivation of this project.

## Setup
follow the instructions in the the [SETUP.md](SETUP.md) file to setup the environment.

## Running the demo
Once the environment is setup, clone this repository

```
git clone https://github.com/D33r-Gee/rootUSD-Prototype.git
cd rootUSD-Prototype
go build sats2rUSD.go
```

run the demo with the following command:
```
go run lnd_dlc_taro_poc.go
```

## Next Steps
* Discuss the project with the Taro team/communitty and see if this approach is something that makes sense?
* Would it wise to integrate DLCs into the Taro protocol? Or should it be a separate project?
* Would it be desirable to integrate the DLC code into LND?
* Experiment with different Oracle implementations (actually have a local instance publishing prices and pubkeys)
* Experiment with other Lightning Network implementations (eclair, c-lightning, etc.)
