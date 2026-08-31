package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
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

	// 1. Load Configurations
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[Main] Configuration load error: %v", err)
	}

	log.Printf("[Main] Timezone: %s (Offset: UTC+7)", cfg.Timezone)
	log.Printf("[Main] Database Path: %s", cfg.DBPath)
	log.Printf("[Main] Target WhatsApp JID: %s", cfg.TargetJID)
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

	// 5. Initialize lightweight HTTP status dashboard (Render reads $PORT, default 8080)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			isLoggedIn := bot.IsLoggedIn()
			qrCode := bot.GetLatestQR()

			var qrHTML string
			var statusBadge string
			var metaRefresh string

			if isLoggedIn {
				statusBadge = `<div class="badge badge-success"><span class="dot dot-green"></span> WhatsApp Terhubung (Logged In)</div>`
				qrHTML = `<div class="status-box">
					<h3 style="color:#4ade80;margin:0 0 8px 0;font-size:16px;">✅ WhatsApp Berhasil Terhubung!</h3>
					<p style="margin:0;font-size:13px;color:#94a3b8;">Bot aktif 24/7 mengirimkan pengingat Sholat & Kultum ke grup.</p>
				</div>`
			} else if qrCode != "" {
				metaRefresh = `<meta http-equiv="refresh" content="6">`
				statusBadge = `<div class="badge badge-warning"><span class="dot dot-yellow"></span> Menunggu Scan WhatsApp</div>`
				qrImgURL := "https://api.qrserver.com/v1/create-qr-code/?size=320x320&data=" + url.QueryEscape(qrCode)
				qrHTML = fmt.Sprintf(`<div class="qr-container">
					<p style="margin:0 0 14px 0;font-size:14px;color:#f8fafc;font-weight:600;">Buka WhatsApp di HP ➔ <b>Perangkat Tertaut</b> ➔ Scan QR di bawah:</p>
					<div class="qr-wrapper"><img src="%s" alt="WhatsApp QR Code" class="qr-image" /></div>
					<p style="margin:12px 0 0 0;font-size:12px;color:#94a3b8;">🔄 QR code ini auto-refresh setiap 6 detik.</p>
				</div>`, qrImgURL)
			} else {
				metaRefresh = `<meta http-equiv="refresh" content="3">`
				statusBadge = `<div class="badge"><span class="dot"></span> Menginisialisasi WhatsApp...</div>`
				qrHTML = `<p style="margin:20px 0;color:#94a3b8;">Sedang menghubungkan ke server WhatsApp...</p>`
			}

			const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Masjid Al-Wasii UNILA - WhatsApp Reminder Bot</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    {{META_REFRESH}}
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
        .card { background: #1e293b; border: 1px solid #334155; border-radius: 16px; padding: 28px; max-width: 440px; width: 100%; box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.4); text-align: center; }
        .badge { display: inline-flex; align-items: center; gap: 8px; background: rgba(56, 189, 248, 0.15); color: #38bdf8; border: 1px solid rgba(56, 189, 248, 0.3); padding: 6px 14px; border-radius: 9999px; font-size: 13px; font-weight: 600; margin-bottom: 16px; }
        .badge-success { background: rgba(34, 197, 94, 0.15); color: #4ade80; border-color: rgba(34, 197, 94, 0.3); }
        .badge-warning { background: rgba(234, 179, 8, 0.15); color: #facc15; border-color: rgba(234, 179, 8, 0.3); }
        .dot { width: 8px; height: 8px; background: #38bdf8; border-radius: 50%; display: inline-block; animation: pulse 2s infinite; }
        .dot-green { background: #22c55e; }
        .dot-yellow { background: #eab308; }
        @keyframes pulse { 0%, 100% { opacity: 1; transform: scale(1); } 50% { opacity: 0.5; transform: scale(1.2); } }
        h1 { font-size: 20px; font-weight: 700; margin: 0 0 6px 0; color: #f8fafc; }
        .subtitle { font-size: 13px; color: #94a3b8; line-height: 1.5; margin: 0 0 20px 0; }
        .qr-container { margin: 16px 0; background: #0f172a; padding: 20px; border-radius: 14px; border: 1px solid #334155; }
        .qr-wrapper { display: inline-block; background: #ffffff; padding: 12px; border-radius: 12px; }
        .qr-image { width: 240px; height: 240px; display: block; }
        .status-box { background: #0f172a; border-radius: 12px; padding: 16px; border: 1px solid #334155; margin: 16px 0; }
        .info { background: #0f172a; border-radius: 12px; padding: 14px; text-align: left; font-size: 13px; color: #cbd5e1; border: 1px solid #334155; margin-top: 18px; }
        .info-row { display: flex; justify-content: space-between; margin-bottom: 6px; }
        .info-row:last-child { margin-bottom: 0; }
        .info-label { color: #64748b; }
        .info-value { font-weight: 600; color: #38bdf8; }
    </style>
</head>
<body>
    <div class="card">
        {{STATUS_BADGE}}
        <h1>🕌 Masjid Al-Wasii UNILA</h1>
        <div class="subtitle">Bot Pengingat Sholat & Kultum Marbot 24/7</div>
        {{QR_CONTENT}}
        <div class="info">
            <div class="info-row"><span class="info-label">Database:</span><span class="info-value">Turso SQLite Cloud ☁️</span></div>
            <div class="info-row"><span class="info-label">Lokasi:</span><span class="info-value">Bandar Lampung (1014)</span></div>
            <div class="info-row"><span class="info-label">Timezone:</span><span class="info-value">Asia/Jakarta (WIB)</span></div>
        </div>
    </div>
</body>
</html>`

			page := strings.ReplaceAll(htmlTemplate, "{{META_REFRESH}}", metaRefresh)
			page = strings.ReplaceAll(page, "{{STATUS_BADGE}}", statusBadge)
			page = strings.ReplaceAll(page, "{{QR_CONTENT}}", qrHTML)

			_, _ = w.Write([]byte(page))
		}),
	}

	go func() {
		log.Printf("[HTTP] Status server listening on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	// 6. Initialize & Start Scheduler
	sched := scheduler.NewScheduler(cfg, store, reg, bot)
	if err := sched.Start(); err != nil {
		log.Printf("[Main] Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// 7. Connect WhatsApp client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bot.Start(ctx); err != nil {
		log.Printf("[Main] WhatsApp client connection error: %v", err)
	}

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
