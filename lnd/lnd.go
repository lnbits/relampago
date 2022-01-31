package lnd

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"time"

	decodepay "github.com/fiatjaf/ln-decodepay"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lightningnetwork/lnd/record"
	rp "github.com/lnbits/relampago"
	"github.com/prometheus/common/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	macaroon "gopkg.in/macaroon.v2"
)

var PaymentPollInterval = 30 * time.Second

type Params struct {
	Host         string
	CertPath     string
	MacaroonPath string
}

type LndWallet struct {
	Params

	Conn      *grpc.ClientConn
	Lightning lnrpc.LightningClient
	Router    routerrpc.RouterClient

	invoiceStatusListeners []chan rp.InvoiceStatus
	paymentStatusListeners []chan rp.PaymentStatus
}

func Start(params Params) (*LndWallet, error) {
	var dialOpts []grpc.DialOption

	// TLS
	tls, err := credentials.NewClientTLSFromFile(params.CertPath, "")
	if err != nil {
		return nil, err
	}
	dialOpts = append(dialOpts, grpc.WithTransportCredentials(tls))

	// Macaroon Auth
	macBytes, err := ioutil.ReadFile(params.MacaroonPath)
	if err != nil {
		return nil, err
	}
	m := &macaroon.Macaroon{}
	err = m.UnmarshalBinary(macBytes)
	if err != nil {
		return nil, err
	}
	creds, err := macaroons.NewMacaroonCredential(m)
	if err != nil {
		return nil, err
	}
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(creds))
	dialOpts = append(dialOpts, grpc.WithBlock())

	// Connect
	conn, err := grpc.Dial(params.Host, dialOpts...)
	if err != nil {
		return nil, err
	}
	ln := lnrpc.NewLightningClient(conn)
	router := routerrpc.NewRouterClient(conn)

	l := &LndWallet{
		Params:    params,
		Conn:      conn,
		Lightning: ln,
		Router:    router,
	}

	go l.startPaymentsStream()
	go l.startInvoicesStream()

	return l, nil
}

// Compile time check to ensure that LndWallet fully implements rp.Wallet
var _ rp.Wallet = (*LndWallet)(nil)

func (l *LndWallet) GetInfo() (rp.WalletInfo, error) {
	res, err := l.Lightning.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		return rp.WalletInfo{}, fmt.Errorf("error calling ChannelBalance: %w", err)
	}

	info, err := l.Lightning.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return rp.WalletInfo{}, fmt.Errorf("error calling GetInfo: %w", err)
	}

	return rp.WalletInfo{
		Balance: int64(res.LocalBalance.Sat),
		Pubkey:  info.IdentityPubkey,
	}, nil
}

func (l *LndWallet) CreateInvoice(params rp.InvoiceParams) (rp.InvoiceData, error) {
	invoice, err := l.Lightning.AddInvoice(context.Background(), &lnrpc.Invoice{
		Memo:            params.Description,
		DescriptionHash: params.DescriptionHash,
		ValueMsat:       params.Msatoshi,
		Expiry:          int64(params.Expiry.Seconds()),
	})
	if err != nil {
		return rp.InvoiceData{}, fmt.Errorf("error calling AddInvoice: %w", err)
	}

	// LookupInvoice to get the preimage since AddInvoice only returns the hash
	res, err := l.Lightning.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHash: invoice.RHash})
	if err != nil {
		return rp.InvoiceData{}, fmt.Errorf("error calling LookupInvoice: %w", err)
	}
	return rp.InvoiceData{
		CheckingID: hex.EncodeToString(res.RHash),
		Preimage:   hex.EncodeToString(res.RPreimage),
		Invoice:    res.PaymentRequest,
	}, nil
}

func (l *LndWallet) GetInvoiceStatus(checkingID string) (rp.InvoiceStatus, error) {
	rHash, err := hex.DecodeString(checkingID)
	if err != nil {
		return rp.InvoiceStatus{}, fmt.Errorf("invalid checkingID: %w", err)
	}
	res, err := l.Lightning.LookupInvoice(context.Background(), &lnrpc.PaymentHash{RHash: rHash})
	if err != nil || res == nil {
		return rp.InvoiceStatus{
			CheckingID:       checkingID,
			Exists:           false,
			Paid:             false,
			MSatoshiReceived: 0,
		}, nil
	}
	return rp.InvoiceStatus{
		CheckingID:       checkingID,
		Exists:           true,
		Paid:             res.State == lnrpc.Invoice_SETTLED,
		MSatoshiReceived: res.AmtPaidMsat,
	}, nil
}

