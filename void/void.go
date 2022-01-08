package void

import rp "github.com/lnbits/relampago"

type VoidWallet struct{}

func Start() (VoidWallet, error) {
	return VoidWallet{}, nil
}

// Compile time check to ensure that VoidWallet fully implements rp.Wallet
var _ rp.Wallet = (*VoidWallet)(nil)

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

func (v VoidWallet) GetInvoiceStatus(checkingID string) (rp.InvoiceStatus, error) {
	return rp.InvoiceStatus{
		CheckingID:       checkingID,
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

func (v VoidWallet) GetPaymentStatus(checkingID string) (rp.PaymentStatus, error) {
	return rp.PaymentStatus{
		CheckingID: checkingID,
		Status:     rp.Pending,
		FeePaid:    0,
		Preimage:   "",
	}, nil
}

func (v VoidWallet) PaymentsStream() (<-chan rp.PaymentStatus, error) {
	return make(chan rp.PaymentStatus), nil
}
