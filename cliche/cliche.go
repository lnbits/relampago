package cliche

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	clichelib "github.com/fiatjaf/go-cliche"
	rp "github.com/lnbits/relampago"
)

type Params struct {
	JARPath    string
	BinaryPath string
	DataDir    string
}

type ClicheWallet struct {
	control *clichelib.Control

	invoiceStatusListeners []chan rp.InvoiceStatus
	paymentStatusListeners []chan rp.PaymentStatus
}

func Start(params Params) (*ClicheWallet, error) {
	e := &ClicheWallet{
		control: &clichelib.Control{
			JARPath:    params.JARPath,
			BinaryPath: params.BinaryPath,
			DataDir:    params.DataDir,
		},
	}

	if err := e.control.Start(); err != nil {
		return nil, err
	}

	go func() {
		for event := range e.control.PaymentSuccesses {
			for _, listener := range e.paymentStatusListeners {
				listener <- rp.PaymentStatus{
					CheckingID: event.PaymentHash,
					Status:     rp.Complete,
					FeePaid:    event.FeeMsatoshi,
					Preimage:   event.Preimage,
				}
			}
		}
	}()

	go func() {
		for event := range e.control.PaymentFailures {
			for _, listener := range e.paymentStatusListeners {
				listener <- rp.PaymentStatus{
					CheckingID: event.PaymentHash,
					Status:     rp.Failed,
				}
			}
		}
	}()

	go func() {
		for event := range e.control.IncomingPayments {
			for _, listener := range e.invoiceStatusListeners {
				listener <- rp.InvoiceStatus{
					CheckingID:       event.PaymentHash,
					Exists:           true,
					Paid:             true,
					MSatoshiReceived: event.Msatoshi,
				}
			}
		}
	}()

	return e, nil
}

// Compile time check to ensure that ClicheWallet fully implements rp.Wallet
var _ rp.Wallet = (*ClicheWallet)(nil)

func (e *ClicheWallet) Kind() string {
	return "eclair"
}

func (e *ClicheWallet) GetInfo() (rp.WalletInfo, error) {
	info, err := e.control.GetInfo()
	if err != nil {
		return rp.WalletInfo{}, fmt.Errorf("error calling 'get-info': %w", err)
	}

	var balance int64
	for _, channel := range info.Channels {
		balance += int64(channel.Balance)
	}

	return rp.WalletInfo{Balance: balance}, nil
}

func (e *ClicheWallet) CreateInvoice(params rp.InvoiceParams) (rp.InvoiceData, error) {
	preimageB := make([]byte, 32)
	if _, err := rand.Read(preimageB); err != nil {
		return rp.InvoiceData{},
			fmt.Errorf("failed to generate random preimage: %w", err)
	}
	preimage := hex.EncodeToString(preimageB)

	inv, err := e.control.CreateInvoice(clichelib.CreateInvoiceParams{
		Msatoshi:        params.Msatoshi,
		Description:     params.Description,
		DescriptionHash: hex.EncodeToString(params.DescriptionHash),
		Preimage:        preimage,
	})
	if err != nil {
		return rp.InvoiceData{}, fmt.Errorf("'create-invoice' call failed: %w", err)
	}
	return rp.InvoiceData{
		Invoice:    inv.Invoice,
		Preimage:   preimage,
		CheckingID: inv.PaymentHash,
	}, nil
}

func (e *ClicheWallet) GetInvoiceStatus(checkingID string) (rp.InvoiceStatus, error) {
	info, err := e.control.CheckPayment(checkingID)
	if err != nil {
		if strings.Contains(err.Error(), "couldn't get payment") {
			return rp.InvoiceStatus{
				CheckingID: checkingID,
				Exists:     false,
			}, nil
		}

		return rp.InvoiceStatus{},
			fmt.Errorf("error on 'check-payment' hash=%s: %w", checkingID, err)
	}

	if !info.IsIncoming {
		// this is actually a payment we sent
		return rp.InvoiceStatus{
			CheckingID: checkingID,
			Exists:     false,
		}, nil
	}

	return rp.InvoiceStatus{
		CheckingID:       checkingID,
		Exists:           true,
		Paid:             info.Status == "complete",
		MSatoshiReceived: info.Msatoshi,
	}, nil
}

func (e *ClicheWallet) PaidInvoicesStream() (<-chan rp.InvoiceStatus, error) {
	listener := make(chan rp.InvoiceStatus)
	e.invoiceStatusListeners = append(e.invoiceStatusListeners, listener)
	return listener, nil
}

func (e *ClicheWallet) MakePayment(params rp.PaymentParams) (rp.PaymentData, error) {
	resp, err := e.control.PayInvoice(clichelib.PayInvoiceParams{
		Invoice:  params.Invoice,
		Msatoshi: params.CustomAmount,
	})
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("error calling 'pay-invoice' with '%s': %w",
			params.Invoice, err)
	}

	return rp.PaymentData{
		CheckingID: resp.PaymentHash,
	}, nil
}

func (e *ClicheWallet) GetPaymentStatus(checkingID string) (rp.PaymentStatus, error) {
	info, err := e.control.CheckPayment(checkingID)
	if err != nil {
		if strings.Contains(err.Error(), "couldn't get payment") {
			return rp.PaymentStatus{
				CheckingID: checkingID,
				Status:     rp.NeverTried,
			}, nil
		}

		return rp.PaymentStatus{},
			fmt.Errorf("error on 'check-payment' hash=%s: %w", checkingID, err)
	}

	if info.IsIncoming {
		// this is actually a payment we received
		return rp.PaymentStatus{
			CheckingID: checkingID,
			Status:     rp.NeverTried,
		}, nil
	}

	status := rp.Unknown
	switch info.Status {
	case "initial":
		status = rp.Pending
	case "pending":
		status = rp.Pending
	case "failed":
		status = rp.Failed
	case "complete":
		status = rp.Complete
	}

	return rp.PaymentStatus{
		CheckingID: checkingID,
		Status:     status,
		FeePaid:    info.FeeMsatoshi,
		Preimage:   info.Preimage,
	}, nil
}

func (e *ClicheWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	listener := make(chan rp.PaymentStatus)
	e.paymentStatusListeners = append(e.paymentStatusListeners, listener)
	return listener, nil
}
