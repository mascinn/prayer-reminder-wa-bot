# 🕌 WhatsApp Reminder Bot - Masjid Al-Wasii UNILA

24/7 automated WhatsApp reminder bot for **Masjid Al-Wasii, Universitas Lampung (UNILA)**, Kota Bandar Lampung (Kecamatan Rajabasa). Built with **Go (Golang)**, **whatsmeow**, **SQLite3**, and ready for 24/7 deployment on **Render** with automatic anti-sleep health ping.

---

## 🌟 Key Features

1. **Jadwal Sholat Integration:**
   - Real-time Kemenag prayer times via MyQuran API (`https://api.myquran.com/v2/sholat/jadwal/1014/{YYYY}/{MM}/{DD}`, City ID `1014` - Kota Bandar Lampung).
   - Daily schedule fetch every day at `00:01 WIB` with in-memory timer coordination and SQLite fallback cache.

2. **Automated Daytime Prayer Reminders (Zhuhur, Ashar, Maghrib, Isya):**
   - Automatically dispatched **10 minutes before** actual adzan time.
   - Mentions assigned Adzan & Imam officers with real WhatsApp `@mention` tags (`MentionedJID`). Supports multiple phone numbers per member (e.g. Ruzi Yandi).
   - Automatically skips Zhuhur on Fridays (replaced by Friday Prayer).

3. **Nightly Subuh & Kultum Reminders:**
   - Dispatched every night at `20:30 WIB` for tomorrow's Subuh prayer.
   - Mentions tomorrow's Adzan, Imam, and Kultum speaker.
   - **Continuous 10-person round-robin loop** for Kultum speakers persisted across restarts in SQLite (`bot_state`).

4. **Optional Friday Preparation Reminder:**
   - Triggered every Thursday night at `21:00 WIB` when `ENABLE_JUMAT_REMINDER=true`.

5. **WhatsApp Reaction Attendance Tracking & Midnight Cut-Off:**
   - Real-time attendance logging via message reactions:
     - 👆 = Adzan tidak menjalankan tugas.
     - 👇 = Imam tidak menjalankan tugas.
     - ✌️ = Keduanya tidak menjalankan tugas.
   - Bot automatically confirms recorded reports by reacting with `✅` (or removes reaction if un-reacted / canceled).
   - **Midnight Cut-off:** Reports and adjustments are accepted until 23:59:59 WIB on the prayer execution date, then permanently locked.

6. **Interactive WhatsApp Commands:**
   - 📖 **Menu & Bantuan:**
     - `!menu` *(alias: `!help`, `!bantuan`)* - Menampilkan panduan lengkap seluruh perintah bot.
   - 📅 **Informasi Jadwal:**
     - `!jadwal` - Menampilkan jadwal semua waktu sholat hari ini & jam adzan real-time.
     - `!jadwal [hari]` - Menampilkan jadwal sholat pada hari tertentu (misal: `!jadwal senin`).
     - `!besok` - Menampilkan jadwal sholat & petugas esok hari.
     - `!subuh`, `!zhuhur`, `!ashar`, `!maghrib`, `!isya` - Cek langsung petugas & jam sholat hari ini.
     - `!matriks` *(alias: `!jadwallengkap`, `!jadwalminggu`)* - Menampilkan tabel matriks tugas 1 pekan penuh (Senin - Minggu).
     - `!tugas [nama]` *(alias: `!tugassaya`, `!jadwalsaya`)* - Menampilkan jadwal tugas pribadi dalam sepekan & giliran kultum.
     - `!kultum` - Menampilkan 10 daftar urutan penceramah kultum Subuh & giliran besok.
     - `!kantin` *(alias: `!jadwalkantin`)* - Menampilkan jadwal piket penarikan infaq kantin.
   - 📊 **Rekapitulasi Keaktifan:**
     - `!rekap` - Rekapitulasi keaktifan petugas bulan berjalan.
     - `!rekap [bulan] [tahun]` - Rekapitulasi bulan tertentu (contoh: `!rekap 08 2026`).
     - `!rekap detail [nama]` - Rincian tanggal & sholat yang tidak dijalankan oleh petugas tertentu.
   - ⚙️ **Sistem & Pengujian:**
     - `!ping` - Cek status kesehatan & koneksi bot.
     - `!jid` - Menampilkan info ID chat WhatsApp & status target grup pengingat.
     - `!setgrup` *(alias: `!settarget`)* - Menyetel grup aktif saat ini sebagai target pengingat.
     - `!test [target]` - Uji coba kirim pesan pengingat langsung (`subuh`, `zhuhur`, `ashar`, `maghrib`, `isya`, `jumat`, `kantin`).

---

## 🔒 Privacy & Public GitHub Repository Setup

