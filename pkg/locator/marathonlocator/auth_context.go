package marathonlocator

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
)

type authContext struct {
	UID          string
	PrivateKey   *rsa.PrivateKey
	Password     string
	AuthEndpoint string
}

func fromPrincipalSecret(secret []byte) (*authContext, error) {
	data := make(map[string]interface{})
	if err := json.Unmarshal(secret, &data); err != nil {
		return nil, err
	}
	uid := data["uid"].(string)
	authEndpoint := data["login_endpoint"].(string)
	if pk, ok := data["private_key"]; ok {
		privateKey, _ := parsePrivateKey([]byte(pk.(string)))
		return &authContext{UID: uid, PrivateKey: privateKey, AuthEndpoint: authEndpoint}, nil
	} else if password, ok := data["password"]; ok {
		return &authContext{UID: uid, Password: password.(string), AuthEndpoint: authEndpoint}, nil
	}
	return nil, nil
}

func parsePrivateKey(key []byte) (*rsa.PrivateKey, error) {
	var err error
	var block *pem.Block

	if block, _ = pem.Decode(key); block == nil {
		return nil, errors.New("Failed to parse PEM private key")
	}

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
			return nil, err
		}
	}

	var pk *rsa.PrivateKey
	var ok bool
	if pk, ok = parsedKey.(*rsa.PrivateKey); !ok {
		return nil, errors.New("Failed to parse PEM private key")
	}

	return pk, nil
}
