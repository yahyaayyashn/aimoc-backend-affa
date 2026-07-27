package service

import (
	"testing"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/blackvue"
)

func strp(s string) *string { return &s }

func TestSDStatusLabel(t *testing.T) {
	cases := []struct {
		name string
		sd   blackvue.SDCardInfo
		want string
	}{
		{"normal", blackvue.SDCardInfo{Total: 1000, Avail: 500, Status: 0x00}, "NORMAL"},
		{"nearly full below ratio", blackvue.SDCardInfo{Total: 1000, Avail: 10, Status: 0x00}, "FULL"},
		{"write failure takes priority even if not full", blackvue.SDCardInfo{Total: 1000, Avail: 500, Status: 0x01}, "WRITE_FAILURE"},
		{"write protected", blackvue.SDCardInfo{Total: 1000, Avail: 500, Status: 0x02}, "WRITE_PROTECTED"},
		{"rw failure", blackvue.SDCardInfo{Total: 1000, Avail: 500, Status: 0x04}, "RW_FAILURE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sdStatusLabel(&c.sd)
			if got != c.want {
				t.Errorf("sdStatusLabel(%+v) = %q, want %q", c.sd, got, c.want)
			}
		})
	}
}

func TestSIMStatusLabel(t *testing.T) {
	cases := []struct {
		name string
		sim  blackvue.SIMStatus
		want string
	}{
		{"normal", blackvue.SIMStatus{PwrStatus: 1, ErrCode: 0}, "NORMAL"},
		{"modem off", blackvue.SIMStatus{PwrStatus: 0, ErrCode: 0}, "MODEM_OFF"},
		{"error", blackvue.SIMStatus{PwrStatus: 1, ErrCode: 3}, "ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := simStatusLabel(&c.sim)
			if got != c.want {
				t.Errorf("simStatusLabel(%+v) = %q, want %q", c.sim, got, c.want)
			}
		})
	}
}

func TestDeviceHealthReasonCode_Priority(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		cam  domain.Camera
		want string
	}{
		{"all healthy", domain.Camera{}, ""},
		{"sd problem only", domain.Camera{SDStatus: strp("WRITE_FAILURE")}, "SD_WRITE_FAILURE"},
		{"rtc problem only", domain.Camera{RTCLowBattery: true}, "RTC_LOW_BATTERY"},
		{"gps problem only", domain.Camera{GPSNoSignalSince: &now}, "GPS_NO_SIGNAL"},
		{"sim problem only", domain.Camera{SIMStatus: strp("MODEM_OFF")}, "SIM_INACTIVE"},
		{
			"sd takes priority over everything else",
			domain.Camera{SDStatus: strp("FULL"), RTCLowBattery: true, GPSNoSignalSince: &now, SIMStatus: strp("ERROR")},
			"SD_FULL",
		},
		{"sd normal does not count as a problem", domain.Camera{SDStatus: strp("NORMAL")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deviceHealthReasonCode(c.cam)
			if got != c.want {
				t.Errorf("deviceHealthReasonCode(%+v) = %q, want %q", c.cam, got, c.want)
			}
		})
	}
}
