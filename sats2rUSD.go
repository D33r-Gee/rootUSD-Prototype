package main

import (
	// "bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"

	"strconv"
	"strings"
	"time"

	// lndwire "github.com/btcsuite/btcd/wire"
	colly "github.com/gocolly/colly/v2"

	"github.com/lightninglabs/taro/tarorpc"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lightningnetwork/lnd/signal"

	// "github.com/lightningnetwork/lnd/lnwallet"

	// btsch "github.com/btcsuite/btcd/chaincfg/chainhash"
	dlcoracle "github.com/mit-dci/dlc-oracle-go"
	"github.com/mit-dci/lit/btcutil"

	// "github.com/mit-dci/lit/btcutil/chaincfg/chainhash"
	"github.com/mit-dci/lit/btcutil/txscript"
	"github.com/mit-dci/lit/crypto/koblitz"
	dlc "github.com/mit-dci/lit/dlc"
	"github.com/mit-dci/lit/lnutil"
	"github.com/mit-dci/lit/portxo"

	// qln "github.com/mit-dci/lit/qln"
	"github.com/mit-dci/lit/sig64"
	"github.com/mit-dci/lit/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

var (
	oracle                                   *dlc.DlcOracle
	oraclePubKey, rPoint, OurFundMultisigPub [33]byte
	oracleSig                                [32]byte
	oracleValue                              int64
	pubKeyId2, pubKeyId3                     string
	dlcDbPath                                string
	hardcodedPKHbytes                        [20]byte
	UseContractFundMultisig                  uint32 = 2147483699
	UseContractPayoutBase                    uint32 = 2147483698
	addressNewByte                           [33]byte

	clientlnd1, clientLnd2 lnrpc.LightningClient
	ctx3, ctx2             context.Context
	wakket3                walletrpc.WalletKitClient

	mgr3, mgr2           *dlc.DlcManager
	path_macaroon_string string
	path_tls_string      string
	lnd2Serv, lnd1Serv   *lndclient.GrpcLndServices

	tarod2_newaddr   *tarorpc.Addr
	tarod2_rusd_addr string
	rusd_genesis_bsi []byte
)

const defaultTimeout = time.Second * 30

func handleError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func main() {
	// Get Oracle LND Client
	clientlnd1, _, _ = lnd1()
	ctx3 = context.Background()
	getInfoResp3, err := clientlnd1.GetInfo(ctx3, &lnrpc.GetInfoRequest{})
	pubKeyId3 = getInfoResp3.IdentityPubkey

	// Get Alisha's LND Client
	clientLnd2, err = lnd2()

	// getInfoResp2, err := clientLnd2.GetInfo(ctx2, &lnrpc.GetInfoRequest{})
	// pubKeyId2 = getInfoResp2.IdentityPubkey

	// Create the DLC manager that receives the contract
	mgr2, err = createManager("/home/opb/.lnd2/dlc.db")

	// Create the DLC manager for the oracle
	mgr3, err = createManager("/home/opb/.lnd1/dlc.db")

	// oracle3, err := createOracle()
	oracle3, oracleSig, oracleValue_string, err := create_Sats2USD_Oracle()
	handleError(err)

	oracleValue, err = strconv.ParseInt(oracleValue_string, 10, 64)
	// oracleValue = int64(oracleValueInt)
	handleError(err)
	fmt.Println("Oracle3 Value Int64: ", oracleValue)
	fmt.Printf("Oracle3 Index: %x\n", oracle3.Idx)

	// Create the contract and set its parameters
	fmt.Println("Creating the contract...")
	// contract3, err := createContract(oracle3.Idx)
	contract3, err := createContract_Sats2USD(oracle3.Idx, oracleValue)

	fmt.Println("Draft Contract3 Index: ", contract3.Idx)

	// Offer the contract
	fmt.Println("Offering the contract...")

	invoice, err := OfferDlc_Sats2Usd(true, contract3.Idx)
	handleError(err)
	// fmt.Println("Invoice Pay Request: ", invoice.PaymentRequest)

	// fmt.Printf("Contract3 Status: %x\n", contract3.Status)

	// Wait for the contract to be exchanged
	fmt.Println("Waiting for the contract to be exchanged...")
	time.Sleep(2 * time.Second)

	// Accept the contract
	fmt.Println("Accepting the contract...")

	contract3, err = ContractAccept_Sats2USD(invoice, contract3.Idx)
	handleError(err)

	active, err := isContractActive(contract3.Idx)
	handleError(err)
	// fmt.Println("Active Bool Status: ", active)
	if active {
		fmt.Println("Contract is active")
	}

	fmt.Println("Contract active. Generate 1 block on regtest and press enter")
	var input string
	fmt.Scanln(&input)

	// Setlle the contract
	fmt.Println("Settling the contract...")

	txSettle, txClaim, err := ContractSettle(contract3.Idx, oracleValue, oracleSig)
	handleError(err)

	// Print out the transactions
	fmt.Printf("Settle Tx: %x\n", txSettle)
	fmt.Printf("Claim Tx: %x\n", txClaim)

	// Conversion of Sats to rUSD successful
	fmt.Println("Conversion of Sats to rUSD successful")
}

func getBytesFromString(pubKey string) (pubArr [33]byte, err error) {
	parsedBytes, err := hex.DecodeString(pubKey)
	copy(pubArr[:], parsedBytes)

	return
}

func createManager(dlcDbPath string) (*dlc.DlcManager, error) {

	mgr, err := dlc.NewManager(dlcDbPath)

	return mgr, err
}

// Create The Oracle
func create_Sats2USD_Oracle() (*dlc.DlcOracle, [32]byte, string, error) {
	privateKey, err := oracle_getOrCreateKey()
	handleError(err)

	// Print out the public key for the oracle
	oraclePubKey = dlcoracle.PublicKeyFromPrivateKey(privateKey)
	fmt.Printf("Oracle public key: %x\n", oraclePubKey)

	// Generate a new one-time signing key
	privPoint, err := dlcoracle.GenerateOneTimeSigningKey()
	handleError(err)

	// Generate the public key to the one-time signing key (R-point) and print it out
	rPoint = dlcoracle.PublicKeyFromPrivateKey(privPoint)
	fmt.Printf("R-Point for next publication: %x\n", rPoint)

	satsamount := oracle_SatsToUsdAmount()
	satsamount_bytes, _ := oracle_getBytesFromString(satsamount)
	// satsamount_bytes := []byte(strconv.Itoa(satsamount))

	oracleSig, err := dlcoracle.ComputeSignature(privateKey, privPoint, satsamount_bytes)
	handleError(err)

	oracle, err = mgr3.AddOracle(oraclePubKey, "Sats2USD Oracle")
	handleError(err)

	return oracle, oracleSig, satsamount, err
}

// Get or Create the Oracle's Private Key
func oracle_getOrCreateKey() ([32]byte, error) {
	// Initialize the byte array that will hold the generated key
	var priv [32]byte

	// Check if the privatekey.hex file exists
	_, err := os.Stat("privatekey.hex")
	if err != nil {
		if os.IsNotExist(err) {
			// If not, generate a new private key by reading 32 random bytes
			rand.Read(priv[:])

			// Convert the key in to a hexadecimal format
			keyhex := fmt.Sprintf("%x\n", priv[:])

			// Save the hexadecimal value into the file
			err := ioutil.WriteFile("privatekey.hex", []byte(keyhex), 0600)

			if err != nil {
				// Unable the save the key file, return the error
				return priv, err
			}
		} else {
			// Some other error occurred while checking the file's
			// existence, return the error
			return priv, err
		}
	}

	// At this point, the file either existed or is created. Read the private key from the file
	keyhex, err := ioutil.ReadFile("privatekey.hex")
	if err != nil {
		// Unable to read the key file, return the error
		return priv, err
	}

	// Trim any whitespace from the file's contents
	keyhex = []byte(strings.TrimSpace(string(keyhex)))

	// Decode the hexadecimal format into a byte array
	key, err := hex.DecodeString(string(keyhex))
	if err != nil {
		// Unable to decode the hexadecimal format, return the error
		return priv, err
	}

	// Copy the variable-width byte array key into priv ([32]byte)
	copy(priv[:], key[:])

	// Return the key
	return priv, nil
}

// Get the current amount of Sats per USD
// TODO: setup an actual oracle locally that broadcasts the current value
// and a public key
func oracle_SatsToUsdAmount() string {
	c := colly.NewCollector()
	// amountofsats := []uint64{}
	amountofsats := []string{}

	c.OnHTML(".converter", func(e *colly.HTMLElement) {

		fmt.Println("Amount of Sats per USD: ", e.ChildText(".converter-title-amount"))
		// satsamount, _ := strconv.ParseUint(e.ChildText(".converter-title-amount"), 0, 64)
		// fmt.Println("Sats Amount uint64: ", satsamount)
		// amountofsats = append(amountofsats, satsamount)
		amountofsats = append(amountofsats, e.ChildText(".converter-title-amount"))
	})

	c.OnRequest(func(r *colly.Request) {
		// fmt.Println("Visiting", r.URL.String())
	})

	c.Visit("https://walletinvestor.com/converter/usd/satoshi/1")

	float_amountofsats, err := strconv.ParseFloat(amountofsats[0], 64)
	// fmt_amountofsats, err := fmt.Printf("%.0f \n", float_amountofsats)
	fmt_amountofsats := fmt.Sprintf("%.0f", float_amountofsats)
	handleError(err)
	fmt.Println("Sats Amount string: ", fmt_amountofsats)

	return fmt_amountofsats
}

// Convert the Sats amount to a byte array
func oracle_getBytesFromString(amountofsats_string string) (amount []byte, err error) {
	parsedBytes, err := hex.DecodeString(amountofsats_string)
	copy(amount[:], parsedBytes)

	return
}

// Create The Contract
// TODO: Set up the Oracle as a Data Feed (localhost) that broadcast a PubKey and the Sats amount
func createContract_Sats2USD(oracleIdx uint64, ov int64) (*lnutil.DlcContract, error) {
	// Create DLC Manager
	// mgr, err := createManager()

	// Create a new empty draft contract
	contract, err := mgr3.AddContract()
	handleError(err)
	fmt.Printf("Contract Index: %x\n", contract.Idx)

	// Configure the contract to use the oracle we need
	err = mgr3.SetContractOracle(contract.Idx, oracleIdx)
	handleError(err)

	// Set the settlement time to June 13, 2018 midnight UTC
	err = mgr3.SetContractSettlementTime(contract.Idx, 1686639600)
	handleError(err)

	// Set the coin type of the contract to Bitcoin Regtest
	err = mgr3.SetContractCoinType(contract.Idx, 257)
	handleError(err)

	// Configure the contract to use the R-point we need
	err = mgr3.SetContractRPoint(contract.Idx, rPoint)
	handleError(err)

	// Set the contract funding to the exchange rate of 1 USD to sats (oraclevalue)
	err = mgr3.SetContractFunding(contract.Idx, 0, ov)
	handleError(err)
	// fmt.Printf("Contract Funding: %x\n", ov)
	// fmt.Println("Contract Funding: ", ov)

	// Configure the contract division so that we get all the
	// funds
	err = mgr3.SetContractDivision(contract.Idx, oracleValue, 0)
	handleError(err)

	return contract, nil
}

// This function is used to check if the node is connected to the peer
// It can be called in the OfferDlc_Sats2USD function
func checkConnectedToPeer(ctx context.Context, lnd1 lnrpc.LightningClient, lnd2 lnrpc.LightningClient) (bool, error) {
	peers, err := lnd1.ListPeers(ctx, &lnrpc.ListPeersRequest{})
	if err != nil {
		// return false, fmt.Errorf(
		// 	"error listing %s's node (%v) peers: %v",
		// 	lnd1.ListAliases(), lnd1.NodeID, err,
		// )
		return false, fmt.Errorf(
			"error listing lnd1's node peers: %v",
			err)
	}

	ctx2 := context.Background()
	getInfoResp2, err := lnd2.GetInfo(ctx2, &lnrpc.GetInfoRequest{})

	for _, peer := range peers.Peers {
		if peer.PubKey == getInfoResp2.IdentityPubkey {
			return true, nil
		}
	}

	return false, nil
}

// Offer the contract to the peer
func OfferDlc_Sats2Usd(peer bool, cIdx uint64) (invoice *lnrpc.AddInvoiceResponse, err error) {
	c, err := mgr3.LoadContract(cIdx)
	handleError(err)

	if c.Status != lnutil.ContractStatusDraft {
		fmt.Println("You cannot offer a contract to someone that is not in draft stage")
	}

	// Keep this to check if the nodes are connected
	// ctxb := context.Background()
	// ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	// defer cancel()
	// nodesAreConnected, err := checkConnectedToPeer(ctxt, clientlnd1, clientLnd2)
	// if !nodesAreConnected {
	// 	return fmt.Errorf("Nodes are not connected, do that first")
	// }

	// fmt.Println("Oracle R-Point: ", c.OracleR)
	var nullBytes [33]byte
	// Check if everything's set
	if c.OracleA == nullBytes {
		fmt.Println("You need to set an oracle for the contract before offering it")
	}

	if c.OracleR == nullBytes {
		fmt.Println("You need to set an R-point for the contract before offering it")
	}

	if c.OracleTimestamp == 0 {
		fmt.Println("You need to set a settlement time for the contract before offering it")
	}

	if c.CoinType == dlc.COINTYPE_NOT_SET {
		fmt.Println("You need to set a coin type for the contract before offering it")
	}

	if c.Division == nil {
		fmt.Println("You need to set a payout division for the contract before offering it")
	}

	if c.OurFundingAmount+c.TheirFundingAmount == 0 {
		fmt.Println("You need to set a funding amount for the peers in contract before offering it")
	}

	// if nodesAreConnected {
	// 	c.PeerIdx = 1
	// }

	c.PeerIdx = 1

	// Keeping this for now
	// var kg portxo.KeyGen
	// kg.Depth = 5
	// kg.Step[0] = 44 | 1<<31
	// kg.Step[1] = c.CoinType | 1<<31
	// kg.Step[2] = UseContractFundMultisig
	// kg.Step[3] = c.PeerIdx | 1<<31
	// kg.Step[4] = uint32(c.Idx) | 1<<31

	c.OurFundMultisigPub, err = getBytesFromString(pubKeyId3)
	handleError(err)
	c.OurPayoutBase, err = getBytesFromString(pubKeyId2)
	handleError(err)

	// No need to fund the contract because this is a one way offer

	// Setting the status of the contract to OfferedByMe and saving it
	c.Status = lnutil.ContractStatusOfferedByMe
	err = mgr3.SaveContract(c)
	handleError(err)
	fmt.Println("Contract Mgr3 Status: ", c.Status)

	// Save the contract to the database in LND2 with mgr2 with Status OfferedToMe
	c.Status = lnutil.ContractStatusOfferedToMe
	err = mgr2.SaveContract(c)
	handleError(err)
	fmt.Println("Contract Mgr2 Status: ", c.Status)

	// LND2 creates an invoice for the amount of sats to be exhanged
	in := &lnrpc.Invoice{Value: oracleValue, Memo: "Sats to USD"}
	invoice, err = clientlnd1.AddInvoice(context.Background(), in)

	return invoice, err
}

func getContext() context.Context {
	shutdownInterceptor, err := signal.Intercept()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctxc, cancel := context.WithCancel(context.Background())
	go func() {
		<-shutdownInterceptor.ShutdownChannel()
		cancel()
	}()
	return ctxc
}

// Hardcoded PKH address so as to not have to create unused addresses in LND2
// Perhaps there's a better way to do this, open to suggestions
func lnd2HardcodedOurchangePKH() (hardcodedPKHbytes [20]byte) {
	parsedBytes, err := hex.DecodeString("6263727431713834726a6e6a7a6b6d33717271657239707a72306d36637761713768676574346c6677337967")
	if err != nil {
		panic(err.Error())
	}
	copy(hardcodedPKHbytes[:], parsedBytes)
	// fmt.Printf("Hardcoded PKH: %x \n", hardcodedPKHbytes)
	return hardcodedPKHbytes
}

// Accept the contract from the peer
func ContractAccept_Sats2USD(invoice *lnrpc.AddInvoiceResponse, cIdx uint64) (contract *lnutil.DlcContract, err error) {
	// nd2 := lnd2client
	// Get the contract from the database in LND2 with mgr2
	c, err := mgr2.LoadContract(cIdx)
	handleError(err)

	c.Status = lnutil.ContractStatusAccepting
	mgr2.SaveContract(c)

	c.OurFundMultisigPub, err = getBytesFromString(pubKeyId2)
	if err != nil {
		fmt.Printf("Error while getting multisig pubkey: %s", err.Error())
		c.Status = lnutil.ContractStatusError
		mgr2.SaveContract(c)
		// return
	}

	c.OurPayoutBase, err = getBytesFromString(pubKeyId3)
	if err != nil {
		fmt.Printf("Error while getting payoutbase: %s", err.Error())
		c.Status = lnutil.ContractStatusError
		mgr2.SaveContract(c)
		// return
	}

	ourPayoutPKHKey, err := getBytesFromString(pubKeyId3)
	if err != nil {
		fmt.Printf("Error while getting our payout pubkey: %s", err.Error())
		c.Status = lnutil.ContractStatusError
		mgr2.SaveContract(c)
		// return
	}
	copy(c.OurPayoutPKH[:], btcutil.Hash160(ourPayoutPKHKey[:]))

	c.Status = lnutil.ContractStatusAccepted

	// Save the contract in LND2
	mgr2.SaveContract(c)

	// Paying the invoice to the Orcale Node lnd1
	ctx2 = context.Background()
	payreq := invoice.PaymentRequest
	_, err = clientLnd2.SendPaymentSync(ctx2, &lnrpc.SendRequest{PaymentRequest: payreq})
	if err != nil {
		fmt.Printf("Error while paying the invoice: %s", err.Error())
		c.Status = lnutil.ContractStatusError
		mgr2.SaveContract(c)
		// return
	}

	// Verify that the payment was successful
	lnd1_invoices, err := clientlnd1.ListInvoices(ctx3, &lnrpc.ListInvoiceRequest{})
	for _, invoice := range lnd1_invoices.Invoices {
		if invoice.PaymentRequest == payreq {
			if invoice.State == lnrpc.Invoice_SETTLED {
				fmt.Println("Payment was successful")
			} else {
				fmt.Println("Payment was not successful")
			}
		}
	}

	// Saving the contract in lnd1 as Accepted
	mgr3.SaveContract(c)

	// Saving the contract in lnd1 as Active
	contract = c
	contract.Status = lnutil.ContractStatusActive
	fmt.Println("Contract Status: ", contract.Status)
	mgr3.SaveContract(contract)

	return contract, nil
}

// SignSettlementTx signs the given settlement tx based on the passed contract
// using the passed private key. Tx is modified in place.
func SignSettlementTx(c *lnutil.DlcContract, tx *wire.MsgTx,
	priv *koblitz.PrivateKey) ([64]byte, error) {

	var sig [64]byte
	// make hash cache
	hCache := txscript.NewTxSigHashes(tx)
	// fmt.Println("hCache", hCache)

	// generate script preimage for signing (ignore key order)
	pre, _, err := lnutil.FundTxScript(c.OurFundMultisigPub,
		c.TheirFundMultisigPub)

	if err != nil {
		return sig, err
	}
	// generate sig
	mySig, err := txscript.RawTxInWitnessSignature(
		tx, hCache, 0, c.TheirFundingAmount+c.OurFundingAmount,
		pre, txscript.SigHashAll, priv)

	if err != nil {
		return sig, err
	}
	// truncate sig (last byte is sighash type, always sighashAll)
	mySig = mySig[:len(mySig)-1]
	return sig64.SigCompress(mySig)
}

func isContractActive(contractIdx uint64) (bool, error) {
	// Fetch the contract from node 1
	contract, err := mgr3.LoadContract(contractIdx)
	if err != nil {
		return false, err
	}

	return (contract.Status == lnutil.ContractStatusActive), nil
}

func ContractSettle(cIdx uint64, oraclevalue int64, oracleSig [32]byte) ([32]byte, [32]byte, error) {
	tarod3_client, err := getTarod1()
	handleError(err)
	tctx3 := context.Background()
	assetslist, err := tarod3_client.ListAssets(tctx3, &tarorpc.ListAssetRequest{})
	handleError(err)
	for _, asset := range assetslist.Assets {
		if asset.AssetGenesis.Name == "rUSD" {
			rusd_genesis_bsi = assetslist.Assets[0].AssetGenesis.GenesisBootstrapInfo
			// fmt.Println("rUSD Genesis BSI: ", rusd_genesis_bsi)
		}
	}
	// rUSD Taro Asset ID
	rusd_assetId := assetslist.Assets[0].AssetGenesis.AssetId

	// Instantiate Alisha's Taro Client
	tarod2_client, err := getTarod2()
	handleError(err)
	tctx2 := context.Background()
	// Get rUSD Balance before settlement
	balreq := &tarorpc.ListBalancesRequest{GroupBy: &tarorpc.ListBalancesRequest_AssetId{AssetId: true}, AssetFilter: rusd_assetId}
	tarod2_getbalance, err := tarod2_client.ListBalances(tctx2, balreq)
	var balanceB4, balanceA5 int64

	for _, balance := range tarod2_getbalance.AssetBalances {
		balanceB4 = balance.Balance
		fmt.Println("rUSD Balance Before Settlement: ", balance.Balance)
	}

	req := &tarorpc.NewAddrRequest{GenesisBootstrapInfo: rusd_genesis_bsi, Amt: 1}

	tarod2_newaddr, err := tarod2_client.NewAddr(tctx2, req)
	handleError(err)
	// fmt.Println("Taro2 New address: ", tarod2_newaddr.Encoded)
	tarod2_rusd_addr = tarod2_newaddr.Encoded

	// Fetch the contract from node 1
	c, err := mgr3.LoadContract(cIdx)
	if err != nil {
		fmt.Println("Cannot load contract", err)
		return [32]byte{}, [32]byte{}, err
	}
	// fmt.Println("Loaded contract", c)

	c.Status = lnutil.ContractStatusSettling
	err = mgr3.SaveContract(c)
	if err != nil {
		fmt.Println("Cannot save contract", err)
		return [32]byte{}, [32]byte{}, err
	}

	d, err := c.GetDivision(oracleValue)
	if err != nil {
		fmt.Println("Cannot get division", err)
		return [32]byte{}, [32]byte{}, err
	}

	var kg portxo.KeyGen
	kg.Depth = 5
	kg.Step[0] = 44 | 1<<31
	kg.Step[1] = c.CoinType | 1<<31
	kg.Step[2] = UseContractFundMultisig
	kg.Step[3] = c.PeerIdx | 1<<31
	kg.Step[4] = uint32(c.Idx) | 1<<31

	priv, err := PrivGet(kg)
	if err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("SettleContract Could not get private key for contract %d", c.Idx)
	}
	// fmt.Printf("SettleContract priv %x\n", priv.Serialize())

	// fmt.Printf("SettleContract priv %x\n", priv)

	// Create the settlement transaction
	settleTx, err := lnutil.SettlementTx(c, *d, true)

	// fmt.Printf("SettleContract Tx Hash %x\n", settleTx.TxHash())

	mySig, err := SignSettlementTx(c, settleTx, priv)
	if err != nil {
		fmt.Printf("SettleContract SignSettlementTx err %s", err.Error())
		return [32]byte{}, [32]byte{}, err
	}

	// fmt.Println("SettleContract mySig", mySig)

	myBigSig := sig64.SigDecompress(mySig)

	theirSig, err := c.GetTheirSettlementSignature(oracleValue)
	theirBigSig := sig64.SigDecompress(theirSig)

	pre, swap, err := lnutil.FundTxScript(c.OurFundMultisigPub, c.TheirFundMultisigPub)
	if err != nil {
		fmt.Printf("SettleContract FundTxScript err %s", err.Error())
		return [32]byte{}, [32]byte{}, err
	}

	// swap if needed
	if swap {
		settleTx.TxIn[0].Witness = SpendMultiSigWitStack(pre, theirBigSig, myBigSig)
	} else {
		settleTx.TxIn[0].Witness = SpendMultiSigWitStack(pre, myBigSig, theirBigSig)
	}

	// TODO: Claim the contract settlement output back to our wallet - otherwise the peer can claim it after locktime.
	txClaim := wire.NewMsgTx()
	txClaim.Version = 2

	settleOutpoint := wire.OutPoint{Hash: settleTx.TxHash(), Index: 0}
	txClaim.AddTxIn(wire.NewTxIn(&settleOutpoint, nil, nil))

	// fmt.Println("SettleContract txClaim", txClaim)

	// TODO: get the address from the LND2 wallet programatically
	addr := lnd2HardcodedOurchangePKH()

	txClaim.AddTxOut(wire.NewTxOut(d.ValueOurs-1000, lnutil.DirectWPKHScriptFromPKH(addr))) // todo calc fee - fee is double here because the contract output already had the fee deducted in the settlement TX

	kg.Step[2] = UseContractPayoutBase
	privSpend, _ := PrivGet(kg)

	pubSpend, _ := PubGet(kg)
	privOracle, pubOracle := koblitz.PrivKeyFromBytes(koblitz.S256(), oracleSig[:])
	privContractOutput := lnutil.CombinePrivateKeys(privSpend, privOracle)

	var pubOracleBytes [33]byte
	copy(pubOracleBytes[:], pubOracle.SerializeCompressed())
	var pubSpendBytes [33]byte
	copy(pubSpendBytes[:], pubSpend.SerializeCompressed())

	settleScript := lnutil.DlcCommitScript(c.OurPayoutBase, pubOracleBytes, c.TheirPayoutBase, 5)
	err = TxClaimSign(txClaim, settleTx.TxOut[0].Value, settleScript, privContractOutput, false)
	if err != nil {
		fmt.Printf("SettleContract SignClaimTx err %s", err.Error())
		return [32]byte{}, [32]byte{}, err
	}

	// Taro3 send the 1 rUSD to Taro2/LND2
	_, err = tarod3_client.SendAsset(ctx2, &tarorpc.SendAssetRequest{TaroAddr: tarod2_newaddr.Encoded})
	// fmt.Println("Taro3 Send Asset Response: ", tarod3_sendasset.String())

	// Verify that Taro2 has received the 1 rUSD
	fmt.Println("Contract Settle. Generate 6 blocks on regtest and press enter")
	var input string
	fmt.Scanln(&input)
	// Verify that Taro2 has received the 1 rUSD
	// var total_tarod2_rusd_bal int64
	// balreq := &tarorpc.ListBalancesRequest{GroupBy: &tarorpc.ListBalancesRequest_AssetId{AssetId: true}, AssetFilter: rusd_assetId}
	tarod2_getbalance, err = tarod2_client.ListBalances(tctx2, balreq)

	for _, balance := range tarod2_getbalance.AssetBalances {
		balanceA5 = balance.Balance
		// Verify the rUSD balance has increased by 1
		if balanceA5 > balanceB4 {
			fmt.Println("rUSD Balance After Settlement: ", balanceA5)
		}
	}

	c.Status = lnutil.ContractStatusClosed
	err = mgr3.SaveContract(c)
	if err != nil {
		fmt.Println("Cannot save contract", err)
		return [32]byte{}, [32]byte{}, err
	}

	// return [32]byte{}, [32]byte{}, nil
	return settleTx.TxHash(), txClaim.TxHash(), nil

}

