// Package notification sends push notifications through Firebase Cloud Messaging.
//
// Credentials are never part of the repository or the image. They come from either
// FIREBASE_CREDENTIALS_BASE64 (the service-account JSON, base64 on one line) or the file named by
// GOOGLE_APPLICATION_CREDENTIALS. With neither set the sender runs disabled: every call is a logged
// no-op, so local development needs no Firebase setup.
package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Messenger is the part of the FCM client the sender needs; tests replace it with a fake.
type Messenger interface {
	SendEachForMulticast(ctx context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error)
	Send(ctx context.Context, msg *messaging.Message) (string, error)
}

// credentialsFromEnv returns the service-account JSON given through FIREBASE_CREDENTIALS_BASE64
// (the file's content, base64 encoded on a single line: it survives .env files and CI variables,
// where a multi-line JSON does not). nil when the variable is not set.
func credentialsFromEnv() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_BASE64"))
	if raw == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("push: FIREBASE_CREDENTIALS_BASE64 is not valid base64: %w", err)
	}
	if !json.Valid(data) {
		return nil, errors.New("push: FIREBASE_CREDENTIALS_BASE64 does not decode to JSON")
	}
	return data, nil
}

// NewFCMMessenger builds the FCM client. It returns (nil, nil) when no credentials are configured,
// which callers treat as "push disabled".
func NewFCMMessenger(ctx context.Context) (Messenger, error) {
	var opts []option.ClientOption
	data, err := credentialsFromEnv()
	if err != nil {
		return nil, err
	}
	if data != nil {
		opts = append(opts, option.WithCredentialsJSON(data))
		log.Println("[push] using credentials from FIREBASE_CREDENTIALS_BASE64")
	} else {
		path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if path == "" {
			log.Println("[push] disabled: neither FIREBASE_CREDENTIALS_BASE64 nor GOOGLE_APPLICATION_CREDENTIALS is set")
			return nil, nil
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("push: credentials file %q is not readable: %w", path, err)
		}
	}
	app, err := firebase.NewApp(ctx, nil, opts...) // with no options it reads the file named in the environment
	if err != nil {
		return nil, fmt.Errorf("push: firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("push: messaging client: %w", err)
	}
	if client == nil {
		return nil, errors.New("push: messaging client is nil")
	}
	log.Println("[push] FCM ready")
	return client, nil
}
