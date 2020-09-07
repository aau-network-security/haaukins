package amigo

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const captchaVerifyAPI = "https://www.google.com/recaptcha/api/siteverify"

type Recaptcha struct {
	secret    string
	lastError []string
}

// Struct for parsing json in google's response
type googleResponse struct {
	Success    bool
	ErrorCodes []string `json:"error-codes"`
}

func NewRecaptcha(secret string) Recaptcha {
	return Recaptcha{secret: secret}
}

// Verifies if current request have valid re-captcha response and returns true or false
// This method also records any errors in validation.
func (r *Recaptcha) Verify(response string) bool {

	r.lastError = make([]string, 1)
	client := &http.Client{Timeout: 20 * time.Second}

	resp, err := client.PostForm(captchaVerifyAPI, url.Values{"secret": {r.secret}, "response": {response}})
	if err != nil {
		r.lastError = append(r.lastError, err.Error())
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		r.lastError = append(r.lastError, err.Error())
		return false
	}

	gr := new(googleResponse)
	err = json.Unmarshal(body, gr)
	if err != nil {
		r.lastError = append(r.lastError, err.Error())
		return false
	}
	if !gr.Success {
		r.lastError = append(r.lastError, gr.ErrorCodes...)
	}
	return gr.Success
}
