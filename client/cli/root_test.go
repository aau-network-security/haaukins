package cli

import (
	"os"
	"testing"
)

func Test_downloadCerts(t *testing.T) {

	notValidCertMap :=  map[string]string{
		"CERT":"not_valid_url",
	}
	emptyCertURL := map[string]string{
		"CERT":"",
	}

	tests := []struct {
		name    string
		certsMap    map[string]string
		wantErr bool
	}{
		// LocalCertificates variable holds valid urls, there should be any problem
		{"No error with valid cert URL ", LocalCertificates, false },
		{"Not valid cert URL", notValidCertMap,true},
		{"Empty string for cert URL",emptyCertURL,true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := downloadCerts(tt.certsMap); (err != nil) != tt.wantErr {
				t.Errorf("downloadCerts() error = %v, wantErr %v", err, tt.wantErr)
			}
			os.RemoveAll("localcerts")
		})
	}

}