func (l *LndWallet) MakePayment(params rp.PaymentParams) (rp.PaymentData, error) {
	inv, err := decodepay.Decodepay(params.Invoice)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("failed to decode invoice '%s': %w", params.Invoice, err)
	}

	req := &routerrpc.SendPaymentRequest{
		PaymentRequest: params.Invoice,
		TimeoutSeconds: 300,
		FeeLimitSat:    math.MaxInt64,
	}
	if params.CustomAmount != 0 {
		req.AmtMsat = params.CustomAmount
	}

	_, err = l.Router.SendPaymentV2(context.Background(), req)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("error calling SendPaymentV2: %w", err)
	}

	// track this so it can emit payment notifications
	go l.trackOutgoingPayment(inv.PaymentHash)

	return rp.PaymentData{
		CheckingID: inv.PaymentHash,
	}, nil
}

func (l *LndWallet) Keysend(params rp.KeysendParams) (rp.PaymentData, error) {
	if params.Amount <= 0 {
		return rp.PaymentData{}, fmt.Errorf("amount must be greater than zero")
	}

	dest, err := hex.DecodeString(params.Dest)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("failed to decode dest: %w", err)
	}

	paymentHash, err := hex.DecodeString(params.PaymentHash)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("failed to decode hash: %w", err)
	}

	if params.DestCustomRecords == nil {
		params.DestCustomRecords = map[uint64][]byte{}
	}

	preimage, err := hex.DecodeString(params.Preimage)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("failed to decode preimage: %w", err)
	}
	params.DestCustomRecords[record.KeySendType] = preimage

	req := &routerrpc.SendPaymentRequest{
		Dest:              dest,
		AmtMsat:           params.Amount,
		PaymentHash:       paymentHash,
		DestFeatures:      []lnrpc.FeatureBit{},
		DestCustomRecords: params.DestCustomRecords,
		TimeoutSeconds:    300,
		FinalCltvDelta:    40,
		FeeLimitSat:       math.MaxInt64,
	}

	_, err = l.Router.SendPaymentV2(context.Background(), req)
	if err != nil {
		return rp.PaymentData{}, fmt.Errorf("error calling SendPaymentV2: %w", err)
	}

	// track this so it can emit payment notifications
	go l.trackOutgoingPayment(string(params.PaymentHash))

	return rp.PaymentData{
		CheckingID: string(params.PaymentHash),
	}, nil
}

func (l *LndWallet) GetPaymentStatus(checkingID string) (rp.PaymentStatus, error) {
	paymentHash, err := hex.DecodeString(checkingID)
	if err != nil {
		return rp.PaymentStatus{}, fmt.Errorf("checkingID must be a valid payment hash 32-byte hex, got '%s': %w", checkingID, err)
	}

	stream, err := l.Router.TrackPaymentV2(
		context.Background(),
		&routerrpc.TrackPaymentRequest{
			PaymentHash:       paymentHash,
			NoInflightUpdates: true,
		},
	)
	if err != nil {
		return rp.PaymentStatus{}, fmt.Errorf("error calling TrackPaymentV2: %w", err)
	}

	// the first event will always be the current state of the payment from the db
	payment, err := stream.Recv()
	if err != nil {
		return rp.PaymentStatus{},
			fmt.Errorf("error calling Recv() on TrackPaymentV2: %w", err)
	}

	return paymentToPaymentStatus(payment), nil
}

func paymentToPaymentStatus(payment *lnrpc.Payment) rp.PaymentStatus {
	status := rp.PaymentStatus{
		CheckingID: payment.PaymentHash,
		Status:     rp.Unknown,
		FeePaid:    0,
		Preimage:   "",
	}

	switch payment.Status {
	case lnrpc.Payment_IN_FLIGHT:
		status.Status = rp.Pending
		return status
	case lnrpc.Payment_FAILED:
		if len(payment.Htlcs) == 0 {
			status.Status = rp.NeverTried
		} else {
			status.Status = rp.Failed
		}
		return status
	case lnrpc.Payment_SUCCEEDED:
		status.Status = rp.Complete
		status.FeePaid = payment.FeeMsat
		status.Preimage = payment.PaymentPreimage
		return status
	default:
		return status
	}
}

