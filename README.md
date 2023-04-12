# Proof of concept for using Discreet Log Contracts (DLCs) with Bitcoin Core, LND, and Taro, to exchange Sats for a stablecoin USD "root USD / rUSD"


The code in this repository is a proof of concept for using Discreet Log Contracts (DLCs) with Bitcoin Core, LND, and Taro, to exchange sats for a stablecoin USD "root USD/rUSD" Taro asset.

This is an experimental project and should not be used for any real-world transactions.

Please be aware that this project is independently developed and not affiliated with Lightning Labs. While it utilizes both [LND](https://github.com/lightningnetwork/lnd) and [Taro](https://github.com/lightninglabs/taro), which were created by Lightning Labs, they have not officially endorsed our work. Nonetheless, we appreciate their contributions to the community!

Also credit goes to MIT's Digital Currency Initiative for the original idea and implementation of the [DLC](https://dci.mit.edu/smart-contracts) protocol. The code in this repository is based on the MIT DLC implementation, which is available here:
* [DLC Tutorial (golang)](https://github.com/mit-dci/lit-rpc-client-go-samples/blob/master/dlctutorial/dlctutorial.go)


## Background and Motivation
Please reference the [background.md](background.md) file for more information on the background, intention and motivation of this project.


## Setup
follow the instructions in the the [SETUP.md](SETUP.md) file to set up the environment.


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
or
```
./sats2rUSD
```


## Next Steps
* Discuss the project with the Taro team/community and see if this approach is something that makes sense?
* Would it be wise to integrate DLCs into the Taro protocol? Or should it be a separate project?
* Would it be desirable to integrate the DLC code into LND?
* Experiment with different Oracle implementations (actually have a local instance publishing prices and pubkeys)
* Experiment with other Lightning Network implementations (eclair, c-lightning, etc.)