// Get the Oracle's Taro Client
func getTarod1() (tarorpc.TaroClient, error) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
	}
	// fmt.Println("The user home directory: " + usr.HomeDir)
	tlsCertPath := path.Join(usr.HomeDir, ".taro3/tls.cert")

	macaroonPath := path.Join(usr.HomeDir, ".taro3/data/regtest/admin.macaroon")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		fmt.Println("Cannot read macaroon file", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Println("Cannot unmarshal macaroon", err)
	}

	credsmac, err := macaroons.NewMacaroonCredential(mac)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(credsmac),
	}

	conn, err := grpc.Dial("localhost:10030", opts...)

	client_tarod3 := tarorpc.NewTaroClient(conn)

	return client_tarod3, nil
}

// Get Alisha's Taro Client
func getTarod2() (tarorpc.TaroClient, error) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
	}
	// fmt.Println("The user home directory: " + usr.HomeDir)
	tlsCertPath := path.Join(usr.HomeDir, ".taro2/tls.cert")

	macaroonPath := path.Join(usr.HomeDir, ".taro2/data/regtest/admin.macaroon")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		fmt.Println("Cannot read macaroon file", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Println("Cannot unmarshal macaroon", err)
	}

	credsmac, err := macaroons.NewMacaroonCredential(mac)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(credsmac),
	}

	conn, err := grpc.Dial("localhost:11029", opts...)

	client_tarod2 := tarorpc.NewTaroClient(conn)

	return client_tarod2, nil
}

