// Migration script: update Turso production DB
// Operasi:
//   1. Hapus Torik dari kultum_queue, rotate supaya besok (tgl 27) = Ananda
//   2. Ganti semua slot "Torik"/"Thoriq" di duty_schedules → "Firdaus"
//   3. Tambah member baru "Firdaus" (no: 6283141877481)
//
// Jalankan: go run ./cmd/migrate_kultum/main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func main() {
	_ = godotenv.Load()

	var db *sql.DB
	var err error

	tursoURL := getEnv("TURSO_DATABASE_URL", getEnv("TURSO_URL", ""))
	tursoToken := getEnv("TURSO_AUTH_TOKEN", getEnv("TURSO_TOKEN", ""))
	dbPath := getEnv("DB_PATH", "./data/bot.db")

	if strings.TrimSpace(tursoURL) != "" && strings.TrimSpace(tursoToken) != "" {
		dsn := fmt.Sprintf("%s?authToken=%s", tursoURL, tursoToken)
		db, err = sql.Open("libsql", dsn)
		if err != nil {
			log.Fatalf("❌ Failed to open Turso: %v", err)
		}
		if err := db.Ping(); err != nil {
			log.Fatalf("❌ Failed to connect Turso: %v", err)
		}
		log.Println("✅ Connected to Turso Cloud database.")
	} else {
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			log.Fatalf("❌ Failed to open SQLite: %v", err)
		}
		log.Printf("✅ Connected to local SQLite: %s", dbPath)
	}
	defer db.Close()

	// ================================================================
	// OPERASI 1: Update kultum_queue
	// ================================================================
	log.Println("\n--- OPERASI 1: Update kultum_queue ---")

	rows, err := db.Query("SELECT member_name FROM kultum_queue WHERE is_active = 1 ORDER BY queue_order ASC")
	if err != nil {
		log.Fatalf("❌ Failed to read kultum_queue: %v", err)
	}
	var queue []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			queue = append(queue, name)
		}
	}
	rows.Close()

	log.Printf("Queue lama (%d anggota): %v", len(queue), queue)

	// Hapus Torik/Thoriq dari queue
	var newQueue []string
	for _, name := range queue {
		if !strings.EqualFold(name, "torik") && !strings.EqualFold(name, "thoriq") {
			newQueue = append(newQueue, name)
		} else {
			log.Printf("  → Menghapus '%s' dari kultum_queue", name)
		}
	}
	log.Printf("Queue setelah hapus Torik (%d anggota): %v", len(newQueue), newQueue)

	// Hitung rotasi supaya besok (tgl 27) = Ananda
	tomorrowDay := time.Now().Day() + 1
	n := len(newQueue)
	if n == 0 {
		log.Fatal("❌ Queue kosong setelah hapus Torik!")
	}

	targetIdx := (tomorrowDay - 1) % n

	// Cari posisi Ananda saat ini
	anandaIdx := -1
	for i, name := range newQueue {
		if strings.EqualFold(name, "ananda") {
			anandaIdx = i
			break
		}
	}
	if anandaIdx == -1 {
		log.Fatal("❌ 'Ananda' tidak ditemukan di queue!")
	}

	log.Printf("Besok tgl %d, target index=%d, Ananda saat ini di index=%d", tomorrowDay, targetIdx, anandaIdx)

	// Hitung rotasi: shift left by rotationAmount
	// Setelah rotate left N: elemen[i] pindah ke [(i - N + len) % len]
	// Kita mau anandaIdx → targetIdx
	// targetIdx = (anandaIdx - rotateLeft + n) % n
	// rotateLeft = (anandaIdx - targetIdx + n) % n
	rotateLeft := (anandaIdx - targetIdx + n) % n
	log.Printf("Rotasi kiri sebesar %d posisi", rotateLeft)

	// Lakukan rotate left
	rotated := append(newQueue[rotateLeft:], newQueue[:rotateLeft]...)
	log.Printf("Queue final: %v", rotated)

	// Verifikasi
	verifyIdx := (tomorrowDay - 1) % len(rotated)
	log.Printf("✅ Verifikasi: besok tgl %d → index %d → %s", tomorrowDay, verifyIdx, rotated[verifyIdx])
	if !strings.EqualFold(rotated[verifyIdx], "ananda") {
		log.Fatalf("❌ Verifikasi gagal! Besok harusnya Ananda tapi dapat: %s", rotated[verifyIdx])
	}

	// Simpan ke DB
	if _, err := db.Exec("DELETE FROM kultum_queue"); err != nil {
		log.Fatalf("❌ Failed to delete kultum_queue: %v", err)
	}
	for i, name := range rotated {
		if _, err := db.Exec("INSERT INTO kultum_queue (queue_order, member_name, is_active) VALUES (?, ?, 1)", i+1, name); err != nil {
			log.Fatalf("❌ Failed to insert %s: %v", name, err)
		}
	}
	log.Println("✅ kultum_queue berhasil diperbarui!")

	// ================================================================
	// OPERASI 2: Ganti "Torik"/"Thoriq" → "Firdaus" di duty_schedules
	// ================================================================
	log.Println("\n--- OPERASI 2: Ganti Torik → Firdaus di duty_schedules ---")

	// Update adzan_member
	res, err := db.Exec(`UPDATE duty_schedules SET adzan_member = 'Firdaus' WHERE LOWER(adzan_member) IN ('torik', 'thoriq')`)
	if err != nil {
		log.Fatalf("❌ Failed to update adzan_member: %v", err)
	}
	n1, _ := res.RowsAffected()

	// Update imam_member
	res, err = db.Exec(`UPDATE duty_schedules SET imam_member = 'Firdaus' WHERE LOWER(imam_member) IN ('torik', 'thoriq')`)
	if err != nil {
		log.Fatalf("❌ Failed to update imam_member: %v", err)
	}
	n2, _ := res.RowsAffected()

	log.Printf("✅ Berhasil update %d slot adzan + %d slot imam: Torik → Firdaus", n1, n2)

	// ================================================================
	// OPERASI 3: Tambah member Firdaus
	// ================================================================
	log.Println("\n--- OPERASI 3: Tambah member Firdaus ---")

	phones := []string{"6283141877481"}
	aliases := []string{"firdaus"}

	phonesJSON, _ := json.Marshal(phones)
	aliasesJSON, _ := json.Marshal(aliases)

	_, err = db.Exec(`
		INSERT INTO members (display_name, phones_json, aliases_json, is_active, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(display_name) DO UPDATE SET
			phones_json  = excluded.phones_json,
			aliases_json = excluded.aliases_json,
			is_active    = 1,
			updated_at   = excluded.updated_at;
	`, "Firdaus", string(phonesJSON), string(aliasesJSON), time.Now().UTC())
	if err != nil {
		log.Fatalf("❌ Failed to upsert Firdaus: %v", err)
	}
	log.Println("✅ Member Firdaus berhasil ditambahkan (phone: 6283141877481)")

	// ================================================================
	// SUMMARY
	// ================================================================
	log.Println("\n========================================")
	log.Println("✅ SEMUA OPERASI SELESAI!")
	log.Printf("  Kultum besok (tgl %d): Ananda", tomorrowDay)
	log.Printf("  Slot sholat Torik → Firdaus: %d slot", n1+n2)
	log.Println("  Member Firdaus: ditambahkan")
	log.Println("========================================")
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}
