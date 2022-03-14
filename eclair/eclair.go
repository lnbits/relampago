package eclair

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fiatjaf/eclair-go"
	rp "github.com/lnbits/relampago"
)

type Params struct {
	Host     string
	Password string
}

type EclairWallet struct {
	Params

	client *eclair.Client
}

func Start(params Params) (*EclairWallet, error) {
	if !strings.HasPrefix(params.Host, "http") {
		params.Host = "http://" + params.Host
	}

	e := &EclairWallet{
		Params: params,
		client: &eclair.Client{
			Host:     params.Host,
			Password: params.Password,
		},
	}

	if ws, err := e.client.Websocket(); err != nil {
		panic(err)
	} else {
		go func() {
			for message := range ws {
				typ := message.Get("type").String()
				if typ == "channel-state-changed" {
					continue
				}

				log.Printf("[%s] %s", typ, message.String())
			}
		}()
	}

	return e, nil
}

// Compile time check to ensure that EclairWallet fully implements rp.Wallet
var _ rp.Wallet = (*EclairWallet)(nil)

func (e *EclairWallet) Kind() string {
	return "eclair"
}

func (e *EclairWallet) GetInfo() (rp.WalletInfo, error) {
	res, err := e.client.Call("channels", map[string]interface{}{})
	if err != nil {
		return rp.WalletInfo{}, fmt.Errorf("error calling 'channels': %w", err)
	}

	var balance int64
	for _, channel := range res.Array() {
		balance += channel.Get("data.commitments.localCommit.spec.toLocal").Int()
	}

	return rp.WalletInfo{Balance: balance}, nil
}

func (e *EclairWallet) CreateInvoice(params rp.InvoiceParams) (rp.InvoiceData, error) {
	args := map[string]interface{}{
		"amountMsat": params.Msatoshi,
	}

	if params.DescriptionHash == nil {
		args["description"] = params.Description
	} else {
		args["descriptionHash"] = hex.EncodeToString(params.DescriptionHash)
	}

	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return rp.InvoiceData{}, fmt.Errorf("failed to make random preimage: %w", err)
	} else {
		args["paymentPreimage"] = hex.EncodeToString(preimage)
	}

	if params.Expiry != nil {
		args["expireIn"] = *params.Expiry / time.Second
	}

	inv, err := e.client.Call("createinvoice", args)
	if err != nil {
		return rp.InvoiceData{}, fmt.Errorf("'createinvoice' call failed: %w", err)
	}
	return rp.InvoiceData{
		Invoice:    inv.Get("serialized").String(),
		Preimage:   args["paymentPreimage"].(string),
		CheckingID: inv.Get("paymentHash").String(),
	}, nil
}

func (e *EclairWallet) GetInvoiceStatus(checkingID string) (rp.InvoiceStatus, error) {
	res, err := e.client.Call("getreceivedinfo", map[string]interface{}{
		"paymentHash": checkingID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "Not found") {
			return rp.InvoiceStatus{
				CheckingID: checkingID,
				Exists:     false,
			}, nil
		}

		return rp.InvoiceStatus{},
			fmt.Errorf("error on 'getreceivedinfo' hash=%s: %w", checkingID, err)
	}

	return rp.InvoiceStatus{
		CheckingID:       checkingID,
		Exists:           true,
		Paid:             res.Get("status.type").String() == "received",
		MSatoshiReceived: res.Get("status.amount").Int(),
	}, nil
}

func (e *EclairWallet) PaidInvoicesStream() (<-chan rp.InvoiceStatus, error) {
	listener := make(chan rp.InvoiceStatus)
	return listener, nil
}

func (e *EclairWallet) MakePayment(params rp.PaymentParams) (rp.PaymentData, error) {
	args := map[string]interface{}{
		"invoice":   params.Invoice,
		"blocking":  false,
		"maxFeePct": 1,
	}
	if params.CustomAmount != 0 {
		args["amountMsat"] = params.CustomAmount
	}

	id, err := e.client.Call("payinvoice", args)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("error calling 'payinvoice' with '%s': %w",
			params.Invoice, err)
	}

	return rp.PaymentData{
		CheckingID: id.Value().(string),
	}, nil
}

func (e *EclairWallet) GetPaymentStatus(checkingID string) (rp.PaymentStatus, error) {
	res, err := e.client.Call("getsentinfo", map[string]interface{}{
		"id": checkingID,
	})
	if err != nil {
		return rp.PaymentStatus{},
			fmt.Errorf("error getting payment %s: %w", checkingID, err)
	}

	if res.Get("#").Int() == 0 {
		return rp.PaymentStatus{
			CheckingID: checkingID,
			Status:     rp.NeverTried,
		}, nil
	} else {
		for _, attempt := range res.Array() {
			status := attempt.Get("status")

			switch status.Get("type").String() {
			case "sent":
				return rp.PaymentStatus{
					CheckingID: checkingID,
					Status:     rp.Complete,
					FeePaid:    status.Get("feesPaid").Int(),
					Preimage:   status.Get("paymentPreimage").String(),
				}, nil
			case "pending":
				return rp.PaymentStatus{
					CheckingID: checkingID,
					Status:     rp.Pending,
				}, nil
			default:
				// what is this?
				return rp.PaymentStatus{
					CheckingID: checkingID,
					Status:     rp.Unknown,
				}, nil
			case "failed":
				// this one failed, but keep checking the others
				continue
			}
		}

		// if we reached here that's because all attempts are failed
		return rp.PaymentStatus{
			CheckingID: checkingID,
			Status:     rp.Failed,
		}, nil
	}
}

func (e *EclairWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	listener := make(chan rp.PaymentStatus)
	return listener, nil
}
