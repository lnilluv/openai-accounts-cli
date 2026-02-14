package domain

type AuthMethod string

const (
	AuthMethodAPIKey  AuthMethod = "api_key"
	AuthMethodChatGPT AuthMethod = "chatgpt"
)

type Auth struct {
	Method AuthMethod
	// SecretRef points to a secret-store entry, typically in "provider://path" form.
	SecretRef string
}
