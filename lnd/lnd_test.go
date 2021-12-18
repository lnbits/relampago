package lnd

import (
	"context"
	"errors"
	"testing"
	"time"

	rp "github.com/fiatjaf/relampago"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"google.golang.org/grpc"
)

//###############//
//  BEGIN TESTS  //
//###############//

func TestGetInfo(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.ChannelBalanceMock = func(_ *lnrpc.ChannelBalanceRequest) (*lnrpc.ChannelBalanceResponse, error) {
		return &lnrpc.ChannelBalanceResponse{
			LocalBalance: &lnrpc.Amount{Sat: 10},
		}, nil
	}

	got, err := lnd.GetInfo()
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got.Balance != 10 {
		t.Errorf("got %v, wanted %v", got.Balance, 10)
	}
}

func TestGetInfo_Error(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.ChannelBalanceMock = func(_ *lnrpc.ChannelBalanceRequest) (*lnrpc.ChannelBalanceResponse, error) {
		return nil, errors.New("error")
	}

	_, err := lnd.GetInfo()
	if err == nil {
		t.Errorf("got %v, wanted %v", err, "error")
	}
}

func TestCreateInvoice(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.AddInvoiceMock = func(_ *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
		return &lnrpc.AddInvoiceResponse{RHash: []byte{255}}, nil
	}
	lightning.LookupInvoiceMock = func(_ *lnrpc.PaymentHash) (*lnrpc.Invoice, error) {
		return &lnrpc.Invoice{
			RHash:          []byte{255},
			RPreimage:      []byte{5},
			PaymentRequest: "ln000",
		}, nil
	}

	expiry := time.Second
	params := rp.InvoiceParams{
		Msatoshi:        10000,
		Description:     "Test",
		DescriptionHash: nil,
		Expiry:          &expiry,
	}
	want := rp.InvoiceData{
		CheckingID: "ff",
		Preimage:   "05",
		Invoice:    "ln000",
	}
	got, err := lnd.CreateInvoice(params)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestGetInvoiceStatus(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.LookupInvoiceMock = func(_ *lnrpc.PaymentHash) (*lnrpc.Invoice, error) {
		return &lnrpc.Invoice{
			RHash:          []byte{255},
			RPreimage:      []byte{5},
			PaymentRequest: "ln000",
			State:          lnrpc.Invoice_SETTLED,
			AmtPaidMsat:    10000,
		}, nil
	}
	checkingID := "ff"
	want := rp.InvoiceStatus{
		CheckingID:       "ff",
		Exists:           true,
		Paid:             true,
		MSatoshiReceived: 10000,
	}
	got, err := lnd.GetInvoiceStatus(checkingID)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}
func TestGetInvoiceStatus_NotPaid(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.LookupInvoiceMock = func(_ *lnrpc.PaymentHash) (*lnrpc.Invoice, error) {
		return &lnrpc.Invoice{
			RHash:          []byte{255},
			RPreimage:      []byte{5},
			PaymentRequest: "ln000",
			State:          lnrpc.Invoice_OPEN,
			AmtPaidMsat:    0,
		}, nil
	}
	checkingID := "ff"
	want := rp.InvoiceStatus{
		CheckingID:       "ff",
		Exists:           true,
		Paid:             false,
		MSatoshiReceived: 0,
	}
	got, err := lnd.GetInvoiceStatus(checkingID)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestGetInvoiceStatus_NotFound(t *testing.T) {
	lightning, _, lnd := setupMocks()
	lightning.LookupInvoiceMock = func(_ *lnrpc.PaymentHash) (*lnrpc.Invoice, error) {
		return nil, errors.New("not found")
	}
	checkingID := "ff"
	want := rp.InvoiceStatus{
		CheckingID:       "ff",
		Exists:           false,
		Paid:             false,
		MSatoshiReceived: 0,
	}
	got, err := lnd.GetInvoiceStatus(checkingID)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestMakePayment(t *testing.T) {
	_, router, lnd := setupMocks()
	router.SendPaymentV2Mock = func(req *routerrpc.SendPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{}, nil
	}
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{}, nil
	}

	params := rp.PaymentParams{
		Invoice:      "lnbc175001ps6e5udpp58ur2s8s2ps4dxnhfmu4rpkr6syx6nc7r3q0hsp644nj7tejdxznsdq5w3jhxapqd9h8vmmfvdjscqzpgxqyz5vqsp50cs6gww9y96g84635a7apkwmmmlv69a2sah89qq03ngdgrvdf4ts9qyyssqs9kx2rngh4ty3h5t9hkrx4dxhfrne2jccluw6eq42hutaejvh474wvfg8untkk484v77043aus92mfshmq6psp487r34c5huglpnf0cq24eqg3",
		CustomAmount: 0,
	}
	want := rp.PaymentData{CheckingID: "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7"}
	got, err := lnd.MakePayment(params)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got.CheckingID != want.CheckingID {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestMakePayment_CustomAmount(t *testing.T) {
	_, router, lnd := setupMocks()
	var called *routerrpc.SendPaymentRequest
	router.SendPaymentV2Mock = func(req *routerrpc.SendPaymentRequest) ([]*lnrpc.Payment, error) {
		called = req
		return []*lnrpc.Payment{{}}, nil
	}
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{}, nil
	}

	params := rp.PaymentParams{
		Invoice:      "lnbc175001ps6e5udpp58ur2s8s2ps4dxnhfmu4rpkr6syx6nc7r3q0hsp644nj7tejdxznsdq5w3jhxapqd9h8vmmfvdjscqzpgxqyz5vqsp50cs6gww9y96g84635a7apkwmmmlv69a2sah89qq03ngdgrvdf4ts9qyyssqs9kx2rngh4ty3h5t9hkrx4dxhfrne2jccluw6eq42hutaejvh474wvfg8untkk484v77043aus92mfshmq6psp487r34c5huglpnf0cq24eqg3",
		CustomAmount: 10000,
	}
	want := rp.PaymentData{CheckingID: "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7"}
	got, err := lnd.MakePayment(params)
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got.CheckingID != want.CheckingID {
		t.Errorf("got %v, wanted %v", got, want)
	}
	if called.AmtMsat != params.CustomAmount {
		t.Errorf("got %v, wanted %v for AmtMsat", called.AmtMsat, params.CustomAmount)
	}
}

func TestMakePayment_SendPaymentError(t *testing.T) {
	_, router, lnd := setupMocks()
	router.SendPaymentV2Mock = func(req *routerrpc.SendPaymentRequest) ([]*lnrpc.Payment, error) {
		return nil, errors.New("error")
	}
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{}, nil
	}

	params := rp.PaymentParams{
		Invoice:      "lnbc175001ps6e5udpp58ur2s8s2ps4dxnhfmu4rpkr6syx6nc7r3q0hsp644nj7tejdxznsdq5w3jhxapqd9h8vmmfvdjscqzpgxqyz5vqsp50cs6gww9y96g84635a7apkwmmmlv69a2sah89qq03ngdgrvdf4ts9qyyssqs9kx2rngh4ty3h5t9hkrx4dxhfrne2jccluw6eq42hutaejvh474wvfg8untkk484v77043aus92mfshmq6psp487r34c5huglpnf0cq24eqg3",
		CustomAmount: 10000,
	}
	_, err := lnd.MakePayment(params)
	if err == nil {
		t.Errorf("got %v, wanted error", err)
	}
}

func TestGetPaymentStatus(t *testing.T) {
	_, router, lnd := setupMocks()
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{{
			PaymentHash:     "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7",
			Status:          lnrpc.Payment_SUCCEEDED,
			FeeMsat:         10000,
			PaymentPreimage: "preimage",
		}}, nil
	}

	want := rp.PaymentStatus{
		CheckingID: "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7",
		Status:     rp.Complete,
		FeePaid:    10000,
		Preimage:   "preimage",
	}
	got, err := lnd.GetPaymentStatus("3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7")
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestGetPaymentStatus_Incomplete(t *testing.T) {
	_, router, lnd := setupMocks()
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return []*lnrpc.Payment{{
			PaymentHash: "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7",
			Status:      lnrpc.Payment_IN_FLIGHT,
		}}, nil
	}

	want := rp.PaymentStatus{
		CheckingID: "3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7",
		Status:     rp.Pending,
		FeePaid:    0,
		Preimage:   "",
	}
	got, err := lnd.GetPaymentStatus("3f06a81e0a0c2ad34ee9df2a30d87a810da9e3c3881f780755ace5e5e64d30a7")
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

func TestGetPaymentStatus_NotFound(t *testing.T) {
	_, router, lnd := setupMocks()
	router.TrackPaymentV2Mock = func(req *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error) {
		return nil, errors.New("lnd will return an error when it can't find the payment or it wasn't initiated")
	}

	_, err := lnd.GetPaymentStatus("5")
	if err == nil {
		t.Errorf("got %v, wanted error", err)
	}
}

func TestPaidInvoicesStream(t *testing.T) {
	lightning, _, lnd := setupMocks()
	PaymentPollInterval = time.Millisecond
	lightning.SubscribeInvoicesMock = func(sub *lnrpc.InvoiceSubscription) ([]*lnrpc.Invoice, error) {
		return []*lnrpc.Invoice{
			{
				RHash:       []byte{16},
				State:       lnrpc.Invoice_OPEN,
				AmtPaidMsat: 0,
			},
			{
				RHash:       []byte{17},
				State:       lnrpc.Invoice_SETTLED,
				AmtPaidMsat: 1000,
			},
		}, nil
	}
	lightning.ListPaymentsMock = func(_ *lnrpc.ListPaymentsRequest) (*lnrpc.ListPaymentsResponse, error) {
		return &lnrpc.ListPaymentsResponse{Payments: []*lnrpc.Payment{}}, nil
	}

	want := rp.InvoiceStatus{
		CheckingID:       "11",
		Exists:           true,
		Paid:             true,
		MSatoshiReceived: 1000,
	}

	go lnd.startInvoicesStream()

	stream, err := lnd.PaidInvoicesStream()
	if err != nil {
		t.Errorf("got %v, wanted %v", err, nil)
	}
	got := <-stream
	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
	}
}

//#############//
//  END TESTS  //
//#############//

type PaymentStreamMock struct {
	grpc.ClientStream
	Data chan *lnrpc.Payment
}

type InvoiceStreamMock struct {
	grpc.ClientStream
	Data chan *lnrpc.Invoice
}

func (s PaymentStreamMock) Recv() (*lnrpc.Payment, error) {
	return <-s.Data, nil
}

func (s InvoiceStreamMock) Recv() (*lnrpc.Invoice, error) {
	return <-s.Data, nil
}

type MockLightningClient struct {
	lnrpc.LightningClient

	ChannelBalanceMock    func(*lnrpc.ChannelBalanceRequest) (*lnrpc.ChannelBalanceResponse, error)
	AddInvoiceMock        func(*lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error)
	LookupInvoiceMock     func(*lnrpc.PaymentHash) (*lnrpc.Invoice, error)
	ListPaymentsMock      func(*lnrpc.ListPaymentsRequest) (*lnrpc.ListPaymentsResponse, error)
	SubscribeInvoicesMock func(*lnrpc.InvoiceSubscription) ([]*lnrpc.Invoice, error)
}

type MockRouterClient struct {
	routerrpc.RouterClient

	SendPaymentV2Mock  func(request *routerrpc.SendPaymentRequest) ([]*lnrpc.Payment, error)
	TrackPaymentV2Mock func(request *routerrpc.TrackPaymentRequest) ([]*lnrpc.Payment, error)
}

func (m *MockLightningClient) ChannelBalance(
	_ context.Context, req *lnrpc.ChannelBalanceRequest, _ ...grpc.CallOption) (*lnrpc.ChannelBalanceResponse, error) {
	return m.ChannelBalanceMock(req)
}

func (m *MockLightningClient) AddInvoice(
	_ context.Context, req *lnrpc.Invoice, _ ...grpc.CallOption) (*lnrpc.AddInvoiceResponse, error) {
	return m.AddInvoiceMock(req)
}

func (m *MockLightningClient) LookupInvoice(
	_ context.Context, req *lnrpc.PaymentHash, _ ...grpc.CallOption) (*lnrpc.Invoice, error) {
	return m.LookupInvoiceMock(req)
}

func (m *MockLightningClient) ListPayments(
	_ context.Context, req *lnrpc.ListPaymentsRequest, _ ...grpc.CallOption) (*lnrpc.ListPaymentsResponse, error) {
	return m.ListPaymentsMock(req)
}

func (m *MockLightningClient) SubscribeInvoices(
	_ context.Context, req *lnrpc.InvoiceSubscription, _ ...grpc.CallOption) (lnrpc.Lightning_SubscribeInvoicesClient, error) {
	client := InvoiceStreamMock{Data: make(chan *lnrpc.Invoice)}
	data, err := m.SubscribeInvoicesMock(req)
	if err != nil {
		return nil, err
	}
	for _, datum := range data {
		d := datum
		go func() { client.Data <- d }()
	}
	return client, nil
}

func (m *MockRouterClient) SendPaymentV2(
	_ context.Context, req *routerrpc.SendPaymentRequest, _ ...grpc.CallOption) (routerrpc.Router_SendPaymentV2Client, error) {
	client := PaymentStreamMock{Data: make(chan *lnrpc.Payment)}
	data, err := m.SendPaymentV2Mock(req)
	if err != nil {
		return nil, err
	}
	for _, datum := range data {
		d := datum
		go func() { client.Data <- d }()
	}
	return client, nil
}

func (m *MockRouterClient) TrackPaymentV2(
	_ context.Context, req *routerrpc.TrackPaymentRequest, _ ...grpc.CallOption) (routerrpc.Router_TrackPaymentV2Client, error) {
	client := PaymentStreamMock{Data: make(chan *lnrpc.Payment)}
	data, err := m.TrackPaymentV2Mock(req)
	if err != nil {
		return nil, err
	}
	for _, datum := range data {
		d := datum
		go func() { client.Data <- d }()
	}
	return client, nil
}

func setupMocks() (*MockLightningClient, *MockRouterClient, LndWallet) {
	lightning := &MockLightningClient{}
	router := &MockRouterClient{}
	return lightning, router, LndWallet{
		Lightning: lightning,
		Router:    router,
	}
}