func (l *LndWallet) PaidInvoicesStream() (<-chan rp.InvoiceStatus, error) {
	listener := make(chan rp.InvoiceStatus)
	l.invoiceStatusListeners = append(l.invoiceStatusListeners, listener)
	return listener, nil
}

func (l *LndWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	listener := make(chan rp.PaymentStatus)
	l.paymentStatusListeners = append(l.paymentStatusListeners, listener)
	return listener, nil
}

func (l *LndWallet) startInvoicesStream() {
	stream, err := l.Lightning.SubscribeInvoices(context.Background(), &lnrpc.InvoiceSubscription{})
	if err != nil {
		log.Fatalf("Failed to SubscribeInvoices: %v", err)
	}
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("Error receiving invoice event: %v", err)
		}

		if res.State != lnrpc.Invoice_SETTLED {
			continue // Only notify for paid invoices
		}
		for _, listener := range l.invoiceStatusListeners {
			go func(listener chan rp.InvoiceStatus) {
				if res.IsKeysend {
					// An HTLC entry must be present with custom records.
					if len(res.Htlcs) == 0 {
						return
					}

					listener <- rp.InvoiceStatus{
						CheckingID:       hex.EncodeToString(res.RHash),
						Preimage:         hex.EncodeToString(res.RPreimage),
						Exists:           true,
						Paid:             res.State == lnrpc.Invoice_SETTLED,
						MSatoshiReceived: res.AmtPaidMsat,
						IsKeySend:        res.IsKeysend,
						CustomRecords:    res.Htlcs[0].CustomRecords,
					}
				} else {
					listener <- rp.InvoiceStatus{
						CheckingID:       hex.EncodeToString(res.RHash),
						Exists:           true,
						Paid:             res.State == lnrpc.Invoice_SETTLED,
						MSatoshiReceived: res.AmtPaidMsat,
					}
				}
			}(listener)
		}
	}
}

func (l *LndWallet) startPaymentsStream() {
	// get latest settled payment index
	res, err := l.Lightning.ListPayments(context.Background(), &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: false,
		IndexOffset:       0,
		MaxPayments:       1,
		Reversed:          true,
	})
	if err != nil {
		log.Error(fmt.Errorf("error getting latest paid index: %w", err))
	}
	if len(res.Payments) == 0 {
		return
	}
	lastPaidIndex := res.Payments[0].PaymentIndex

	// get all pending payments
	res, err = l.Lightning.ListPayments(context.Background(), &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: true,
		IndexOffset:       lastPaidIndex,
		Reversed:          false,
	})
	if err != nil {
		log.Error(fmt.Errorf("error listing pending payments: %w", err))
	}

	// track all these pending payments
	for _, payment := range res.Payments {
		go l.trackOutgoingPayment(payment.PaymentHash)
	}
}

func (l *LndWallet) trackOutgoingPayment(hash string) {
	paymentHash, err := hex.DecodeString(hash)
	if err != nil {
		panic(fmt.Errorf("failed to decode hex on trackOutgoingPayment(%s): %w",
			hash, err))
	}

	time.Sleep(time.Second * 5)
	stream, err := l.Router.TrackPaymentV2(
		context.Background(),
		&routerrpc.TrackPaymentRequest{
			PaymentHash:       paymentHash,
			NoInflightUpdates: true,
		},
	)
	if err != nil {
		log.Error(fmt.Errorf(
			"call to TrackPaymentV2 failed on trackOutgoingPayment(%s): %w", hash, err))
	}

	status := rp.PaymentStatus{
		Status:     rp.Unknown,
		CheckingID: hash,
	}

checkPaymentStatus:
	for {
		payment, err := stream.Recv()

		if err != nil {
			log.Error(fmt.Errorf("failed to stream.Recv() on trackOutgoingPayment(%s): %w",
				hash, err))
		}

		switch payment.Status {
		case lnrpc.Payment_UNKNOWN:
			// was never attempted (but maybe it will still be in the next seconds?)
			return
		case lnrpc.Payment_SUCCEEDED:
			status.Status = rp.Complete
			status.FeePaid = payment.FeeMsat
			status.Preimage = payment.PaymentPreimage
			break checkPaymentStatus
		case lnrpc.Payment_FAILED:
			status.Status = rp.Failed
			break checkPaymentStatus
		default:
			// all other cases are ignored
			return
		}
	}

	// at this point we know this payment either failed or succeeded
	for _, listener := range l.paymentStatusListeners {
		listener <- status
	}
}
