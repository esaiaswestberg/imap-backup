package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/emersion/go-imap/client"
	"gopkg.in/yaml.v3"
)

type Account struct {
	Name        string `yaml:"name"`
	Credentials struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"credentials"`
	Server struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Security string `yaml:"security"`
	} `yaml:"server"`
}

func ReadAccounts(path string) ([]Account, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	if err := yaml.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func SanitizeName(name string) string {
	safe := strings.ReplaceAll(name, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	return safe
}

func Connect(acc Account) (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", acc.Server.Host, acc.Server.Port)
	var c *client.Client
	var err error

	if acc.Server.Security == "SSL/TLS" {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		return nil, err
	}

	if err := c.Login(acc.Credentials.Username, acc.Credentials.Password); err != nil {
		c.Logout()
		return nil, err
	}
	return c, nil
}
