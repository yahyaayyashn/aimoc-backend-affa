// socket.go — client untuk "command socket" BlackVue (port 9771, protokol raw TCP
// milik Fleet SDK, BUKAN endpoint CGI/HTTP). Dipakai DeviceHealthPoller
// (internal/service/device_health.go) untuk baca status SD card, RTC, sinyal GPS
// (proxy lewat Driving Info), dan SIM/modem langsung dari dashcam.
//
// Protokol (Big-Endian, referensi: Fleet SDK 3.0 API Documentation):
//
//	Request  header (12B): StartCode(4B,0x00000001) Version(2B,3000) DataType(2B) DataSize(4B,=0)
//	Response header (12B): sama format + Data(DataSize byte, JSON)
//
// Idle timeout command-socket ~15 detik -- jangan biarkan koneksi menganggur lama;
// tiap siklus poll buka koneksi baru, query berurutan, lalu tutup (lihat pola sama
// di blackvue_socket_client.py, script referensi tim, yang framing-nya diikuti persis).
package blackvue

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"time"
)

const (
	socketStartCode   uint32 = 0x00000001
	socketProtocolVer uint16 = 3000

	CmdSDCardInfo  uint16 = 0x0003
	CmdDrivingInfo uint16 = 0x0005
	CmdRTCStatus   uint16 = 0x0006
	CmdSIMStatus   uint16 = 0x000C
)

// SDCardInfo — response DataType 0x0003.
type SDCardInfo struct {
	Total  int64 `json:"total"` // KByte
	Avail  int64 `json:"avail"` // KByte
	Used   int64 `json:"used"`  // KByte
	Status int   `json:"status"`
	// Status: 0x00 Normal, 0x01 Write failure, 0x02 Write protected, 0x04 Read/Write failure
}

// RTCStatus — response DataType 0x0006.
type RTCStatus struct {
	LowVol int   `json:"low_vol"` // 1 = baterai RTC drop, jam sistem reset ke default
	UTC    int64 `json:"utc"`
}

// DrivingInfo — response DataType 0x0005. Hanya field GPS yang dipakai di sini
// (no_gps_count/no_gps_duration) -- dipakai sebagai proxy status sinyal GPS,
// bukan koordinat asli (lihat diskusi review 25 Jul 2026: GPS bukan bagian dari
// 0x0006 seperti disangka awal, dan koordinat asli butuh HTTP livedata.cgi terpisah
// -- di luar scope, cukup sinyal sehat/tidaknya).
type DrivingInfo struct {
	NoGPSCount    int `json:"no_gps_count"`
	NoGPSDuration int `json:"no_gps_duration"` // jam
}

// SIMStatus — response DataType 0x000C (bukan 0x0016 seperti di daftar awal --
// 0x0016 adalah command lain, dikoreksi lewat riset protokol 25 Jul 2026).
type SIMStatus struct {
	SimStatus int `json:"sim_status"`
	PwrStatus int `json:"pwr_status"` // 0=Off, 1=On (daya modem)
	ErrCode   int `json:"err_code"`
}

type SocketClient struct {
	conn net.Conn
}

// DialSocket membuka koneksi TCP mentah ke command-socket dashcam (port dari
// Fleet Server proxy "Socket_<mac>", resolve lewat ResolveSocketPort -- port ini
// berubah tiap restart fleet-server, jangan pernah cache).
func DialSocket(host string, port int, timeout time.Duration) (*SocketClient, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return nil, fmt.Errorf("gagal dial command-socket: %w", err)
	}
	return &SocketClient{conn: conn}, nil
}

func (c *SocketClient) Close() error {
	return c.conn.Close()
}

// Query mengirim 1 command (semua command di paket ini read-only, DataSize=0)
// dan menunggu response, return raw JSON body (bisa nil kalau DataSize response=0).
func (c *SocketClient) Query(cmd uint16, timeout time.Duration) (json.RawMessage, error) {
	if err := c.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	header := make([]byte, 12)
	binary.BigEndian.PutUint32(header[0:4], socketStartCode)
	binary.BigEndian.PutUint16(header[4:6], socketProtocolVer)
	binary.BigEndian.PutUint16(header[6:8], cmd)
	binary.BigEndian.PutUint32(header[8:12], 0)

	if _, err := c.conn.Write(header); err != nil {
		return nil, fmt.Errorf("gagal kirim command 0x%04X: %w", cmd, err)
	}

	respHeader := make([]byte, 12)
	if _, err := io.ReadFull(c.conn, respHeader); err != nil {
		return nil, fmt.Errorf("gagal baca header response 0x%04X: %w", cmd, err)
	}
	dataSize := binary.BigEndian.Uint32(respHeader[8:12])
	if dataSize == 0 {
		return nil, nil
	}
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return nil, fmt.Errorf("gagal baca body response 0x%04X: %w", cmd, err)
	}
	return sanitizeHexLiterals(data), nil
}

// sanitizeHexLiterals — firmware dashcam nyata mengirim field angka dalam notasi hex
// literal (mis. "status":0x00), BUKAN JSON valid (dikonfirmasi lewat capture langsung
// ke device asli 25 Jul 2026 -- persis seperti contoh di dokumentasi SDK yang tadinya
// dikira sekadar typo dokumentasi). encoding/json menolak ini mentah-mentah, jadi
// dikonversi dulu ke desimal sebelum di-unmarshal.
var hexLiteralRe = regexp.MustCompile(`:\s*0[xX]([0-9A-Fa-f]+)`)

func sanitizeHexLiterals(raw []byte) []byte {
	return hexLiteralRe.ReplaceAllFunc(raw, func(match []byte) []byte {
		sub := hexLiteralRe.FindSubmatch(match)
		n, err := strconv.ParseInt(string(sub[1]), 16, 64)
		if err != nil {
			return match
		}
		return []byte(":" + strconv.FormatInt(n, 10))
	})
}

// QuerySDCardInfo, QueryRTCStatus, QueryDrivingInfo, QuerySIMStatus — helper query +
// unmarshal langsung ke struct terkait, dipakai DeviceHealthPoller.
func (c *SocketClient) QuerySDCardInfo(timeout time.Duration) (*SDCardInfo, error) {
	raw, err := c.Query(CmdSDCardInfo, timeout)
	if err != nil {
		return nil, err
	}
	var out SDCardInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("gagal parse SD card info: %w", err)
	}
	return &out, nil
}

func (c *SocketClient) QueryRTCStatus(timeout time.Duration) (*RTCStatus, error) {
	raw, err := c.Query(CmdRTCStatus, timeout)
	if err != nil {
		return nil, err
	}
	var out RTCStatus
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("gagal parse RTC status: %w", err)
	}
	return &out, nil
}

func (c *SocketClient) QueryDrivingInfo(timeout time.Duration) (*DrivingInfo, error) {
	raw, err := c.Query(CmdDrivingInfo, timeout)
	if err != nil {
		return nil, err
	}
	var out DrivingInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("gagal parse driving info: %w", err)
	}
	return &out, nil
}

func (c *SocketClient) QuerySIMStatus(timeout time.Duration) (*SIMStatus, error) {
	raw, err := c.Query(CmdSIMStatus, timeout)
	if err != nil {
		return nil, err
	}
	var out SIMStatus
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("gagal parse SIM status: %w", err)
	}
	return &out, nil
}
