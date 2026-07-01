package domain

import (
	"encoding/json"
)

// ProjectConfig holds per-project sending identity, provider reference, enabled channels and quotas.
// provider_creds_ref is a reference to a cluster secret; credentials are never stored in the row.
type ProjectConfig struct {
	Namespace          string                 `json:"namespace"`
	FromEmail          string                 `json:"from_email"`
	FromName           string                 `json:"from_name"`
	ProviderCredsRef   string                 `json:"provider_creds_ref,omitempty"`
	ChannelsEnabled    []string               `json:"channels_enabled"`
	DefaultTemplateSet string                 `json:"default_template_set,omitempty"`
	Quota              map[string]interface{} `json:"quota,omitempty"`
}

// ChannelsEnabledJSON returns the channels slice as a JSON string for storage.
func (p *ProjectConfig) ChannelsEnabledJSON() (string, error) {
	b, err := json.Marshal(p.ChannelsEnabled)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
