package blackvue

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

// fakeCommandSocketServer mensimulasikan command-socket dashcam (protokol Fleet SDK):
// baca header 12B, balas header 12B + JSON body sesuai command yang diminta. Dipakai
// untuk verifikasi framing SocketClient tanpa perlu dashcam/Fleet Server asli.
func fakeCommandSocketServer(t *testing.T, responses map[uint16]string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gagal listen: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			header := make([]byte, 12)
			if _, err := conn.Read(header); err != nil {
				return
			}
			cmd := binary.BigEndian.Uint16(header[6:8])
			body := []byte(responses[cmd])

			respHeader := make([]byte, 12)
			binary.BigEndian.PutUint32(respHeader[0:4], socketStartCode)
			binary.BigEndian.PutUint16(respHeader[4:6], socketProtocolVer)
			binary.BigEndian.PutUint16(respHeader[6:8], cmd)
			binary.BigEndian.PutUint32(respHeader[8:12], uint32(len(body)))
			conn.Write(respHeader)
			conn.Write(body)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func TestSocketClient_QuerySDCardInfo(t *testing.T) {
	// "status":0x00 (hex literal, bukan JSON valid) -- persis kuirk firmware asli
	// yang dikonfirmasi lewat capture langsung ke dashcam 25 Jul 2026.
	addr := fakeCommandSocketServer(t, map[uint16]string{
		CmdSDCardInfo: `{"cid":"abc","total":1000,"avail":500,"used":500,"status":0x00}`,
	})
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	client, err := DialSocket(host, port, 2*time.Second)
	if err != nil {
		t.Fatalf("DialSocket gagal: %v", err)
	}
	defer client.Close()

	sd, err := client.QuerySDCardInfo(2 * time.Second)
	if err != nil {
		t.Fatalf("QuerySDCardInfo gagal: %v", err)
	}
	if sd.Total != 1000 || sd.Avail != 500 || sd.Used != 500 || sd.Status != 0 {
		t.Errorf("SDCardInfo salah parse: %+v", sd)
	}
}

func TestSanitizeHexLiterals(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"single-digit hex", `{"status":0x00}`, `{"status":0}`},
		{"multi-digit hex", `{"status":0x04}`, `{"status":4}`},
		{"uppercase hex", `{"status":0X02}`, `{"status":2}`},
		{"already-decimal untouched", `{"status":123}`, `{"status":123}`},
		{"multiple fields", `{"a":0x01,"b":2,"c":0x0A}`, `{"a":1,"b":2,"c":10}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(sanitizeHexLiterals([]byte(c.in)))
			if got != c.want {
				t.Errorf("sanitizeHexLiterals(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSocketClient_QueryRTCAndSIM(t *testing.T) {
	addr := fakeCommandSocketServer(t, map[uint16]string{
		CmdRTCStatus: `{"low_vol":1,"utc":1700000000}`,
		CmdSIMStatus: `{"mac":"AA","psn":"BB","sim_status":0,"pwr_status":1,"err_code":0}`,
	})
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)

	client, err := DialSocket(host, port, 2*time.Second)
	if err != nil {
		t.Fatalf("DialSocket gagal: %v", err)
	}
	defer client.Close()

	rtc, err := client.QueryRTCStatus(2 * time.Second)
	if err != nil {
		t.Fatalf("QueryRTCStatus gagal: %v", err)
	}
	if rtc.LowVol != 1 {
		t.Errorf("RTCStatus.LowVol = %d, want 1", rtc.LowVol)
	}

	sim, err := client.QuerySIMStatus(2 * time.Second)
	if err != nil {
		t.Fatalf("QuerySIMStatus gagal: %v", err)
	}
	if sim.PwrStatus != 1 {
		t.Errorf("SIMStatus.PwrStatus = %d, want 1", sim.PwrStatus)
	}
}
