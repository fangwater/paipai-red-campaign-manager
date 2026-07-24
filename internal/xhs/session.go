package xhs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const SessionVersion = 1

type Session struct {
	Version int   `json:"version"`
	Token   Token `json:"token"`
}

func SaveSession(path string, token Token) error {
	if path == "" {
		return errors.New("XHS Spotlight session path is required")
	}
	data, err := json.MarshalIndent(Session{Version: SessionVersion, Token: token}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode XHS Spotlight session: %w", err)
	}
	data = append(data, '\n')
	directory := filepath.Dir(path)
	_, statErr := os.Stat(directory)
	directoryCreated := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !directoryCreated {
		return fmt.Errorf("inspect XHS Spotlight session directory: %w", statErr)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create XHS Spotlight session directory: %w", err)
	}
	if directoryCreated {
		if err := os.Chmod(directory, 0o700); err != nil {
			return fmt.Errorf("secure XHS Spotlight session directory: %w", err)
		}
	}

	temporary, err := os.CreateTemp(directory, ".session-*")
	if err != nil {
		return fmt.Errorf("create temporary XHS Spotlight session: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure temporary XHS Spotlight session: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write XHS Spotlight session: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync XHS Spotlight session: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close XHS Spotlight session: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace XHS Spotlight session: %w", err)
	}
	return nil
}

func LoadSession(path string) (Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("read XHS Spotlight session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode XHS Spotlight session: %w", err)
	}
	if session.Version != SessionVersion {
		return Session{}, fmt.Errorf("unsupported XHS Spotlight session version %d", session.Version)
	}
	if session.Token.AccessToken == "" || session.Token.RefreshToken == "" {
		return Session{}, errors.New("XHS Spotlight session contains empty tokens")
	}
	return session, nil
}

func (client *Client) RefreshSession(ctx context.Context, path string) (Token, error) {
	session, err := LoadSession(path)
	if err != nil {
		return Token{}, err
	}
	refreshed, err := client.RefreshToken(ctx, session.Token.RefreshToken)
	if err != nil {
		return Token{}, err
	}
	if err := SaveSession(path, refreshed); err != nil {
		return Token{}, err
	}
	return refreshed, nil
}
