# 🕌 WhatsApp Reminder Bot - Masjid Al-Wasii UNILA

24/7 automated WhatsApp reminder bot for **Masjid Al-Wasii, Universitas Lampung (UNILA)**, Kota Bandar Lampung (Kecamatan Rajabasa). Built with **Go (Golang)**, **whatsmeow**, **SQLite3**, and ready for 24/7 deployment on **Fly.io** with persistent volume mounts.

---

## 🌟 Key Features

1. **Jadwal Sholat Integration:**
   - Real-time Kemenag prayer times via MyQuran API (`https://api.myquran.com/v2/sholat/jadwal/1014/{YYYY}/{MM}/{DD}`, City ID `1014` - Kota Bandar Lampung).
   - Daily schedule fetch every day at `00:01 WIB` with in-memory timer coordination and SQLite fallback cache.

2. **Automated Daytime Prayer Reminders (Zhuhur, Ashar, Maghrib, Isya):**
   - Automatically dispatched **10 minutes before** actual adzan time.
   - Mentions assigned Adzan & Imam officers with real WhatsApp `@mention` tags (`MentionedJID`).
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
   - `!test [subuh|zhuhur|ashar|maghrib|isya|jumat]` - Test reminder dispatching immediately.

---

## 🔒 Privacy & Public GitHub Repository Setup

To ensure **privacy and security** when publishing this repository publicly on GitHub:
* **Private Data Excluded from Git:** File `bot.db` (sesi WhatsApp login), `.env`, dan `data/members.json` (daftar nomor HP asli) sudah otomatis dikecualikan oleh `.gitignore`.
* **Template Anggota:** Tersedia `members.example.json` sebagai contoh format.
* **Di Komputer Lokal:** Letakkan file `members.json` di dalam folder `data/members.json`.
* **Di Fly.io:** Anda bisa mengatur nomor HP via Fly Secrets `MEMBERS_JSON` atau menaruh file `members.json` di persistent volume `/data`.

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
4. Sesi login tersimpan permanen di `./data/bot.db`.

---

## ☁️ Deployment on Fly.io

### 1. Install Flyctl & Log in
```bash
fly auth login
```

### 2. Create Fly.io Application & Volume
```bash
# Create the app
fly apps create masjid-alwasii-remind-bot

# Create 1GB persistent volume in Singapore (sin) region for SQLite data
fly volumes create remind_bot_data --region sin --size 1
```

### 3. Set Target JID & Configuration Secrets
```bash
fly secrets set TARGET_JID="120363431135211849@g.us" ENABLE_JUMAT_REMINDER="false" CITY_ID="1014"
```

Untuk memasukkan data nomor HP anggota ke Fly.io secara aman (tanpa masuk ke Git):
```bash
fly secrets set MEMBERS_JSON='[{"display_name":"Fajar","phone":"6285768971813","aliases":["fajar"]},{"display_name":"Iskandar","phone":"6285758426987","aliases":["iskandar"]},{"display_name":"Ananda","phone":"6285180530165","aliases":["ananda","nanda"]},{"display_name":"Arif","phone":"6283181878854","aliases":["arif"]},{"display_name":"Arjuna","phone":"6285268988283","aliases":["arjuna","juna"]},{"display_name":"Basit","phone":"6285766840697","aliases":["basit","abdul basit"]},{"display_name":"Imam","phone":"6288274018823","aliases":["imam"]},{"display_name":"Haris","phone":"6282367759870","aliases":["haris","dhiki","diki"]},{"display_name":"Thoriq","phone":"6285664249480","aliases":["thoriq","torik","t"]},{"display_name":"Ruzi","phone":"6282298399181","aliases":["ruzi"]},{"display_name":"Makhasin","phone":"6285758970652","aliases":["makhasin","khasin"]}]'
```

### 4. Deploy
```bash
fly deploy
```

### 5. Pair WhatsApp via Fly Logs
```bash
fly logs
```
Scan QR code yang muncul di log jika belum pernah dipairing sebelumnya.

---

## 🧪 Testing Suite

Jalankan pengujian unit test:
```bash
go test -v ./...
```

---

## 🛡️ License
Open source untuk Masjid Al-Wasii, Universitas Lampung (UNILA).
