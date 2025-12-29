package main

import (
	"flag"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/esaiaswestberg/imap-backup/pkg/backup"
	"github.com/esaiaswestberg/imap-backup/pkg/config"
	"github.com/esaiaswestberg/imap-backup/pkg/core"
	"github.com/esaiaswestberg/imap-backup/pkg/restore"
)

func main() {
	// Flag parsing
	restoreAccountName := flag.String("restore-account", "", "Restore specific account by name or 'all'")
	restoreAccountsList := flag.String("restore-accounts", "", "Comma-separated list of account names to restore")
	flag.Parse()

	cfg := config.Load()

	// Handle Restore Mode
	if *restoreAccountName != "" || *restoreAccountsList != "" {
		log.Printf("Starting IMAP Restore Mode")
		accounts, err := core.ReadAccounts(cfg.AccountsFile)
		if err != nil {
			log.Fatalf("Error reading accounts file: %v", err)
		}

		targetAccounts := make(map[string]bool)
		if *restoreAccountName == "all" {
			for _, acc := range accounts {
				targetAccounts[acc.Name] = true
			}
		} else if *restoreAccountName != "" {
			targetAccounts[*restoreAccountName] = true
		}

		if *restoreAccountsList != "" {
			names := strings.Split(*restoreAccountsList, ",")
			for _, name := range names {
				targetAccounts[strings.TrimSpace(name)] = true
			}
		}

		for _, acc := range accounts {
			if targetAccounts[acc.Name] {
				restore.Run(acc, cfg.OutputDir)
			}
		}
		return
	}

	// Normal Backup Mode
	log.Printf("Starting IMAP Backup")
	log.Printf("Output Directory: %s", cfg.OutputDir)
	log.Printf("Accounts File: %s", cfg.AccountsFile)
	log.Printf("Interval Mode: %v (%d mins)", cfg.RunOnInterval, cfg.IntervalMinutes)
	log.Printf("IDLE Mode: %v", cfg.Idle)

	// Initial backup of all accounts/folders
	accounts, err := core.ReadAccounts(cfg.AccountsFile)
	if err != nil {
		log.Fatalf("Error reading accounts file: %v", err)
	}

	log.Println("Running initial backup...")
	for _, acc := range accounts {
		backup.Run(acc, cfg.OutputDir)
	}
	log.Println("Initial backup complete.")

	if cfg.Idle {
		var wg sync.WaitGroup
		for _, acc := range accounts {
			wg.Add(1)
			go backup.Monitor(acc, cfg.OutputDir, &wg)
		}
		
		if cfg.RunOnInterval {
			go func() {
				for {
					time.Sleep(time.Duration(cfg.IntervalMinutes) * time.Minute)
					log.Printf("Running interval backup...")
					for _, acc := range accounts {
						backup.Run(acc, cfg.OutputDir)
					}
					log.Printf("Interval backup complete.")
				}
			}()
		}

		wg.Wait()
	} else if cfg.RunOnInterval {
		for {
			log.Printf("Sleeping for %d minutes...", cfg.IntervalMinutes)
			time.Sleep(time.Duration(cfg.IntervalMinutes) * time.Minute)
			
			log.Printf("Running interval backup...")
			for _, acc := range accounts {
				backup.Run(acc, cfg.OutputDir)
			}
			log.Printf("Backup cycle complete.")
		}
	} else {
		log.Println("Done.")
	}
}
