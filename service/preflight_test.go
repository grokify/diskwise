package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/grokify/diskwise/platform"
)

type fakeProbe struct {
	vol      platform.VolumeStats
	free     int64
	freeErr  error
	snaps    []string
	snapsErr error
}

func (f fakeProbe) Volume(string) (platform.VolumeStats, error) { return f.vol, nil }
func (f fakeProbe) ContainerFreeBytes(context.Context, string) (int64, error) {
	return f.free, f.freeErr
}
func (f fakeProbe) LocalSnapshots(context.Context) ([]string, error) { return f.snaps, f.snapsErr }

func TestService_Preflight_Sufficient(t *testing.T) {
	const gib = int64(1 << 30)
	svc := newTestService(t).WithProbe(fakeProbe{
		vol:  platform.VolumeStats{MountPoint: "/", FSType: "apfs", CapacityBytes: 1000 * gib, AvailableBytes: 164 * gib},
		free: 176 * gib,
	})
	res, err := svc.Preflight(context.Background(), PreflightRequest{Path: "/", NeedBytes: 50 * gib})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sufficient || res.ShortfallBytes != 0 {
		t.Errorf("Sufficient=%v Shortfall=%d, want true/0", res.Sufficient, res.ShortfallBytes)
	}
	if res.ContainerFreeBytes != 176*gib {
		t.Errorf("ContainerFreeBytes = %d", res.ContainerFreeBytes)
	}
}

func TestService_Preflight_ShortfallMentionsSnapshots(t *testing.T) {
	const gib = int64(1 << 30)
	svc := newTestService(t).WithProbe(fakeProbe{
		vol:   platform.VolumeStats{AvailableBytes: 20 * gib},
		snaps: []string{"com.apple.TimeMachine.x.local"},
	})
	res, err := svc.Preflight(context.Background(), PreflightRequest{Path: "/", NeedBytes: 50 * gib})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sufficient || res.ShortfallBytes != 30*gib {
		t.Errorf("Sufficient=%v Shortfall=%d, want false/%d", res.Sufficient, res.ShortfallBytes, 30*gib)
	}
	var mentions bool
	for _, n := range res.Notes {
		if strings.Contains(n, "1 local snapshot(s)") {
			mentions = true
		}
	}
	if !mentions {
		t.Errorf("Notes = %v, want one mentioning the local snapshot", res.Notes)
	}
}

func TestService_Preflight_ProbeFailuresAreNotFatal(t *testing.T) {
	svc := newTestService(t).WithProbe(fakeProbe{
		vol:      platform.VolumeStats{AvailableBytes: 10},
		freeErr:  errors.New("no diskutil"),
		snapsErr: errors.New("no tmutil"),
	})
	res, err := svc.Preflight(context.Background(), PreflightRequest{Path: "/"})
	if err != nil {
		t.Fatalf("best-effort probes must not fail the preflight: %v", err)
	}
	if !res.Sufficient {
		t.Error("NeedBytes 0 should always be sufficient")
	}
	if len(res.Notes) < 3 {
		t.Errorf("Notes = %v, want both probe failures plus the purgeable caveat", res.Notes)
	}
}
