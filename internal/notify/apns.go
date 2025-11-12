package notify

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
)

// Sender describes a push notification sender.
type Sender interface {
	Send(deviceToken, title, body string, custom map[string]any) error
}

// APNS implements Sender via Apple Push Notification service.
type APNS struct {
	client *apns2.Client
	topic  string
}

// FromEnv builds an APNS client using environment variables.
func FromEnv() (*APNS, error) {
	keyPath := strings.TrimSpace(os.Getenv("APNS_P8_PATH"))
	keyID := strings.TrimSpace(os.Getenv("APNS_KEY_ID"))
	teamID := strings.TrimSpace(os.Getenv("APNS_TEAM_ID"))
	topic := strings.TrimSpace(os.Getenv("APNS_BUNDLE_ID"))
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APNS_ENV")))

	switch {
	case keyPath == "":
		return nil, fmt.Errorf("APNS_P8_PATH must be set")
	case keyID == "":
		return nil, fmt.Errorf("APNS_KEY_ID must be set")
	case teamID == "":
		return nil, fmt.Errorf("APNS_TEAM_ID must be set")
	case topic == "":
		return nil, fmt.Errorf("APNS_BUNDLE_ID must be set")
	case env == "":
		return nil, fmt.Errorf("APNS_ENV must be set")
	}

	authKey, err := loadAuthKey(keyPath)
	if err != nil {
		return nil, err
	}

	token := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}
	client := apns2.NewTokenClient(token)
	switch env {
	case "production":
		// default client already targets production
	case "sandbox":
		client = client.Development()
	default:
		return nil, fmt.Errorf("invalid APNS_ENV %q (want sandbox|production)", env)
	}

	return &APNS{client: client, topic: topic}, nil
}

// Send sends a push notification with alert fields and custom payload.
func (a *APNS) Send(deviceToken, title, body string, custom map[string]any) error {
	if a == nil || a.client == nil {
		return fmt.Errorf("apns client not initialized")
	}
	pl := payload.NewPayload().
		AlertTitle(title).
		AlertBody(body)
	for k, v := range custom {
		pl.Custom(k, v)
	}
	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.topic,
		Payload:     pl,
	}
	res, err := a.client.Push(notification)
	if err != nil {
		log.Printf("apns push failed: status=0 reason=%v token=%s host=%s", err, deviceToken, a.client.Host)
		return err
	}
	if res == nil {
		log.Printf("apns push failed: status=0 reason=response_nil token=%s", deviceToken)
		return fmt.Errorf("apns response is nil")
	}
	if !res.Sent() {
		log.Printf("apns push failed: status=%d reason=%s token=%s", res.StatusCode, res.Reason, deviceToken)
		return fmt.Errorf("apns push failed with status %d: %s", res.StatusCode, res.Reason)
	}
	return nil
}

func loadAuthKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read APNs key: %w", err)
	}
	key, err := token.AuthKeyFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse APNs key: %w", err)
	}
	return key, nil
}
