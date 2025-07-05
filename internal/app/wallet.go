package app

import (
	"github.com/saradorri/gameintegrator/internal/domain"
	"github.com/saradorri/gameintegrator/internal/infrastructure/external/wallet"
)

func (a *application) InitWalletService() domain.WalletService {
	return wallet.NewWalletService(a.config.Wallet.URL, a.config.Wallet.APIKey)
}
