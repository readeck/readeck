package profile

import "time"

// TokenForm describes a token form.
type tokenForm struct {
	Expires   *time.Time `json:"expires"`
	IsEnabled bool       `json:"is_enabled"`
}

type deleteForm struct {
	Cancel bool `json:"cancel"`
}
