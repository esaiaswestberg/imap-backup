package restore

import (
	"bytes"
	"log"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/esaiaswestberg/imap-backup/pkg/core"
	"github.com/emersion/go-imap/client"
)

func Run(acc core.Account, outputDir string) {
	log.Printf("Starting RESTORE for account: %s", acc.Name)

	accountDir := filepath.Join(outputDir, core.SanitizeName(acc.Name))
	if _, err := os.Stat(accountDir); os.IsNotExist(err) {
		log.Printf("Backup directory for account %s not found at %s", acc.Name, accountDir)
		return
	}

	c, err := core.Connect(acc)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", acc.Name, err)
		return
	}
	defer c.Logout()

	entries, err := os.ReadDir(accountDir)
	if err != nil {
		log.Printf("Failed to read account directory %s: %v", accountDir, err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			mboxName := entry.Name() // Use directory name as mailbox name
			mboxPath := filepath.Join(accountDir, mboxName)
			log.Printf("Restoring mailbox: %s", mboxName)

			// Ensure mailbox exists
			if err := c.Create(mboxName); err != nil {
				// Ignore error if it already exists
			}

			files, err := os.ReadDir(mboxPath)
			if err != nil {
				log.Printf("Failed to read mailbox directory %s: %v", mboxPath, err)
				continue
			}

			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(file.Name(), ".eml") {
					restoreMessage(c, mboxName, filepath.Join(mboxPath, file.Name()))
				}
			}
		}
	}
	log.Printf("Restore complete for account: %s", acc.Name)
}

func restoreMessage(c *client.Client, mboxName, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read message file %s: %v", filePath, err)
		return
	}

	// Parse message to get Date
	r := bytes.NewReader(content)
	m, err := mail.ReadMessage(r)
	var date time.Time
	if err == nil {
		headerDate := m.Header.Get("Date")
		parsedDate, err := mail.ParseDate(headerDate)
		if err == nil {
			date = parsedDate
		} else {
			date = time.Now()
		}
	} else {
		date = time.Now()
	}

	// Reset reader for Append
	r.Seek(0, 0)

	// Append to mailbox
	if err := c.Append(mboxName, nil, date, r); err != nil {
		log.Printf("Failed to append message %s to %s: %v", filePath, mboxName, err)
	}
}
