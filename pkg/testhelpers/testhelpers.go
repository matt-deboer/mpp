package testhelpers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	mrand "math/rand"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"encoding/json"
	"io/ioutil"
)

func GenPrivateKey(t *testing.T) *rsa.PrivateKey {
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	return pk
}

func ToPEM(pk *rsa.PrivateKey) []byte {
	var buffer bytes.Buffer
	pem.Encode(&buffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
	return buffer.Bytes()
}

func GenToken() string {
	b := make([]rune, 64)
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	for i := range b {
		b[i] = letters[mrand.Intn(len(letters))]
	}
	return string(b)
}

type MockAuthEndpoint struct {
	PK    *rsa.PrivateKey
	Token string
}

func (m *MockAuthEndpoint) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	bytes, _ := ioutil.ReadAll(req.Body)
	data := make(map[string]interface{})
	json.Unmarshal(bytes, &data)
	// TODO: verify the token
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"token":"` + m.Token + `"}`))
}

type MockTarget struct {
	ExpectedAuthZ string
}

func (m *MockTarget) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	authZ := req.Header.Get("Authorization")
	if authZ == m.ExpectedAuthZ {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
	}
}
