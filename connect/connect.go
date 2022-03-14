package relampago_connect

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/lnbits/relampago"
	"github.com/lnbits/relampago/eclair"
	"github.com/lnbits/relampago/lnd"
	"github.com/lnbits/relampago/sparko"
	"github.com/lnbits/relampago/void"
)

type LightningBackendSettings struct {
	BackendType    string `envconfig:"LIGHTNING_BACKEND_TYPE"`
	ConnectTimeout string `envconfig:"LIGHTNING_CONNECT_TIMEOUT" default:"15"`

	SparkoURL   string `envconfig:"SPARKO_URL"`
	SparkoToken string `envconfig:"SPARKO_TOKEN"`

	LNDHost         string `envconfig:"LND_HOST"`
	LNDCertPath     string `envconfig:"LND_CERT_PATH"`
	LNDMacaroonPath string `envconfig:"LND_MACAROON_PATH"`

	EclairHost     string `envconfig:"ECLAIR_HOST"`
	EclairPassword string `envconfig:"ECLAIR_PASSWORD"`
}

func Connect() (relampago.Wallet, error) {
	var lbs LightningBackendSettings
	err := envconfig.Process("", &lbs)
	if err != nil {
		return nil, fmt.Errorf("failed to process envconfig: %w", err)
	}

	connectTimeout, err := strconv.Atoi(lbs.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	// start lightning backend
	switch lbs.BackendType {
	case "lndrest":
	case "lnd", "lndgrpc":
		return lnd.Start(lnd.Params{
			Host:           lbs.LNDHost,
			CertPath:       lbs.LNDCertPath,
			MacaroonPath:   lbs.LNDMacaroonPath,
			ConnectTimeout: time.Duration(connectTimeout) * time.Second,
		})
	case "eclair":
		return eclair.Start(eclair.Params{
			Host:     lbs.EclairHost,
			Password: lbs.EclairPassword,
		})
	case "clightning":
	case "sparko":
		return sparko.Start(sparko.Params{
			Host:           lbs.SparkoURL,
			Key:            lbs.SparkoToken,
			ConnectTimeout: time.Duration(connectTimeout) * time.Second,
		})
	case "lnbits":
	case "lnpay":
	case "zebedee":
	}

	// use void wallet that does nothing
	return void.Start()
}
