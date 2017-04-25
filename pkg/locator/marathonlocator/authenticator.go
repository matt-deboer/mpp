package marathonlocator

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type authenticator struct {
	client *http.Client
	creds  *authContext
	hash   crypto.Hash
	authZ  string
}

func (a *authenticator) authenticate() (string, error) {

	var body string
	var bodyLog string
	if a.creds.PrivateKey != nil {
		token, _ := a.getSelfSignedToken()
		body = fmt.Sprintf(`{"uid":"%s","token":"%s"}`, a.creds.UID, token)
		bodyLog = body
	} else {
		body = fmt.Sprintf(`{"uid":"%s","password":"%s"}`, a.creds.UID, a.creds.Password)
		bodyLog = fmt.Sprintf(`{"uid":"%s","password":"%s"}`, a.creds.UID, "****")
	}

	if log.GetLevel() >= log.DebugLevel {
		log.Infof("Authenticating: POST %s  %s", a.creds.AuthEndpoint, bodyLog)
	}

	r, _ := http.NewRequest("POST", a.creds.AuthEndpoint, bytes.NewBufferString(body))
	r.Header.Add("Content-Type", "application/json")
	resp, err := a.client.Do(r)
	if err != nil {
		return "", err
	}

	if log.GetLevel() >= log.DebugLevel {
		log.Infof("Authentication result: %d", resp.StatusCode)
	}

	rbody := []byte{}
	if resp.Body != nil {
		rbodyString, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		rbody = rbodyString
		defer resp.Body.Close()
	}

	if resp.StatusCode == 200 {
		data := make(map[string]interface{})
		if err := json.Unmarshal(rbody, &data); err != nil {
			return "", err
		}
		return data["token"].(string), nil
	}

	log.Error(fmt.Sprintf("POST %s : %d\n%s",
		a.creds.AuthEndpoint, resp.StatusCode, resp.Body))
	return "", errors.New("Failed to authenticate")

}

func (a *authenticator) getSelfSignedToken() (string, error) {

	head := base64URLEncode([]byte(`{"alg":"RS256","typ":"JWT"}`))
	body := base64URLEncode([]byte(fmt.Sprintf(`{"uid": "%s"}`, a.creds.UID)))
	rawToken := head + "." + body

	hashed := sha256.Sum256([]byte(rawToken))
	sig, err := rsa.SignPKCS1v15(rand.Reader, a.creds.PrivateKey, a.hash, hashed[:])
	if err != nil {
		return "", err
	}

	signature := base64URLEncode(sig)
	return rawToken + "." + signature, nil
}

func base64URLEncode(bytes []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(bytes), "=")
}

func newAuthenticator(creds *authContext, insecure bool) *authenticator {
	var client *http.Client
	if insecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	} else {
		client = &http.Client{}
	}
	return &authenticator{creds: creds, hash: crypto.SHA256, client: client}
}
