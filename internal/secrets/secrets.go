package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/99designs/keyring"
	"github.com/adrg/xdg"
)

const serviceName = "jdeen-cli"

var ErrNotFound = errors.New("session is not configured")

type Session struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	SessionID        string    `json:"session_id"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type Store interface {
	Get(profile string) (Session, error)
	Set(profile string, session Session) error
	Delete(profile string) error
}

type KeyringStore struct{ keyring keyring.Keyring }

func Open() (Store, error) {
	dir, err := xdg.ConfigFile(filepath.Join("jdeen", "keyring"))
	if err != nil {
		return nil, fmt.Errorf("resolve keyring path: %w", err)
	}
	ring, err := keyring.Open(keyring.Config{
		ServiceName: serviceName,
		FileDir:     dir,
		FilePasswordFunc: func(string) (string, error) {
			password := os.Getenv("JDEEN_KEYRING_PASSWORD")
			if password == "" {
				return "", errors.New("file keyring backend requires JDEEN_KEYRING_PASSWORD")
			}
			return password, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}
	return KeyringStore{keyring: ring}, nil
}

func (store KeyringStore) Get(profile string) (Session, error) {
	item, err := store.keyring.Get(key(profile))
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var session Session
	if err := json.Unmarshal(item.Data, &session); err != nil {
		return Session{}, fmt.Errorf("decode stored session: %w", err)
	}
	return session, nil
}

func (store KeyringStore) Set(profile string, session Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return store.keyring.Set(keyring.Item{Key: key(profile), Data: data})
}

func (store KeyringStore) Delete(profile string) error {
	err := store.keyring.Remove(key(profile))
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil
	}
	return err
}

func key(profile string) string { return "session:" + profile }
