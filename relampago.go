package relampago

import "time"

type Wallet interface {
	GetInfo() (WalletInfo, error)

	CreateInvoice(InvoiceParams) (InvoiceData, error)
	GetInvoiceStatus(string) (InvoiceStatus, error)
	PaidInvoicesStream() (<-chan InvoiceStatus, error)

	MakePayment(PaymentParams) (PaymentData, error)
	Keysend(KeysendParams) (PaymentData, error)
	GetPaymentStatus(string) (PaymentStatus, error)
	PaymentsStream() (<-chan PaymentStatus, error)
}

type WalletInfo struct {
	Balance int64  `json:"balance"`
	Pubkey  string `json:"pubkey"`
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
	CheckingID       string            `json:"checkingID"`
	Preimage         string            `json:"preimage"`
	Exists           bool              `json:"exists"`
	Paid             bool              `json:"paid"`
	MSatoshiReceived int64             `json:"msatoshiReceived"`
	IsKeySend        bool              `json:"isKeySend"`
	CustomRecords    map[uint64][]byte `json:"customRecords"`
}

type PaymentParams struct {
	Invoice      string `json:"invoice"`
	CustomAmount int64  `json:"customAmount"`
}

type KeysendParams struct {
	Dest              string            `json:"dest"`
	PaymentHash       string            `json:"paymentHash"`
	Preimage          string            `json:"preimage"`
	Amount            int64             `json:"amount"`
	DestCustomRecords map[uint64][]byte `json:"destCustomRecords"`
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

// 	// description hash?
// 	var hexh, b64h string
// 	if params.DescriptionHash != nil {
// 		hexh = hex.EncodeToString(params.DescriptionHash)
// 		b64h = base64.StdEncoding.EncodeToString(params.DescriptionHash)
// 	}
//
// 	switch backend := params.Backend.(type) {
// 	case SparkoParams:
// 		spark := &lightning.Client{
// 			SparkURL:    backend.Host,
// 			SparkToken:  backend.Key,
// 			CallTimeout: time.Second * 3,
// 		}
//
// 		var method, desc string
// 		if params.DescriptionHash == nil {
// 			method = "invoice"
// 			desc = params.Description
// 		} else {
// 			method = "invoicewithdescriptionhash"
// 			desc = hexh
// 		}
//
// 		label := params.Label
// 		if label == "" {
// 			label = "makeinvoice/" + strconv.FormatInt(time.Now().Unix(), 16)
// 		}
//
// 		inv, err := spark.Call(method, params.Msatoshi, label, desc)
// 		if err != nil {
// 			return "", fmt.Errorf(method+" call failed: %w", err)
// 		}
// 		return inv.Get("bolt11").String(), nil
//
// 	case LNDParams:
// 		body, _ := sjson.Set("{}", "value_msat", params.Msatoshi)
//
// 		if params.DescriptionHash == nil {
// 			body, _ = sjson.Set(body, "memo", params.Description)
// 		} else {
// 			body, _ = sjson.Set(body, "description_hash", b64h)
// 		}
//
// 		req, err := http.NewRequest("POST",
// 			backend.Host+"/v1/invoices",
// 			bytes.NewBufferString(body),
// 		)
// 		if err != nil {
// 			return "", err
// 		}
//
// 		// macaroon must be hex, so if it is on base64 we adjust that
// 		if b, err := base64.StdEncoding.DecodeString(backend.Macaroon); err == nil {
// 			backend.Macaroon = hex.EncodeToString(b)
// 		}
//
// 		req.Header.Set("Grpc-Metadata-macaroon", backend.Macaroon)
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
// 			return "", fmt.Errorf("call to lnd failed (%d): %s", resp.StatusCode, text)
// 		}
//
// 		b, err := ioutil.ReadAll(resp.Body)
// 		if err != nil {
// 			return "", err
// 		}
//
// 		return gjson.ParseBytes(b).Get("payment_request").String(), nil
//
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
