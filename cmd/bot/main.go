package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	// 2. Initialize Persistent Storage (Turso libSQL Cloud or Local SQLite)
	store, err := storage.NewStorage(cfg.DBPath, cfg.TursoURL, cfg.TursoToken)
	if err != nil {
		log.Fatalf("[Main] Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// 3. Initialize Phonebook registry & Duty Matrix (with DB sync / auto-seeding)
	localMembers := phonebook.LoadRegistry(cfg.MembersFile, cfg.MembersJSON).GetAllMembers()
	localSchedule := matrix.LoadSchedule(cfg.ScheduleFile, cfg.ScheduleJSON)

	store.AutoSeedInitialData(localMembers, localSchedule)

	var reg *phonebook.Registry
	if dbMembers, err := store.LoadMembers(); err == nil && len(dbMembers) > 0 {
		reg = phonebook.NewRegistryFromMembers(dbMembers)
		log.Printf("[Main] Loaded %d registered community members from database.", len(dbMembers))
	} else {
		reg = phonebook.NewRegistryFromMembers(localMembers)
		log.Printf("[Main] Loaded %d registered community members into phonebook.", len(reg.GetAllMembers()))
	}

	if dbMatrix, err := store.LoadDutyMatrix(); err == nil && len(dbMatrix) > 0 {
		if dbQueue, err := store.LoadKultumQueue(); err == nil && len(dbQueue) > 0 {
			sc := &matrix.ScheduleConfig{
				KultumQueue: dbQueue,
				WeeklyMatrix: matrix.WeeklyMatrixRaw{
					Monday:    dbMatrix[time.Monday],
					Tuesday:   dbMatrix[time.Tuesday],
					Wednesday: dbMatrix[time.Wednesday],
					Thursday:  dbMatrix[time.Thursday],
					Friday:    dbMatrix[time.Friday],
					Saturday:  dbMatrix[time.Saturday],
					Sunday:    dbMatrix[time.Sunday],
				},
			}
			matrix.SetActiveSchedule(sc)
		}
	}
	log.Printf("[Main] Loaded duty matrix with %d kultum rotation speakers.", matrix.KultumQueueLen())

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

	// 7. Initialize lightweight HTTP status dashboard (Render reads $PORT, default 8080)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Masjid Al-Wasii UNILA - WhatsApp Reminder Bot</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
        .card { background: #1e293b; border: 1px solid #334155; border-radius: 16px; padding: 32px; max-width: 480px; width: 100%; box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3); text-align: center; }
        .badge { display: inline-flex; align-items: center; gap: 8px; background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); padding: 6px 14px; border-radius: 9999px; font-size: 14px; font-weight: 600; margin-bottom: 20px; }
        .dot { width: 8px; height: 8px; background: #22c55e; border-radius: 50%; display: inline-block; animation: pulse 2s infinite; }
        @keyframes pulse { 0%, 100% { opacity: 1; transform: scale(1); } 50% { opacity: 0.5; transform: scale(1.2); } }
        h1 { font-size: 22px; font-weight: 700; margin: 0 0 10px 0; color: #f8fafc; }
        p { font-size: 14px; color: #94a3b8; line-height: 1.6; margin: 0 0 20px 0; }
        .info { background: #0f172a; border-radius: 10px; padding: 14px; text-align: left; font-size: 13px; color: #cbd5e1; border: 1px solid #334155; }
        .info-row { display: flex; justify-content: space-between; margin-bottom: 6px; }
        .info-row:last-child { margin-bottom: 0; }
        .info-label { color: #64748b; }
        .info-value { font-weight: 600; color: #38bdf8; }
    </style>
</head>
<body>
    <div class="card">
        <div class="badge"><span class="dot"></span> Bot Aktif 24/7</div>
        <h1>🕌 Masjid Al-Wasii UNILA</h1>
        <p>Bot Pengingat Sholat & Kultum Otomatis WhatsApp berjalan aktif di latar belakang.</p>
        <div class="info">
            <div class="info-row"><span class="info-label">Lokasi:</span><span class="info-value">Bandar Lampung (1014)</span></div>
            <div class="info-row"><span class="info-label">Status:</span><span class="info-value">Connected 🟢</span></div>
            <div class="info-row"><span class="info-label">Timezone:</span><span class="info-value">Asia/Jakarta (WIB)</span></div>
        </div>
    </div>
</body>
</html>`))
		}),
	}

	go func() {
		log.Printf("[HTTP] Status server listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	// 8. Setup graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	log.Println("[Main] Bot service is running. Press Ctrl+C to terminate.")
	sig := <-sigChan
	log.Printf("[Main] Received termination signal (%s). Shutting down gracefully...", sig)

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	_ = httpServer.Shutdown(shutdownCtx)
	sched.Stop()
	bot.Stop()
	<-shutdownCtx.Done()

	fmt.Println("[Main] Bot stopped cleanly. Wassalamu'alaikum.")
}
