package main

import (
	"fmt"

	"github.com/openmeet-team/survey/internal/oauth"
)

// keygen generates a JWK for OAuth client authentication
func main() {
	jwk := oauth.GenerateSecretJWK()
	fmt.Println(jwk)
}
