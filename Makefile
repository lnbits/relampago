PKG := github.com/lnbits/relampago

GOBUILD := GO111MODULE=on go build -v

build:
	$(GOBUILD) $(PKG)
	$(GOBUILD) $(PKG)/void
	$(GOBUILD) $(PKG)/sparko
	$(GOBUILD) $(PKG)/lnd

test:
	go test ./...
