package tokens

import (
	"encoding/json"
	"fmt"

	"github.com/cristalhq/jwt/v3"
	"github.com/readeck/readeck/configs"
)

// NewJwtToken returns a new JWT token instance using
// a given ID and signing with the configuration's JWT secret key.
func NewJwtToken(UID string) (*jwt.Token, error) {
	signer, err := jwt.NewSignerEdDSA(configs.JwtSk())
	if err != nil {
		return nil, err
	}

	claims := &jwt.RegisteredClaims{
		ID: fmt.Sprintf("%s", UID),
	}

	builder := jwt.NewBuilder(signer)
	token, err := builder.Build(claims)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// GetJwtClaims checks a raw JWT claims and returns it when it passes
// the signature validation.
func GetJwtClaims(data string) (jwt.StandardClaims, error) {
	var claims jwt.StandardClaims

	verifier, err := jwt.NewVerifierEdDSA(configs.JwtPk())
	if err != nil {
		return claims, err
	}

	newToken, err := jwt.ParseAndVerifyString(data, verifier)
	if err != nil {
		return claims, err
	}

	err = json.Unmarshal(newToken.RawClaims(), &claims)
	if err != nil {
		return claims, err
	}

	return claims, nil
}