To ensure **privacy and security** when publishing this repository publicly on GitHub:
* **Private Data Excluded from Git:** File `bot.db` (sesi WhatsApp login), `.env`, `data/members.json` (daftar nomor HP asli), dan `data/schedule.json` (jadwal lokal) sudah otomatis dikecualikan oleh `.gitignore`.
* **Template Konfigurasi Publik:** Tersedia `members.example.json` dan `schedule.example.json` sebagai contoh format.
* **Di Komputer Lokal:** File data berada di `data/members.json` dan `data/schedule.json`.
* **Di Render Cloud:** Konfigurasi nomor HP dan jadwal dimasukkan via Environment Variables (`MEMBERS_JSON`, `SCHEDULE_JSON`).

---

## 📋 Weekly Duty Matrix (Contoh)

| Hari | Subuh (Adzan / Imam) | Zhuhur (Adzan / Imam) | Ashar (Adzan / Imam) | Maghrib (Adzan / Imam) | Isya (Adzan / Imam) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Senin** | Ahmad / Zaid | Bilal / Umar | Ali / Usman | Hamzah / Hasan | Husain / Salman |
| **Selasa** | Zaid / Ahmad | Ali / Bilal | Usman / Hamzah | Hasan / Husain | Salman / Ahmad |
| **Rabu** | Bilal / Ahmad | Umar / Zaid | Ali / Hamzah | Usman / Hasan | Husain / Salman |
| **Kamis** | Usman / Zaid | Umar / Bilal | Hamzah / Ali | Hasan / Husain | Salman / Ahmad |
| **Jum'at** | Hamzah / Ahmad | _(Sholat Jum'at)_ | Bilal / Ali | Usman / Hasan | Husain / Salman |
| **Sabtu** | Hasan / Zaid | Ahmad / Umar | Ali / Hamzah | Bilal / Usman | Salman / Ahmad |
| **Minggu** | Husain / Ahmad | Zaid / Bilal | Umar / Ali | Hamzah / Hasan | Usman / Salman |

### 🎙️ Subuh Kultum Round-Robin Queue
Urutan penceramah kultum Subuh berputar bergantian secara otomatis (*round-robin*) dan tersimpan permanen di database.

---

## 🚀 Quick Start (Local Development)

### 1. Setup Environment
```bash
cp .env.example .env
```
Isi `TARGET_JID` di `.env` (bisa didapatkan dengan mengetik `!jid` di grup/chat WhatsApp).

### 2. Run the Bot
```bash
go run main.go
```

Pada startup pertama, QR code ASCII akan muncul di terminal:
1. Buka **WhatsApp** di HP.
2. Pilih **Perangkat Tertaut (Linked Devices)** ➔ **Tautkan Perangkat**.
3. Scan QR code di terminal.
4. Sesi login tersimpan di `./data/bot.db`.

---

## ☁️ Deployment on Render (100% Free 24/7)

### 1. Create Web Service on Render
1. Buka [https://dashboard.render.com](https://dashboard.render.com) dan login via **GitHub**.
2. Klik **New +** ➔ Pilih **Web Service**.
3. Pilih repository: `mascinn/prayer-reminder-wa-bot`.
4. Pilih Region: **Singapore (Southeast Asia)**.
5. Pilih Runtime: **Docker** dan Plan: **Free ($0/month)**.

### 2. Set Environment Variables
Tambahkan variabel berikut di dashboard Render:

* `TARGET_JID` = `120363431135211849@g.us`
* `CITY_ID` = `1014`
* `ENABLE_JUMAT_REMINDER` = `false`
* `TIMEZONE` = `Asia/Jakarta`
* `LOG_LEVEL` = `INFO`
* `TURSO_DATABASE_URL` = `libsql://prayer-reminder-wa-bot-mascinn.aws-ap-northeast-1.turso.io`
* `TURSO_AUTH_TOKEN` = `eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...`

*(Data anggota, nomor HP, dan jadwal mingguan otomatis dibaca langsung dari database Turso Cloud tanpa perlu memasukkan JSON secara manual).*

### 3. Deploy & Scan WhatsApp
1. Klik **Create Web Service**.
2. Buka tab **Logs** dan scan QR Code WhatsApp yang muncul.

### 4. Setup Anti-Sleep Ping (cron-job.org)
1. Buka [https://cron-job.org](https://cron-job.org) ➔ Buat akun gratis.
2. Buat Cronjob baru:
   * **URL:** Masukkan URL Render Anda (misal `https://prayer-reminder-wa-bot.onrender.com`).
   * **Schedule:** Every 5 minutes.
3. Bot akan menyala 24/7 non-stop tanpa pernah sleep!

---

## 🧪 Testing Suite

Jalankan pengujian unit test:
```bash
go test -v ./...
```

---

## 🛡️ License
Open source untuk Masjid Al-Wasii, Universitas Lampung (UNILA).
