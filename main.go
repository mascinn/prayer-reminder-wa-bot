package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"remind-bot/internal/config"
	"remind-bot/internal/matrix"
	"remind-bot/internal/phonebook"
	"remind-bot/internal/scheduler"
	"remind-bot/internal/storage"
	"remind-bot/internal/whatsapp"
)

func main() {
	log.Println("==================================================================")
	log.Println("  Masjid Al-Wasii UNILA - WhatsApp Reminder Bot (24/7 Service)   ")
	log.Println("==================================================================")

	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[Main] Failed to load configuration: %v", err)
	}

	log.Printf("[Main] Timezone: %s (Offset: UTC+7)", cfg.Timezone)
	log.Printf("[Main] Database Path: %s", cfg.DBPath)
	log.Printf("[Main] Target WhatsApp JID: %s", func() string {
		if cfg.TargetJID == "" {
			return "(Not set - run '!jid' in group to find and set TARGET_JID)"
		}
		return cfg.TargetJID
	}())
	log.Printf("[Main] Friday Reminder Enabled: %t", cfg.EnableJumatReminder)
	log.Printf("[Main] Kemenag City ID: %s (Kota Bandar Lampung / Rajabasa / UNILA)", cfg.CityID)

	// 2. Initialize Phonebook registry & Duty Matrix
	reg := phonebook.LoadRegistry(cfg.MembersFile, cfg.MembersJSON)
	log.Printf("[Main] Loaded %d registered community members into phonebook.", len(reg.GetAllMembers()))

	matrixCfg := matrix.LoadSchedule(cfg.ScheduleFile, cfg.ScheduleJSON)
	log.Printf("[Main] Loaded duty matrix with %d kultum rotation speakers.", matrixCfg.KultumQueueLen())

	// 3. Initialize SQLite state & auth storage
	store, err := storage.NewStorage(cfg.DBPath)
	if err != nil {
		log.Fatalf("[Main] Failed to initialize SQLite storage: %v", err)
	}
	defer store.Close()

	kultumIdx, err := store.GetKultumIndex()
	if err != nil {
		log.Printf("[Main] Warning reading kultum index: %v", err)
	}
	log.Printf("[Main] Current Kultum Speaker: %s (Index: %d of %d)",
		matrix.GetKultumSpeaker(kultumIdx), kultumIdx+1, matrix.KultumQueueLen())

	// 4. Initialize WhatsApp bot client
	bot, err := whatsapp.NewBot(cfg, store, reg)
	if err != nil {
		log.Fatalf("[Main] Failed to initialize WhatsApp client: %v", err)
	}
	defer bot.Stop()

	// 5. Initialize & Start Scheduler
	sched := scheduler.NewScheduler(cfg, store, reg, bot)
	if err := sched.Start(); err != nil {
		log.Fatalf("[Main] Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// 6. Connect WhatsApp client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bot.Start(ctx); err != nil {
		log.Fatalf("[Main] WhatsApp client connection error: %v", err)
	}

	// 7. Setup graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Println("[Main] Bot service is running. Press Ctrl+C to terminate.")
	sig := <-sigChan
	log.Printf("[Main] Received termination signal (%s). Shutting down gracefully...", sig)

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	sched.Stop()
	bot.Stop()
	<-shutdownCtx.Done()

	fmt.Println("[Main] Bot stopped cleanly. Wassalamu'alaikum.")
}
