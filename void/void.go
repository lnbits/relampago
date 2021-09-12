package void

import rp "github.com/fiatjaf/relampago"

type VoidWallet struct{}

func New() VoidWallet {
	return VoidWallet{}
}

func (v VoidWallet) GetInfo() (rp.WalletInfo, error) {
	return rp.WalletInfo{
		Balance: 0,
	}, nil
}

func (v VoidWallet) CreateInvoice(rp.InvoiceParams) (rp.InvoiceData, error) {
	return rp.InvoiceData{
		CheckingID: "void",
		Preimage:   "0000000000000000000000000000000000000000000000000000000000000000",
		Invoice:    "lnbc1",
	}, nil
}

func (v VoidWallet) GetInvoiceStatus(string) (rp.InvoiceStatus, error) {
	return rp.InvoiceStatus{
		Exists:           true,
		Paid:             false,
		MSatoshiReceived: 0,
	}, nil
}

func (v VoidWallet) PaidInvoicesStream() (<-chan rp.InvoiceStatus, error) {
	return make(chan rp.InvoiceStatus), nil
}

func (v VoidWallet) MakePayment(rp.PaymentParams) (rp.PaymentData, error) {
	return rp.PaymentData{
		CheckingID: "void",
	}, nil
}

func (v VoidWallet) GetPaymentStatus(string) (rp.PaymentStatus, error) {
	return rp.PaymentStatus{
		Status:   rp.Pending,
		FeePaid:  0,
		Preimage: "",
	}, nil
}

func (v VoidWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	return make(chan rp.PaymentStatus), nil
}
