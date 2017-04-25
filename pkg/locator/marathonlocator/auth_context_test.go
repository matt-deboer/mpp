package marathonlocator

import (
	"encoding/json"
	"testing"

	"github.com/matt-deboer/mpp/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestParsePrincipalSecret(t *testing.T) {

	pkBytes := testhelpers.ToPEM(testhelpers.GenPrivateKey(t))

	_json := make(map[string]interface{})
	_json["login_endpoint"] = "http://login-here.com"
	_json["uid"] = "random"
	_json["private_key"] = string(pkBytes)
	jsonBytes, _ := json.Marshal(_json)

	creds, err := fromPrincipalSecret(jsonBytes)
	assert.NoError(t, err)
	assert.NotNil(t, creds, "Creds should be parsed")
	assert.Equal(t, "random", creds.UID, "UID not equal")
	assert.Equal(t, "http://login-here.com", creds.AuthEndpoint)
	assert.NotNil(t, creds.PrivateKey, "Private key should be parsed")
}
