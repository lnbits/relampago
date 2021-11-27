package sparko

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	decodepay "github.com/fiatjaf/ln-decodepay"
	rp "github.com/fiatjaf/relampago"
	sse "github.com/r3labs/sse/v2"
	"github.com/tidwall/gjson"
)

type Params struct {
	Host string
	Key  string

	InvoiceLabelPrefix string // optional, defaults to 'relampago'
}

type SparkoWallet struct {
	Params
	client *lightning.Client

	invoiceStatusListeners []chan rp.InvoiceStatus
	paymentStatusListeners []chan rp.PaymentStatus
}

func Start(params Params) *SparkoWallet {
	if !strings.HasPrefix(params.Host, "http") {
		params.Host = "http://" + params.Host
	}
	if strings.HasSuffix(params.Host, "/rpc") {
		params.Host = params.Host[0 : len(params.Host)-4]
	}

	spark := &lightning.Client{
		SparkURL:    params.Host + "/rpc",
		SparkToken:  params.Key,
		CallTimeout: time.Second * 15,
	}

	s := &SparkoWallet{
		Params: params,
		client: spark,
	}

	sseClient := sse.NewClient(params.Host + "/stream?access-key=" + params.Key)
	go sseClient.Subscribe("", func(ev *sse.Event) {
		data := gjson.ParseBytes(ev.Data)
		switch string(ev.Event) {
		case "sendpay_success":
			success := data.Get("sendpay_success")
			for _, listener := range s.paymentStatusListeners {
				listener <- rp.PaymentStatus{
					CheckingID: success.Get("payment_hash").String(),
					Status:     rp.Complete,
					FeePaid:    success.Get("msatoshi_sent").Int() - success.Get("msatoshi").Int(),
					Preimage:   success.Get("payment_preimage").String(),
				}
			}
		case "sendpay_failure":
			hash := data.Get("sendpay_failure.data.payment_hash").String()
			status, err := s.GetPaymentStatus(hash)
			if err != nil {
				return
			}

			for _, listener := range s.paymentStatusListeners {
				listener <- status
			}
		case "invoice_payment":
			label := data.Get("invoice_payment.label").String()
			status, err := s.GetInvoiceStatus(label)
			if err != nil {
				return
			}

			for _, listener := range s.invoiceStatusListeners {
				listener <- status
			}
		}
	})

	return s
}

func (s *SparkoWallet) GetInfo() (rp.WalletInfo, error) {
	res, err := s.client.Call("listfunds")
	if err != nil {
		return rp.WalletInfo{}, fmt.Errorf("error calling listfunds: %w", err)
	}

	var balance int64
	for _, channel := range res.Get("channels").Array() {
		balance += channel.Get("channel_sat").Int()
	}

	return rp.WalletInfo{balance}, nil
}

func (s *SparkoWallet) CreateInvoice(params rp.InvoiceParams) (rp.InvoiceData, error) {
	var (
		method string
		args   = make(map[string]interface{})
	)

	args["msatoshi"] = params.Msatoshi

	if params.DescriptionHash == nil {
		method = "invoice"
		args["description"] = params.Description
	} else {
		method = "invoicewithdescriptionhash"
		args["description_hash"] = hex.EncodeToString(params.DescriptionHash)
	}

	labelPrefix := s.InvoiceLabelPrefix
	if labelPrefix == "" {
		labelPrefix = "relampago"
	}
	args["label"] = labelPrefix + "/" + strconv.FormatInt(time.Now().Unix(), 16)

	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return rp.InvoiceData{}, fmt.Errorf("failed to make random preimage: %w", err)
	} else {
		args["preimage"] = hex.EncodeToString(preimage)
	}

	if params.Expiry != nil {
		args["expiry"] = *params.Expiry / time.Second
	}

	inv, err := s.client.Call(method, args)
	if err != nil {
		return rp.InvoiceData{}, fmt.Errorf("%s call failed: %w", method, err)
	}
	return rp.InvoiceData{
		Invoice:    inv.Get("bolt11").String(),
		Preimage:   args["preimage"].(string),
		CheckingID: args["label"].(string),
	}, nil
}

func (s *SparkoWallet) GetInvoiceStatus(checkingID string) (rp.InvoiceStatus, error) {
	res, err := s.client.Call("listinvoices", map[string]interface{}{"label": checkingID})
	if err != nil {
		return rp.InvoiceStatus{}, fmt.Errorf("error getting invoice label=%s: %w", checkingID, err)
	}

	return rp.InvoiceStatus{
		CheckingID:       checkingID,
		Exists:           res.Get("invoices.#").Int() == 1,
		Paid:             res.Get("invoices.0.status").String() == "paid",
		MSatoshiReceived: res.Get("invoices.0.msatoshi_received").Int(),
	}, nil
}

func (s *SparkoWallet) PaidInvoicesStream() (<-chan rp.InvoiceStatus, error) {
	listener := make(chan rp.InvoiceStatus)
	s.invoiceStatusListeners = append(s.invoiceStatusListeners, listener)
	return listener, nil
}

func (s *SparkoWallet) MakePayment(params rp.PaymentParams) (rp.PaymentData, error) {
	inv, err := decodepay.Decodepay(params.Invoice)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("failed to decode invoice '%s': %w", params.Invoice, err)
	}

	args := map[string]interface{}{
		"bolt11": params.Invoice,
	}
	if params.CustomAmount != 0 {
		args["msatoshi"] = params.CustomAmount
	}
	go func() {
		// I think we need some time here just so the caller can update their DB with
		// the checkingID we will return
		time.Sleep(500 * time.Millisecond)
		s.client.CallWithCustomTimeout(time.Second*1, "pay", args)
	}()

	return rp.PaymentData{
		CheckingID: inv.PaymentHash,
	}, nil
}

func (s *SparkoWallet) GetPaymentStatus(checkingID string) (rp.PaymentStatus, error) {
	res, err := s.client.Call("listpays", map[string]interface{}{
		"payment_hash": checkingID,
	})
	if err != nil {
		return rp.PaymentStatus{}, fmt.Errorf("error getting payment %s: %w", checkingID, err)
	}

	status := rp.PaymentStatus{CheckingID: checkingID}

	switch res.Get("pays.0.status").String() {
	case "complete":
		status.Status = rp.Complete
		needed, _ := strconv.ParseInt(res.Get("pays.0.amount_msat").String(), 10, 64)
		sent, _ := strconv.ParseInt(res.Get("pays.0.amount_sent_msat").String(), 10, 64)
		status.FeePaid = sent - needed
		status.Preimage = res.Get("pays.0.preimage").String()
	case "failed":
		status.Status = rp.Failed
	case "pending":
		status.Status = rp.Pending
	}
	if res.Get("pays.#").Int() == 0 {
		status.Status = rp.NeverTried
	}

	return status, nil
}

func (s *SparkoWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	listener := make(chan rp.PaymentStatus)
	s.paymentStatusListeners = append(s.paymentStatusListeners, listener)
	return listener, nil
}