func printJSON(resp interface{}) {
	b, err := json.Marshal(resp)
	if err != nil {
		handleError(err)
	}

	var out bytes.Buffer
	json.Indent(&out, b, "", "\t")
	out.WriteString("\n")
	out.WriteTo(os.Stdout)
}

// the scriptsig to put on a P2SH input.  Sigs need to be in order!
func SpendMultiSigWitStack(pre, sigA, sigB []byte) [][]byte {

	witStack := make([][]byte, 4)

	witStack[0] = nil // it's not an OP_0 !!!! argh!
	witStack[1] = sigA
	witStack[2] = sigB
	witStack[3] = pre

	return witStack
}
func TxClaimSign(claimTx *wire.MsgTx, value int64, pre []byte,
	priv *koblitz.PrivateKey, timeout bool) error {

	// make hash cache
	hCache := txscript.NewTxSigHashes(claimTx)

	// generate sig
	mySig, err := txscript.RawTxInWitnessSignature(
		claimTx, hCache, 0, value, pre, txscript.SigHashAll, priv)
	if err != nil {
		return err
	}

	witStash := make([][]byte, 3)
	witStash[0] = mySig
	if timeout {
		witStash[1] = nil
	} else {
		witStash[1] = []byte{0x01}
	}
	witStash[2] = pre
	claimTx.TxIn[0].Witness = witStash
	return nil

}

