package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Configuration defaults
const (
	DefaultOutputDir       = "./output"
	DefaultIntervalMinutes = 60
	DefaultAccountsFile    = "accounts.yaml"
	DefaultRunOnInterval   = false
	DefaultIdle            = false
)

type Config struct {
	OutputDir       string
	IntervalMinutes int
	AccountsFile    string
	RunOnInterval   bool
	Idle            bool
}

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

func loadConfig() Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, we just log it for debugging purposes
		// or ignore it silently as we have defaults.
		// log.Printf("No .env file found")
	}

	cfg := Config{
		OutputDir:       os.Getenv("BACKUP_OUTPUT_DIR"),
		AccountsFile:    os.Getenv("BACKUP_ACCOUNTS_FILE"),
		RunOnInterval:   DefaultRunOnInterval,
		Idle:            DefaultIdle,
	}

	if cfg.OutputDir == "" {
		cfg.OutputDir = DefaultOutputDir
	}
	if cfg.AccountsFile == "" {
		cfg.AccountsFile = DefaultAccountsFile
	}

	intervalStr := os.Getenv("BACKUP_INTERVAL_MINUTES")
	if intervalStr == "" {
		cfg.IntervalMinutes = DefaultIntervalMinutes
	} else {
		val, err := strconv.Atoi(intervalStr)
		if err != nil {
			log.Printf("Invalid BACKUP_INTERVAL_MINUTES, using default: %v", err)
			cfg.IntervalMinutes = DefaultIntervalMinutes
		} else {
			cfg.IntervalMinutes = val
		}
	}

	runOnIntervalStr := os.Getenv("BACKUP_RUN_ON_INTERVAL")
	if runOnIntervalStr != "" {
		val, err := strconv.ParseBool(runOnIntervalStr)
		if err != nil {
			log.Printf("Invalid BACKUP_RUN_ON_INTERVAL, using default (%v): %v", DefaultRunOnInterval, err)
		} else {
			cfg.RunOnInterval = val
		}
	}

	idleStr := os.Getenv("BACKUP_IDLE")
	if idleStr != "" {
		val, err := strconv.ParseBool(idleStr)
		if err != nil {
			log.Printf("Invalid BACKUP_IDLE, using default (%v): %v", DefaultIdle, err)
		} else {
			cfg.Idle = val
		}
	}

	return cfg
}

func readAccounts(path string) ([]Account, error) {
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

func sanitizeName(name string) string {
	safe := strings.ReplaceAll(name, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	return safe
}

func connect(acc Account) (*client.Client, error) {
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

func backupAccount(acc Account, outputDir string) {
	log.Printf("Starting backup for account: %s", acc.Name)

	c, err := connect(acc)
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

	accountDir := filepath.Join(outputDir, sanitizeName(acc.Name))

	for _, mbox := range mboxes {
		syncMailbox(c, mbox.Name, accountDir)
	}
}

func syncMailbox(c *client.Client, mboxName string, accountDir string) {
	// log.Printf("Processing mailbox: %s", mboxName) // Reduced verbosity
	
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

	mboxDir := filepath.Join(accountDir, sanitizeName(mboxName))
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

func monitorAccount(acc Account, outputDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Starting IDLE monitor for account: %s", acc.Name)

	for {
		c, err := connect(acc)
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
				accountDir := filepath.Join(outputDir, sanitizeName(acc.Name))
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

func main() {
	config := loadConfig()

	log.Printf("Starting IMAP Backup")
	log.Printf("Output Directory: %s", config.OutputDir)
	log.Printf("Accounts File: %s", config.AccountsFile)
	log.Printf("Interval Mode: %v (%d mins)", config.RunOnInterval, config.IntervalMinutes)
	log.Printf("IDLE Mode: %v", config.Idle)

	// Initial backup of all accounts/folders
	accounts, err := readAccounts(config.AccountsFile)
	if err != nil {
		log.Fatalf("Error reading accounts file: %v", err)
	}

	log.Println("Running initial backup...")
	for _, acc := range accounts {
		backupAccount(acc, config.OutputDir)
	}
	log.Println("Initial backup complete.")

	if config.Idle {
		var wg sync.WaitGroup
		for _, acc := range accounts {
			wg.Add(1)
			go monitorAccount(acc, config.OutputDir, &wg)
		}
		
		// If both modes are on, we also run the interval loop in the background?
		// Or does IDLE supersede?
		// Let's run interval in a separate goroutine if enabled
		if config.RunOnInterval {
			go func() {
				for {
					time.Sleep(time.Duration(config.IntervalMinutes) * time.Minute)
					log.Printf("Running interval backup...")
					// Re-read accounts in case they changed? For now, stick to loaded
					for _, acc := range accounts {
						backupAccount(acc, config.OutputDir)
					}
					log.Printf("Interval backup complete.")
				}
			}()
		}

		wg.Wait() // Block forever on IDLE monitors
	} else if config.RunOnInterval {
		for {
			log.Printf("Sleeping for %d minutes...", config.IntervalMinutes)
			time.Sleep(time.Duration(config.IntervalMinutes) * time.Minute)
			
			log.Printf("Running interval backup...")
			for _, acc := range accounts {
				backupAccount(acc, config.OutputDir)
			}
			log.Printf("Backup cycle complete.")
		}
	} else {
		log.Println("Done.")
	}
}
