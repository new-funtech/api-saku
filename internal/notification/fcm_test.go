package notification

import (
	"encoding/base64"
	"testing"
)

func TestCredentialsFromEnv(t *testing.T) {
	t.Setenv("FIREBASE_CREDENTIALS_BASE64", "")
	if data, err := credentialsFromEnv(); data != nil || err != nil {
		t.Fatalf("unset: %v %v", data, err)
	}

	json := `{"type":"service_account","project_id":"x"}`
	t.Setenv("FIREBASE_CREDENTIALS_BASE64", "  "+base64.StdEncoding.EncodeToString([]byte(json))+"\n")
	data, err := credentialsFromEnv()
	if err != nil || string(data) != json {
		t.Fatalf("valid: %q %v", data, err)
	}

	t.Setenv("FIREBASE_CREDENTIALS_BASE64", "not base64 !!")
	if _, err := credentialsFromEnv(); err == nil {
		t.Fatal("bad base64 must be an error")
	}
	t.Setenv("FIREBASE_CREDENTIALS_BASE64", base64.StdEncoding.EncodeToString([]byte("plain text")))
	if _, err := credentialsFromEnv(); err == nil {
		t.Fatal("non-JSON content must be an error")
	}
}