func PrivGet(kg portxo.KeyGen) (*koblitz.PrivateKey, error) {
	//
	kpv := kg.Bytes()
	keypriv, keypub := koblitz.PrivKeyFromBytes(koblitz.S256(), kpv)
	// fmt.Println("PrivGet keypub", keypub)
	_ = keypub
	return keypriv, nil
}

func PubGet(kg portxo.KeyGen) (*koblitz.PublicKey, error) {
	//
	kpv := kg.Bytes()
	_, keypub := koblitz.PrivKeyFromBytes(koblitz.S256(), kpv)
	// fmt.Println("PrivGet keypub", keypub)

	return keypub, nil
}

// Get Oracle's LND Client
func lnd1() (lnrpc.LightningClient, walletrpc.WalletKitClient, error) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
		// return err
	}
	// fmt.Println("The user home directory: " + usr.HomeDir)
	tlsCertPath := path.Join(usr.HomeDir, ".lnd1/tls.cert")
	macaroonPath := path.Join(usr.HomeDir, ".lnd1/data/chain/bitcoin/regtest/admin.macaroon")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		// return
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		fmt.Println("Cannot read macaroon file", err)
		// return
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Println("Cannot unmarshal macaroon", err)
		// return
	}

	credsmac, err := macaroons.NewMacaroonCredential(mac)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		// grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
		grpc.WithPerRPCCredentials(credsmac),
	}

	conn, err := grpc.Dial("localhost:10009", opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		// return
	}
	lnd1 := lnrpc.NewLightningClient(conn)
	wallet3 := walletrpc.NewWalletKitClient(conn)

	return lnd1, wallet3, nil
}

// Get Alisha's LND Client
func lnd2() (lnrpc.LightningClient, error) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
		// return err
	}
	// fmt.Println("The user home directory: " + usr.HomeDir)
	tlsCertPath := path.Join(usr.HomeDir, ".lnd2/tls.cert")
	macaroonPath := path.Join(usr.HomeDir, ".lnd2/data/chain/bitcoin/regtest/admin.macaroon")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		// return
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
	if err != nil {
		fmt.Println("Cannot read macaroon file", err)
		// return
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Println("Cannot unmarshal macaroon", err)
		// return
	}

	credsmac, err := macaroons.NewMacaroonCredential(mac)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		// grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
		grpc.WithPerRPCCredentials(credsmac),
	}

	conn, err := grpc.Dial("localhost:11009", opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		// return
	}
	lnd2 := lnrpc.NewLightningClient(conn)

	return lnd2, nil
}
