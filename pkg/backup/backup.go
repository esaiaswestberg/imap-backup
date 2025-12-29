package backup

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/esaiaswestberg/imap-backup/pkg/core"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
)

func Run(acc core.Account, outputDir string) {
	log.Printf("Starting backup for account: %s", acc.Name)

	c, err := core.Connect(acc)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", acc.Name, err)
		return
	}
	defer c.Logout()

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	var mboxes []*imap.MailboxInfo
	for m := range mailboxes {
		mboxes = append(mboxes, m)
	}
	if err := <-done; err != nil {
		log.Printf("Failed to list mailboxes for %s: %v", acc.Name, err)
		return
	}

	accountDir := filepath.Join(outputDir, core.SanitizeName(acc.Name))

	for _, mbox := range mboxes {
		syncMailbox(c, mbox.Name, accountDir)
	}
}

func syncMailbox(c *client.Client, mboxName string, accountDir string) {
	_, err := c.Select(mboxName, true) // Read-only
	if err != nil {
		log.Printf("Failed to select mailbox %s: %v", mboxName, err)
		return
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.DeletedFlag} 
	ids, err := c.Search(criteria)
	if err != nil {
		log.Printf("Failed to search mailbox %s: %v", mboxName, err)
		return
	}

	if len(ids) == 0 {
		return
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(ids...)

	messages := make(chan *imap.Message, 100)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, []imap.FetchItem{imap.FetchUid}, messages)
	}()

	mboxDir := filepath.Join(accountDir, core.SanitizeName(mboxName))
	if err := os.MkdirAll(mboxDir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %v", mboxDir, err)
		return
	}

	var uidsToFetch []uint32
	for msg := range messages {
		filename := filepath.Join(mboxDir, fmt.Sprintf("%d.eml", msg.Uid))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			uidsToFetch = append(uidsToFetch, msg.Uid)
		}
	}

	if err := <-done; err != nil {
		log.Printf("Failed to fetch UIDs for %s: %v", mboxName, err)
		return
	}

	if len(uidsToFetch) > 0 {
		log.Printf("Downloading %d new messages from %s", len(uidsToFetch), mboxName)
		fetchAndSaveMessages(c, uidsToFetch, mboxDir)
	}
}

func fetchAndSaveMessages(c *client.Client, uids []uint32, mboxDir string) {
	batchSize := 100
	for i := 0; i < len(uids); i += batchSize {
		end := i + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		batch := uids[i:end]

		seqSet := new(imap.SeqSet)
		seqSet.AddNum(batch...)

		section := &imap.BodySectionName{}
		messages := make(chan *imap.Message, 10)
		done := make(chan error, 1)
		go func() {
			done <- c.UidFetch(seqSet, []imap.FetchItem{imap.FetchUid, section.FetchItem()}, messages)
		}()

		for msg := range messages {
			r := msg.GetBody(section)
			if r == nil {
				log.Printf("Server didn't return message body for UID %d", msg.Uid)
				continue
			}

			filename := filepath.Join(mboxDir, fmt.Sprintf("%d.eml", msg.Uid))
			f, err := os.Create(filename)
			if err != nil {
				log.Printf("Failed to create file %s: %v", filename, err)
				continue
			}
			
			if _, err := io.Copy(f, r); err != nil {
				log.Printf("Failed to write to file %s: %v", filename, err)
				f.Close()
				os.Remove(filename)
				continue
			}
			f.Close()
		}

		if err := <-done; err != nil {
			log.Printf("Batch fetch failed: %v", err)
		}
	}
}

func Monitor(acc core.Account, outputDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Starting IDLE monitor for account: %s", acc.Name)

	for {
		c, err := core.Connect(acc)
		if err != nil {
			log.Printf("Monitor connection failed for %s: %v. Retrying in 1 minute...", acc.Name, err)
			time.Sleep(1 * time.Minute)
			continue
		}

		// Select INBOX
		if _, err := c.Select("INBOX", false); err != nil {
			log.Printf("Monitor failed to select INBOX for %s: %v", acc.Name, err)
			c.Logout()
			time.Sleep(1 * time.Minute)
			continue
		}

		idleClient := idle.NewClient(c)
		
		// Create a channel to receive mailbox updates
		updates := make(chan client.Update, 10) // Buffer updates
		c.Updates = updates

		// Start IDLE
		done := make(chan error, 1)
		stop := make(chan struct{})
		go func() {
			done <- idleClient.IdleWithFallback(stop, 0) // 0 means default timeout (usually 29 mins)
		}()

		log.Printf("Monitoring INBOX for %s...", acc.Name)

		// Wait for updates or error
		shouldReconnect := false
		
		select {
		case <-updates:
			// Stop IDLE to sync
			close(stop)
			
			// Drain remaining updates until IDLE finishes to prevent deadlock
			draining := true
			for draining {
				select {
				case <-updates:
					// discard
				case err := <-done:
					draining = false
					if err != nil {
						log.Printf("IDLE finished with error for %s: %v", acc.Name, err)
						shouldReconnect = true
					}
				}
			}

			if !shouldReconnect {
				log.Printf("New activity detected for %s. Syncing INBOX...", acc.Name)
				
				// Disable updates during sync to avoid unexpected channel sends
				c.Updates = nil
				
				// Sync INBOX
				accountDir := filepath.Join(outputDir, core.SanitizeName(acc.Name))
				syncMailbox(c, "INBOX", accountDir)
				
				// Loop back to restart IDLE
				continue 
			}

		case err := <-done:
			log.Printf("IDLE stopped unexpectedly for %s: %v", acc.Name, err)
			shouldReconnect = true
		}

		if shouldReconnect {
			c.Logout()
			log.Printf("Reconnecting monitor for %s in 10 seconds...", acc.Name)
			time.Sleep(10 * time.Second)
		}
	}
}
