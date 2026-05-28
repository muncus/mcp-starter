package registry

import (
	"fmt"
	"sync"

	"golang.org/x/oauth2"
)

// Scope represents a configurable OAuth scope with human-readable descriptions.
type Scope struct {
	Value       string `json:"value" yaml:"value"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Default     bool   `json:"default" yaml:"default"`
}

// Provider represents a configurable OAuth provider.
type Provider struct {
	Name         string          `json:"name" yaml:"name"`
	ClientID     string          `json:"client_id" yaml:"client_id"`
	ClientSecret string          `json:"client_secret" yaml:"client_secret"`
	Endpoint     oauth2.Endpoint `json:"-" yaml:"-"`
	RedirectURL  string          `json:"redirect_url" yaml:"redirect_url"`
	Scopes       []Scope         `json:"scopes" yaml:"scopes"`
}

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// RegisterProvider adds a provider configuration to the registry.
func RegisterProvider(p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.Name] = p
}

// GetProvider retrieves a provider configuration by name.
func GetProvider(name string) (Provider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// RequestScope adds a scope to the provider's scope list if not already present.
func RequestScope(provider string, scope Scope) (Provider, error) {
	providersMu.Lock()
	defer providersMu.Unlock()

	p, ok := providers[provider]
	if !ok {
		return Provider{}, fmt.Errorf("unknown OAuth provider: %s", provider)
	}

	for _, s := range p.Scopes {
		if s.Value == scope.Value {
			return p, nil // Scope already present
		}
	}

	p.Scopes = append(p.Scopes, scope)
	providers[provider] = p
	return p, nil
}
