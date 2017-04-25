package marathonlocator

import (
	"encoding/json"
	"testing"

	"github.com/matt-deboer/mpp/pkg/testhelpers"
	"github.com/stretchr/testify/assert"

	"net/http/httptest"
)

func TestAuthenticate(t *testing.T) {

	pk := testhelpers.GenPrivateKey(t)
	token := testhelpers.GenToken()

	authServer := httptest.NewTLSServer(&testhelpers.MockAuthEndpoint{PK: pk, Token: token})
	defer authServer.Close()

	targetServer := httptest.NewTLSServer(&testhelpers.MockTarget{ExpectedAuthZ: "token=" + token})
	defer targetServer.Close()

	pkBytes := testhelpers.ToPEM(testhelpers.GenPrivateKey(t))

	_json := make(map[string]interface{})
	_json["login_endpoint"] = authServer.URL
	_json["uid"] = "random"
	_json["private_key"] = string(pkBytes)
	jsonBytes, _ := json.Marshal(_json)

	creds, err := fromPrincipalSecret(jsonBytes)
	a := newAuthenticator(creds, true)

	authToken, err := a.authenticate()
	assert.NoError(t, err)
	assert.NotEmpty(t, authToken)
}
