package settings

import (
	"context"
	"github.com/mediago-dev/mediago-drama/services/server/internal/platform/codexapp"
)

// NewCodexImageSession shares the managed Codex home without exporting its tokens.
func (service *Settings) NewCodexImageSession(ctx context.Context) (codexapp.Client, error) {
	if service == nil || service.codexAccount == nil || service.codexAccount.binPath == "" {
		return nil, ErrCodexAccountUnavailable
	}
	return codexapp.StartImage(ctx, service.codexAccount.binPath)
}

// CodexImageAvailable checks the subscription capability without generating media.
func (service *Settings) CodexImageAvailable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, codexAccountRequestTimeout)
	defer cancel()
	session, err := service.NewCodexImageSession(ctx)
	if err != nil {
		return false
	}
	defer session.Close()
	available, err := codexapp.ImageAvailable(ctx, session)
	return err == nil && available
}
