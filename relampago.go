package relampago

import "time"

type Wallet interface {
	Kind() string
	GetInfo() (WalletInfo, error)

	CreateInvoice(InvoiceParams) (InvoiceData, error)
	GetInvoiceStatus(string) (InvoiceStatus, error)
	PaidInvoicesStream() (<-chan InvoiceStatus, error)

	MakePayment(PaymentParams) (PaymentData, error)
	GetPaymentStatus(string) (PaymentStatus, error)
	PaymentsStream() (<-chan PaymentStatus, error)
}

type WalletInfo struct {
	Balance int64 `json:"balance"`
}

type InvoiceParams struct {
	Msatoshi        int64          `json:"msatoshi"`
	Description     string         `json:"description"`
	DescriptionHash []byte         `json:"descriptionHash"`
	Expiry          *time.Duration `json:"expiry"`
}

type InvoiceData struct {
	CheckingID string `json:"checkingID"`
	Preimage   string `json:"preimage"`
	Invoice    string `json:"invoice"`
}

type InvoiceStatus struct {
	CheckingID       string `json:"checkingID"`
	Exists           bool   `json:"exists"`
	Paid             bool   `json:"paid"`
	MSatoshiReceived int64  `json:"msatoshiReceived"`
}

type PaymentParams struct {
	Invoice      string `json:"invoice"`
	CustomAmount int64  `json:"customAmount"`
}

type PaymentData struct {
	CheckingID string `json:"checkingID"`
}

type Status string

const (
	Unknown    Status = "unknown"
	NeverTried Status = "never-tried"
	Pending    Status = "pending"
	Failed     Status = "failed"
	Complete   Status = "complete"
)

type PaymentStatus struct {
	CheckingID string `json:"checkingID"`
	Status     Status `json:"status"`
	FeePaid    int64  `json:"feePaid"`
	Preimage   string `json:"preimage"`
}

// 	case LNBitsParams:
// 		body, _ := sjson.Set("{}", "amount", params.Msatoshi/1000)
// 		body, _ = sjson.Set(body, "out", false)
//
// 		if params.DescriptionHash == nil {
// 			if params.Description == "" {
// 				body, _ = sjson.Set(body, "memo", "created by makeinvoice")
// 			} else {
// 				body, _ = sjson.Set(body, "memo", params.Description)
// 			}
// 		} else {
// 			body, _ = sjson.Set(body, "description_hash", hexh)
// 		}
//
// 		req, err := http.NewRequest("POST",
// 			backend.Host+"/api/v1/payments",
// 			bytes.NewBufferString(body),
// 		)
// 		if err != nil {
// 			return "", err
// 		}
//
// 		req.Header.Set("X-Api-Key", backend.Key)
// 		req.Header.Set("Content-Type", "application/json")
// 		resp, err := http.DefaultClient.Do(req)
// 		if err != nil {
// 			return "", err
// 		}
// 		defer resp.Body.Close()
// 		if resp.StatusCode >= 300 {
// 			body, _ := ioutil.ReadAll(resp.Body)
// 			text := string(body)
// 			if len(text) > 300 {
// 				text = text[:300]
// 			}
// 			return "", fmt.Errorf("call to lnbits failed (%d): %s", resp.StatusCode, text)
// 		}
//
// 		defer resp.Body.Close()
// 		b, err := ioutil.ReadAll(resp.Body)
// 		if err != nil {
// 			return "", err
// 		}
//
// 		return gjson.ParseBytes(b).Get("payment_request").String(), nil
// 	case LNPayParams:
// 		client := lnpay.NewClient(backend.PublicAccessKey)
// 		wallet := client.Wallet(backend.WalletInvoiceKey)
// 		lntx, err := wallet.Invoice(lnpay.InvoiceParams{
// 			NumSatoshis:     params.Msatoshi / 1000,
// 			DescriptionHash: hexh,
// 		})
// 		if err != nil {
// 			return "", fmt.Errorf("error creating invoice on lnpay: %w", err)
// 		}
//
// 		return lntx.PaymentRequest, nil
// 	}
//
// 	return "", errors.New("missing backend params")
// }
