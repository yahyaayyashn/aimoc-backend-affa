# AIMOC Backend (GoLang + Fiber + PostgreSQL)

## Setup

1. Salin `.env.example` ke `.env` lalu sesuaikan kredensial DB & JWT.
2. Buat database PostgreSQL:
   ```sql
   CREATE DATABASE aimoc;
   ```
3. Jalankan migration (gunakan `golang-migrate` atau eksekusi manual file `migrations/*.up.sql` urut).
4. Install dependencies:
   ```bash
   go mod tidy
   ```
5. Jalankan server:
   ```bash
   go run ./cmd/api
   ```
6. Server berjalan di `http://localhost:8080`.

## User default (password: Admin@123)

- `admin@aimoc.id` (Super Admin)
- `manajemen@aimoc.id` (Manajemen)
- `sales@aimoc.id` (Admin Sales)
- `operator@aimoc.id` (Operator)

## Webhook CCTV AI

Header: `X-Webhook-Secret: <CCTV_WEBHOOK_SECRET>`

- `POST /api/v1/cctv/gate-in`
- `POST /api/v1/cctv/loading-start`
- `POST /api/v1/cctv/loading-end`
- `POST /api/v1/cctv/gate-out`

## WebSocket

- `ws://localhost:8080/ws/dashboard`
- `ws://localhost:8080/ws/admin`
- `ws://localhost:8080/ws/operator`
- `ws://localhost:8080/ws/customer:<user_id>`
