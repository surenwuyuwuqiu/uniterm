package session

import (
	"os"

	"golang.org/x/crypto/ssh"
)

func makeSSHAuthMethods(config ConnectionConfig) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	switch config.AuthType {
	case "password":
		methods = append(methods, ssh.Password(config.Password))
	case "key":
		key, err := os.ReadFile(config.KeyPath)
		if err != nil {
			methods = append(methods, ssh.Password(config.Password))
			break
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			methods = append(methods, ssh.Password(config.Password))
			break
		}
		methods = append(methods, ssh.PublicKeys(signer))
	case "agent":
		methods = append(methods, ssh.Password(config.Password))
	default:
		methods = append(methods, ssh.Password(config.Password))
	}

	return methods
}
