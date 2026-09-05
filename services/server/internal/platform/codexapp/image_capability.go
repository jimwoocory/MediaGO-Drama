package codexapp

import "context"

// ImageAvailable requires both a ChatGPT login and provider-native image output.
// A vision-input model or an API-key login is not sufficient.
func ImageAvailable(ctx context.Context, session Client) (bool, error) {
	var account struct {
		Account *struct {
			Type string `json:"type"`
		} `json:"account"`
	}
	if err := session.Call(ctx, "account/read", map[string]bool{"refreshToken": false}, &account); err != nil {
		return false, err
	}
	if account.Account == nil || account.Account.Type != "chatgpt" {
		return false, nil
	}
	var capabilities struct {
		ImageGeneration bool `json:"imageGeneration"`
	}
	if err := session.Call(ctx, "modelProvider/capabilities/read", struct{}{}, &capabilities); err != nil {
		return false, err
	}
	return capabilities.ImageGeneration, nil
}
