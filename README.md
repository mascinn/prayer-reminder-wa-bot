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

5. **Interactive Diagnostic Commands:**
   - `!ping` - Health check verification.
   - `!jid` - Displays the current chat / group JID for easy configuration.
   - `!jadwal` - Displays today's full schedule and assigned officers.
   - `!kultum` - Displays the 10-member Kultum queue with current pointer.
   - `!setkultum [1-10 / nama]` - Manually set/adjust the active Kultum speaker pointer.
   - `!test [subuh|zhuhur|ashar|maghrib|isya|jumat]` - Test reminder dispatching immediately.

---

## 🔒 Privacy & Public GitHub Repository Setup

To ensure **privacy and security** when publishing this repository publicly on GitHub:
* **Private Data Excluded from Git:** File `bot.db` (sesi WhatsApp login), `.env`, `data/members.json` (daftar nomor HP asli), dan `data/schedule.json` (jadwal lokal) sudah otomatis dikecualikan oleh `.gitignore`.
* **Template Konfigurasi Publik:** Tersedia `members.example.json` dan `schedule.example.json` sebagai contoh format.
* **Di Komputer Lokal:** File data berada di `data/members.json` dan `data/schedule.json`.
* **Di Render Cloud:** Konfigurasi nomor HP dan jadwal dimasukkan via Environment Variables (`MEMBERS_JSON`, `SCHEDULE_JSON`).

---

## 📋 Weekly Duty Matrix

| Day | Subuh (Adzan / Imam) | Zhuhur (Adzan / Imam) | Ashar (Adzan / Imam) | Maghrib (Adzan / Imam) | Isya (Adzan / Imam) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Senin** | Imam / Haris | Ruzi / Basit | Arjuna / Ananda | Ruzi / Ananda | Basit / Iskandar |
| **Selasa** | Basit / Haris | Arjuna / Ruzi | Makhasin / Fajar | Imam / T | Makhasin / Imam |
| **Rabu** | Basit / Haris | T / Ruzi | Ruzi / T | Iskandar / Arjuna | Ruzi / Imam |
| **Kamis** | Ananda / Haris | T / Ruzi | Fajar / Arjuna | Basit / Imam | Fajar / Makhasin |
| **Jum'at** | Makhasin / Haris | _(Sholat Jum'at)_ | Ananda / Basit | Fajar / Arjuna | Imam / Makhasin |
| **Sabtu** | Basit / Haris | Fajar / Ananda | Iskandar / Makhasin | Iskandar / Ananda | Imam / Arjuna |
| **Minggu** | Arjuna / Haris | Iskandar / Fajar | Makhasin / Ananda | Iskandar / Fajar | Fajar / Iskandar |

### 🎙️ Subuh Kultum Round-Robin Queue (10 Members)
1. **Iskandar** ➔ 2. **Haris** ➔ 3. **Thoriq** ➔ 4. **Ruzi** ➔ 5. **Fajar** ➔ 6. **Ananda** ➔ 7. **Makhasin** ➔ 8. **Arjuna** ➔ 9. **Imam** ➔ 10. **Basit** ➔ _(loops back to 1)_.

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
* `MEMBERS_JSON` =
```json
[{"display_name":"Fajar","phone":"6285768971813","aliases":["fajar","fajar aji pangestu"]},{"display_name":"Iskandar","phone":"6285758426987","aliases":["iskandar"]},{"display_name":"Ananda","phone":"6285180530165","aliases":["ananda","nanda","ananda kusuma"]},{"display_name":"Arif","phone":"6283181878854","aliases":["arif","arif hidayat"]},{"display_name":"Arjuna","phone":"6285268988283","aliases":["arjuna","juna","arjuna yulizar mahendra"]},{"display_name":"Basit","phone":"6285766840697","aliases":["basit","abdul basit","basit diwa fakara"]},{"display_name":"Imam","phone":"6288274018823","aliases":["imam","imam rifai"]},{"display_name":"Haris","phone":"6282367759870","aliases":["haris","dhiki","diki","dhiki harisno"]},{"display_name":"Thoriq","phone":"6285664249480","aliases":["thoriq","torik","t","torik lianda"]},{"display_name":"Ruzi","phones":["6282298399181","6285711024192"],"aliases":["ruzi","ruzi yandi"]},{"display_name":"Makhasin","phone":"6285758970652","aliases":["makhasin","khasin"]}]
```

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
