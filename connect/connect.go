package relampago_connect

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"github.com/lnbits/relampago"
	"github.com/lnbits/relampago/lnd"
	"github.com/lnbits/relampago/sparko"
	"github.com/lnbits/relampago/void"
)

type LightningBackendSettings struct {
	BackendType string `envconfig:"LIGHTNING_BACKEND_TYPE"`

	SparkoURL   string `envconfig:"SPARKO_URL"`
	SparkoToken string `envconfig:"SPARKO_TOKEN"`

	LNDHost         string `envconfig:"LND_HOST"`
	LNDCertPath     string `envconfig:"LND_CERT_PATH"`
	LNDMacaroonPath string `envconfig:"LND_MACAROON_PATH"`
}

func Connect() (relampago.Wallet, error) {
	var lbs LightningBackendSettings
	err := envconfig.Process("", &lbs)
	if err != nil {
		return nil, fmt.Errorf("failed to process envconfig: %w", err)
	}

	// start lightning backend
	switch lbs.BackendType {
	case "lndrest":
	case "lndgrpc":
		return lnd.Start(lnd.Params{
			Host:         lbs.LNDHost,
			CertPath:     lbs.LNDCertPath,
			MacaroonPath: lbs.LNDMacaroonPath,
		})
	case "eclair":
	case "clightning":
	case "sparko":
		return sparko.Start(sparko.Params{
			Host:               lbs.SparkoURL,
			Key:                lbs.SparkoToken,
			InvoiceLabelPrefix: "lbs",
		})
	case "lnbits":
	case "lnpay":
	case "zebedee":
	}

	// use void wallet that does nothing
	return void.Start()
}
